package com.example.ptranslate.core.model

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder

class Sha256Test {

    @get:Rule
    val folder = TemporaryFolder()

    private fun fileWith(content: ByteArray) = folder.newFile().apply { writeBytes(content) }

    // The first two are the standard test vectors published with SHA-256.
    @Test
    fun `matches the published test vectors`() {
        assertEquals(
            "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
            Sha256.ofFile(fileWith(ByteArray(0))),
        )
        assertEquals(
            "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
            Sha256.ofFile(fileWith("abc".toByteArray())),
        )
    }

    @Test
    fun `hashes a file larger than its read buffer`() {
        // One million "a"s, another published vector, read in 64 KB chunks.
        val file = fileWith(ByteArray(1_000_000) { 'a'.code.toByte() })

        assertEquals("cdc76e5c9914fb9281a1c7e284d73e67f1809a48a497200e046d39ccc7112cd0", Sha256.ofFile(file))
    }

    @Test
    fun `one changed byte changes the hash`() {
        val original = ByteArray(100_000) { (it % 251).toByte() }
        val corrupted = original.copyOf().also { it[50_000] = (it[50_000] + 1).toByte() }

        assertNotEquals(Sha256.ofFile(fileWith(original)), Sha256.ofFile(fileWith(corrupted)))
    }
}
