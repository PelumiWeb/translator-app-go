package com.example.ptranslate.core.account

import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Base64

/**
 * A nonce for one sign-in. The provider is given [hashed], and writes it into
 * the token it issues; the server is given [raw], hashes it, and checks the two
 * match (ADR 0018). A token caught on the way is useless without the raw nonce.
 */
class Nonce private constructor(val raw: String) {
    /** SHA-256 of [raw], as lower-case hex. */
    val hashed: String = MessageDigest.getInstance("SHA-256")
        .digest(raw.toByteArray())
        .joinToString("") { "%02x".format(it) }

    companion object {
        fun random(): Nonce {
            val bytes = ByteArray(32).also(SecureRandom()::nextBytes)
            return Nonce(Base64.getUrlEncoder().withoutPadding().encodeToString(bytes))
        }

        internal fun of(raw: String) = Nonce(raw)
    }
}
