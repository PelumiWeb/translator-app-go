package com.example.ptranslate.core.account

import android.content.SharedPreferences
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import java.security.GeneralSecurityException
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * Keeps the session in [preferences], with the token encrypted by a key that
 * lives in the Android Keystore and never leaves it (ADR 0018). A copy of the
 * app's files is no use without the phone itself. The profile is not secret
 * and is kept as it is, so the app can open without waiting for the network.
 */
class KeystoreSessionStore(private val preferences: SharedPreferences) : SessionStore {

    private val json = Json { ignoreUnknownKeys = true }

    override fun load(): Session? {
        val sealed = preferences.getString(TOKEN, null) ?: return null
        val profileJson = preferences.getString(PROFILE, null) ?: return null
        return try {
            Session(token = open(sealed), profile = json.decodeFromString<Profile>(profileJson))
        } catch (_: GeneralSecurityException) {
            // The key is gone, as after a restore onto another phone: the
            // token cannot be read, so the person signs in again.
            null
        } catch (_: SerializationException) {
            null
        } catch (_: IllegalArgumentException) {
            null // not valid Base64
        }
    }

    override fun save(session: Session) {
        preferences.edit()
            .putString(TOKEN, seal(session.token))
            .putString(PROFILE, json.encodeToString(session.profile))
            .apply()
    }

    override fun clear() {
        preferences.edit().remove(TOKEN).remove(PROFILE).apply()
    }

    /** Encrypts with AES-GCM; the result is the IV and the ciphertext, together, in Base64. */
    private fun seal(text: String): String {
        val cipher = Cipher.getInstance(TRANSFORMATION).apply { init(Cipher.ENCRYPT_MODE, key()) }
        val sealed = cipher.iv + cipher.doFinal(text.toByteArray())
        return Base64.encodeToString(sealed, Base64.NO_WRAP)
    }

    private fun open(sealed: String): String {
        val bytes = Base64.decode(sealed, Base64.NO_WRAP)
        val cipher = Cipher.getInstance(TRANSFORMATION).apply {
            init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, bytes, 0, IV_BYTES))
        }
        return String(cipher.doFinal(bytes, IV_BYTES, bytes.size - IV_BYTES))
    }

    private fun key(): SecretKey {
        val keyStore = KeyStore.getInstance(KEYSTORE).apply { load(null) }
        (keyStore.getEntry(KEY_ALIAS, null) as? KeyStore.SecretKeyEntry)?.let { return it.secretKey }
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, KEYSTORE).apply {
            init(
                KeyGenParameterSpec.Builder(KEY_ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .build(),
            )
        }.generateKey()
    }

    private companion object {
        const val KEYSTORE = "AndroidKeyStore"
        const val KEY_ALIAS = "session"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val IV_BYTES = 12
        const val TOKEN = "session_token"
        const val PROFILE = "session_profile"
    }
}
