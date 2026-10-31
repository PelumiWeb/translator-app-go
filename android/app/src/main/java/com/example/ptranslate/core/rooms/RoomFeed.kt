package com.example.ptranslate.core.rooms

import com.example.ptranslate.core.net.BackendException
import com.example.ptranslate.core.net.RoomsClient
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.shareIn

/**
 * The stream of changes to the user's rooms, kept open while any screen
 * listens and reopened when it drops, waiting longer after each failure. One
 * connection serves every screen.
 */
class RoomFeed(
    rooms: RoomsClient,
    scope: CoroutineScope,
    retryDelayMs: (attempt: Int) -> Long = { attempt -> (1000L shl attempt.coerceAtMost(5)).coerceAtMost(30_000) },
) {
    private val _connected = MutableStateFlow(true)

    /** False while the stream is down and being reopened, for the "Reconnecting" notice. */
    val connected: StateFlow<Boolean> = _connected.asStateFlow()

    val changes: SharedFlow<RoomChange> = flow {
        var attempt = 0
        while (true) {
            try {
                rooms.changes().collect { change ->
                    if (change == RoomChange.Ready) {
                        attempt = 0
                        _connected.value = true
                    }
                    emit(change)
                }
            } catch (_: BackendException) {
                // Reopened below, like a stream the server closed.
            }
            _connected.value = false
            delay(retryDelayMs(attempt++))
        }
    }.shareIn(scope, SharingStarted.WhileSubscribed(stopTimeoutMillis = 5_000))
}
