package com.example.ptranslate.core.account

import com.example.ptranslate.core.net.BackendException
import kotlinx.coroutines.test.runTest
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import java.security.MessageDigest

class AccountTest {
    private val server = MockWebServer()

    @After
    fun tearDown() = server.shutdown()

    private fun json(body: String, code: Int = 200) = MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body)

    private val signedUp = Session("tok", Profile(id = "u1", displayName = "Ada", handle = "ada", language = "en"))

    @Test
    fun `a first sign-in keeps the session and starts signing up from the name`() = runTest {
        server.enqueue(json("""{"token":"t1","user":{"id":"u1","display_name":"Ada","handle":"","language":""},"new_user":true}""", 201))
        val store = MemorySessionStore()
        val account = testAccount(server, store)

        account.signInWithGoogle("id-token", "raw-nonce")

        val request = server.takeRequest()
        assertEquals("/v1/sessions/google", request.path)
        assertEquals("""{"identity_token":"id-token","nonce":"raw-nonce"}""", request.body.readUtf8())
        assertEquals("t1", account.token)
        assertEquals("t1", store.session?.token)
        assertEquals(OnboardingStep.NAME, account.step.value)
    }

    @Test
    fun `an account set up on another phone goes straight in`() = runTest {
        server.enqueue(json("""{"token":"t2","user":{"id":"u1","display_name":"Ada","handle":"ada","language":"en"},"new_user":false}"""))
        val account = testAccount(server)

        account.signInWithGoogle("id-token", "n")

        assertEquals(OnboardingStep.DONE, account.step.value)
    }

    @Test
    fun `requests carry the token, and a 401 forgets the session`() = runTest {
        server.enqueue(json("""{"error":{"code":"unauthenticated","message":"the session has ended"}}""", 401))
        val store = MemorySessionStore(signedUp)
        val account = testAccount(server, store, MemoryProgress(OnboardingStep.DONE))

        try {
            account.refresh()
            fail("expected the refusal")
        } catch (e: BackendException) {
            assertEquals(401, e.status)
            assertEquals("unauthenticated", e.code)
        }
        assertEquals("Bearer tok", server.takeRequest().getHeader("Authorization"))
        assertNull(account.session.value)
        assertNull(store.session)
        assertEquals(OnboardingStep.NAME, account.step.value)
    }

    @Test
    fun `a profile change sends only what changes`() = runTest {
        server.enqueue(json("""{"id":"u1","display_name":"Ada","handle":"ada.l","language":"en"}"""))
        val account = testAccount(server, MemorySessionStore(signedUp))

        account.updateProfile(handle = "ada.l")

        val request = server.takeRequest()
        assertEquals("PATCH", request.method)
        assertEquals("""{"handle":"ada.l"}""", request.body.readUtf8())
        assertEquals("ada.l", account.profile?.handle)
    }

    @Test
    fun `an ID check reads the suggestions, and a missing person has a code`() = runTest {
        server.enqueue(json("""{"handle":"ada","available":false,"suggestions":["ada.l","ada26"]}"""))
        server.enqueue(json("""{"error":{"code":"user_not_found","message":"no one has that ID"}}""", 404))
        val account = testAccount(server, MemorySessionStore(signedUp))

        assertEquals(HandleCheck("ada", available = false, suggestions = listOf("ada.l", "ada26")), account.checkHandle("ada"))
        try {
            account.findUser("nobody")
            fail("expected the refusal")
        } catch (e: BackendException) {
            assertEquals("user_not_found", e.code)
            assertFalse(e.retryable)
        }
        // A 404 is not the end of the session.
        assertTrue(account.session.value != null)
    }

    @Test
    fun `signing out forgets the session even when the server cannot be told`() = runTest {
        server.shutdown()
        val account = testAccount(server, MemorySessionStore(signedUp), MemoryProgress(OnboardingStep.DONE))
        try {
            account.signOut()
        } catch (_: BackendException) {
        }
        assertNull(account.session.value)
    }

    @Test
    fun `the nonce Google gets is the SHA-256 of the one the server gets`() {
        val nonce = Nonce.random()
        val expected = MessageDigest.getInstance("SHA-256").digest(nonce.raw.toByteArray()).joinToString("") { "%02x".format(it) }
        assertEquals(expected, nonce.hashed)
        assertTrue(nonce.raw.length >= 43)
        assertTrue(Nonce.random().raw != nonce.raw)
    }

    @Test
    fun `the ID rules say which one is broken`() {
        val cases = mapOf(
            "ada" to null,
            "ada.l_26" to null,
            "ab" to "At least 3 characters.",
            "a".repeat(21) to "At most 20 characters.",
            "ada-l" to "Only letters, numbers, dots and underscores.",
            "Ada" to "Only letters, numbers, dots and underscores.",
        )
        cases.forEach { (handle, problem) -> assertEquals(handle, problem, Handles.problemWith(handle)) }
        assertEquals("ada.l", Handles.normalise("  @Ada.L "))
    }
}
