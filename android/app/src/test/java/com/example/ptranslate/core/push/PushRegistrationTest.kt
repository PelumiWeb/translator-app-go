package com.example.ptranslate.core.push

import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.IOException

@OptIn(ExperimentalCoroutinesApi::class)
class PushRegistrationTest {
    private val session = MutableStateFlow<String?>(null)
    private val sent = mutableListOf<String>() // "session service token"
    private var failures = 0

    // backgroundScope: the registration's work is cancelled when each test ends.
    private fun TestScope.registration(): PushRegistration {
        val registration = PushRegistration(
            service = "fcm",
            sessions = session,
            register = { service, token ->
                if (failures > 0) {
                    failures--
                    throw IOException("offline")
                }
                sent += "${session.value} $service $token"
            },
            scope = backgroundScope,
            retryDelays = listOf(1_000, 5_000),
        )
        registration.start()
        return registration
    }

    @Test
    fun `nothing is sent until someone is signed in and the phone has an address`() = runTest {
        val registration = registration()
        registration.onAddress("phone-1")
        runCurrent()
        assertEquals(emptyList<String>(), sent)

        session.value = "s1"
        runCurrent()
        assertEquals(listOf("s1 fcm phone-1"), sent)
    }

    @Test
    fun `a new sign-in and a new address are each sent`() = runTest {
        session.value = "s1"
        val registration = registration()
        registration.onAddress("phone-1")
        runCurrent()

        registration.onAddress("phone-2")
        runCurrent()
        session.value = null
        runCurrent()
        session.value = "s2"
        runCurrent()

        assertEquals(listOf("s1 fcm phone-1", "s1 fcm phone-2", "s2 fcm phone-2"), sent)
    }

    @Test
    fun `it tries again while the server cannot be reached, then waits for the next launch`() = runTest {
        session.value = "s1"
        failures = 1
        val registration = registration()
        registration.onAddress("phone-1")
        runCurrent()
        assertEquals(emptyList<String>(), sent)

        advanceTimeBy(1_001)
        assertEquals(listOf("s1 fcm phone-1"), sent)

        // Three failures use up the first try and both retries.
        failures = 3
        registration.onAddress("phone-2")
        advanceTimeBy(10_000)
        assertEquals(listOf("s1 fcm phone-1"), sent)
    }
}
