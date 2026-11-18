package com.example.ptranslate.core.audio

import android.annotation.SuppressLint
import android.media.AudioFormat
import android.media.AudioRecord
import android.media.MediaRecorder
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlin.concurrent.thread

/**
 * Records 16 kHz mono PCM with [AudioRecord].
 *
 * Capture stops by itself after [maxDurationMs], one Whisper window, so a
 * forgotten recording cannot grow without bound or hold the microphone.
 */
class AndroidAudioRecorder(
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
    private val maxDurationMs: Long = 30_000,
) : AudioRecorder {

    // The permission is requested by the UI before start() is ever called;
    // without it the constructor below throws SecurityException.
    @SuppressLint("MissingPermission")
    override fun start(): Recording {
        val minBufferBytes = AudioRecord.getMinBufferSize(
            PcmAudio.SAMPLE_RATE_HZ,
            AudioFormat.CHANNEL_IN_MONO,
            AudioFormat.ENCODING_PCM_16BIT,
        )
        check(minBufferBytes > 0) { "This device cannot record 16 kHz mono audio" }

        val record = AudioRecord(
            // Tuned for speech recognition: no gain control or other effects
            // that help phone calls but hurt transcription.
            MediaRecorder.AudioSource.VOICE_RECOGNITION,
            PcmAudio.SAMPLE_RATE_HZ,
            AudioFormat.CHANNEL_IN_MONO,
            AudioFormat.ENCODING_PCM_16BIT,
            minBufferBytes * 2,
        )
        if (record.state != AudioRecord.STATE_INITIALIZED) {
            record.release()
            error("Could not open the microphone")
        }

        val maxSamples = (maxDurationMs * PcmAudio.SAMPLE_RATE_HZ / 1000).toInt()
        return ActiveRecording(record, maxSamples, ioDispatcher)
    }
}

private class ActiveRecording(
    private val record: AudioRecord,
    maxSamples: Int,
    private val ioDispatcher: CoroutineDispatcher,
) : Recording {

    // Written by the capture thread, read by others only after join().
    private val samples = ShortArray(maxSamples)
    private var count = 0

    private enum class State { RECORDING, PAUSED, ENDED }

    // The capture thread waits on the lock while paused; pause, resume and the end change the state under it.
    private val lock = Object()
    private var state = State.RECORDING

    // AudioRecord.read blocks, so capture gets a thread of its own instead of
    // occupying a coroutine dispatcher thread for up to 30 seconds. It is the
    // only thread that starts or stops the AudioRecord.
    private val captureThread = thread(name = "audio-capture") {
        val chunk = ShortArray(1024)
        try {
            record.startRecording()
            while (count < samples.size && awaitRecording()) {
                val read = record.read(chunk, 0, minOf(chunk.size, samples.size - count))
                if (read <= 0) break
                chunk.copyInto(samples, destinationOffset = count, endIndex = read)
                count += read
            }
        } finally {
            // The thread that opened the microphone always closes it.
            runCatching { record.stop() }
            record.release()
        }
    }

    /** Waits out a pause with the microphone stopped. False once the recording has ended. */
    private fun awaitRecording(): Boolean {
        synchronized(lock) {
            when (state) {
                State.RECORDING -> return true
                State.ENDED -> return false
                State.PAUSED -> Unit
            }
        }
        runCatching { record.stop() }
        synchronized(lock) {
            while (state == State.PAUSED) lock.wait()
            if (state == State.ENDED) return false
        }
        // Another app may hold the microphone now; the recording then ends with what it has.
        return runCatching { record.startRecording() }.isSuccess &&
            record.recordingState == AudioRecord.RECORDSTATE_RECORDING
    }

    override fun pause() = change(State.PAUSED)

    override fun resume() = change(State.RECORDING)

    override suspend fun stop(): PcmAudio {
        end()
        withContext(ioDispatcher) { captureThread.join() }
        return PcmAudio(samples.copyOf(count))
    }

    override fun cancel() = end()

    private fun change(to: State) = synchronized(lock) {
        if (state != State.ENDED) {
            state = to
            lock.notifyAll()
        }
    }

    private fun end() = synchronized(lock) {
        state = State.ENDED
        lock.notifyAll()
    }
}
