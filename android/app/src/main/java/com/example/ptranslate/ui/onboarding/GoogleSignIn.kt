package com.example.ptranslate.ui.onboarding

import android.content.Context
import androidx.credentials.CredentialManager
import androidx.credentials.CustomCredential
import androidx.credentials.GetCredentialRequest
import com.example.ptranslate.core.account.Nonce
import com.google.android.libraries.identity.googleid.GetSignInWithGoogleOption
import com.google.android.libraries.identity.googleid.GoogleIdTokenCredential

/**
 * Gets a Google ID token through Android's Credential Manager, which draws its
 * own account sheet. It needs an Activity's context to show it, which is why
 * it lives in the UI and not in core.
 *
 * @param webClientId the Google Cloud project's "web" OAuth client ID, the
 *   audience the server checks tokens against. Empty when this build has none.
 */
class GoogleSignIn(private val webClientId: String) {

    val configured: Boolean get() = webClientId.isNotEmpty()

    /** Shows the sheet and returns the chosen account's ID token, carrying the hash of [nonce]. */
    suspend fun idToken(activity: Context, nonce: Nonce): String {
        val option = GetSignInWithGoogleOption.Builder(webClientId).setNonce(nonce.hashed).build()
        val request = GetCredentialRequest.Builder().addCredentialOption(option).build()
        val credential = CredentialManager.create(activity).getCredential(activity, request).credential
        check(credential is CustomCredential && credential.type == GoogleIdTokenCredential.TYPE_GOOGLE_ID_TOKEN_CREDENTIAL) {
            "Google answered with something other than an ID token"
        }
        return GoogleIdTokenCredential.createFrom(credential.data).idToken
    }
}
