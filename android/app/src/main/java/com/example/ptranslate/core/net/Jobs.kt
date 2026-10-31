package com.example.ptranslate.core.net

import kotlinx.coroutines.delay

/**
 * Runs [call] until it succeeds, waiting [retryDelayMs] between tries. Gives
 * up at once when the server refused the request, since the same request gets
 * the same answer, and after [maxAttempts] tries in all.
 *
 * @throws BackendException the last failure.
 */
suspend fun <T> retrying(maxAttempts: Int, retryDelayMs: (attempt: Int) -> Long, call: suspend () -> T): T {
    var attempt = 1
    while (true) {
        try {
            return call()
        } catch (e: BackendException) {
            if (!e.retryable || attempt >= maxAttempts) throw e
        }
        delay(retryDelayMs(attempt))
        attempt++
    }
}

/**
 * Follows a job to its end, reconnecting when the stream breaks. Each
 * reconnect tells the server the last event already received, and the server
 * continues from the next one, so nothing is repeated or skipped.
 *
 * [onEvent] sees each event once, in order, up to and including
 * [JobEvent.Done]. [onReconnecting] runs before each new connection.
 *
 * @throws BackendException when the job failed (with its message), when the
 *   server refused the stream, or after [maxAttempts] connections in a row
 *   that brought nothing new.
 */
suspend fun BackendClient.followJob(
    jobId: String,
    maxAttempts: Int,
    retryDelayMs: (attempt: Int) -> Long,
    onReconnecting: suspend () -> Unit = {},
    onEvent: suspend (JobEvent) -> Unit,
) {
    var lastSeq = 0
    var failures = 0 // in a row, without any new event in between

    while (true) {
        var finished = false
        val seqBefore = lastSeq
        try {
            jobEvents(jobId, afterSeq = lastSeq).collect { update ->
                // A guard for a server that replays more than it was asked
                // to: never pass on the same event twice.
                if (update.seq <= lastSeq) return@collect
                lastSeq = update.seq

                when (val event = update.event) {
                    // The job itself failed. Reconnecting would only replay
                    // the same ending.
                    is JobEvent.Failed -> throw BackendException(event.message)
                    is JobEvent.Done -> {
                        finished = true
                        onEvent(event)
                    }
                    else -> onEvent(event)
                }
            }
        } catch (e: BackendException) {
            if (!e.retryable) throw e
        }
        if (finished) return

        // The stream ended, cleanly or not, before the job did. The server
        // closes a stream only after "done" or "error", so this was the
        // connection, a server restart, or a client that fell behind. In
        // every case the answer is the same: ask again.
        if (lastSeq > seqBefore) failures = 0 // it was making progress
        failures++
        if (failures >= maxAttempts) {
            throw BackendException("The connection to the server was lost")
        }
        onReconnecting()
        delay(retryDelayMs(failures))
    }
}
