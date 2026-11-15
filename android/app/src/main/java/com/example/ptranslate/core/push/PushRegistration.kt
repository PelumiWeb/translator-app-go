package com.example.ptranslate.core.push

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filterNotNull
import kotlinx.coroutines.launch

/**
 * Keeps the server told where this phone receives notifications (ADR 0026): each time someone signs in, and each
 * time the push service gives the phone a new address. Signing out needs nothing here: the server forgets the
 * address with the session.
 *
 * @param sessions the session's token, or null while no one is signed in.
 * @param register sends the address to the server, for the session signed in now.
 * @param retryDelays how long to wait before each further try, when the server cannot be reached.
 */
class PushRegistration(
    private val service: String,
    private val sessions: Flow<String?>,
    private val register: suspend (service: String, token: String) -> Unit,
    private val scope: CoroutineScope,
    private val retryDelays: List<Long> = listOf(10_000, 60_000, 300_000),
) {
    private val address = MutableStateFlow<String?>(null)

    /** The push service gave this phone [token]: when the app starts, or a new one later. */
    fun onAddress(token: String) {
        address.value = token
    }

    fun start() {
        scope.launch {
            combine(sessions, address) { session, token -> if (session != null && token != null) session to token else null }
                .filterNotNull()
                .distinctUntilChanged()
                // A new session or address while one is still being sent replaces it.
                .collectLatest { (_, token) -> send(token) }
        }
    }

    private suspend fun send(token: String) {
        for (wait in listOf(0L) + retryDelays) {
            delay(wait)
            try {
                register(service, token)
                return
            } catch (e: CancellationException) {
                throw e
            } catch (_: Exception) {
                // Offline, or the server is restarting: try again after the next wait, then at the next launch.
            }
        }
    }
}
