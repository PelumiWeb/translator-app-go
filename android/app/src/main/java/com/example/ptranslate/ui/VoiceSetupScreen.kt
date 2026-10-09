package com.example.ptranslate.ui

import android.Manifest
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.example.ptranslate.core.voice.VoiceSampleRules
import com.example.ptranslate.ui.theme.PtranslateTheme

/**
 * About twenty seconds when read aloud at an ordinary pace. Varied sounds and
 * a natural rhythm give a voice-cloning engine more to work with than a list
 * of words would.
 */
private const val PASSAGE =
    "When I travel, I like to talk with the people I meet about their food, their music, and the places " +
        "they love. A good conversation makes the world feel smaller. I am recording this so that when my " +
        "words are translated, they can still be spoken in my own voice."

/** Where the user records the sample of their voice that translations are spoken in. */
@Composable
fun VoiceSetupScreen(
    onDone: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: VoiceSetupViewModel = viewModel(factory = VoiceSetupViewModel.Factory),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current

    val askForMicrophone = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        if (granted) viewModel.onRecordClicked() else viewModel.onPermissionDenied()
    }

    // Leaving is an event, not a state to stay in: tell the caller once, and
    // reset so the screen starts fresh the next time it is opened.
    LaunchedEffect(state.finished) {
        if (state.finished) {
            onDone()
            viewModel.onLeft()
        }
    }

    VoiceSetupContent(
        state = state,
        onRecordClick = {
            val granted = ContextCompat.checkSelfPermission(context, Manifest.permission.RECORD_AUDIO) ==
                PackageManager.PERMISSION_GRANTED
            if (granted) viewModel.onRecordClicked() else askForMicrophone.launch(Manifest.permission.RECORD_AUDIO)
        },
        onListenClick = viewModel::onListenClicked,
        onUseClick = viewModel::onUseClicked,
        onSkipClick = viewModel::onSkipClicked,
        onDeleteClick = viewModel::onDeleteClicked,
        modifier = modifier,
    )
}

@Composable
private fun VoiceSetupContent(
    state: VoiceSetupUiState,
    onRecordClick: () -> Unit,
    onListenClick: () -> Unit,
    onUseClick: () -> Unit,
    onSkipClick: () -> Unit,
    onDeleteClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text("Record your voice", style = MaterialTheme.typography.headlineSmall)
        Text(
            "Your translations can be spoken in your own voice. To do that, the app needs a short " +
                "recording of you reading the passage below.",
        )
        // Said before the user records, in plain words: what happens to it.
        Text(
            "The recording stays on this phone. When a translation is spoken, it is sent to the server " +
                "together with the text, used once to make the speech, and deleted. You can delete it " +
                "from this phone at any time.",
            style = MaterialTheme.typography.bodySmall,
        )

        Surface(tonalElevation = 2.dp, shape = MaterialTheme.shapes.medium) {
            Text(PASSAGE, style = MaterialTheme.typography.bodyLarge, modifier = Modifier.padding(16.dp))
        }

        when (state.stage) {
            VoiceSetupStage.INTRO -> Intro(state, onRecordClick, onSkipClick, onDeleteClick)
            VoiceSetupStage.RECORDING -> Recording(state, onRecordClick)
            VoiceSetupStage.REVIEW -> Review(state, onRecordClick, onListenClick, onUseClick, onSkipClick)
        }

        state.error?.let {
            Text(text = it, color = MaterialTheme.colorScheme.error)
        }
    }
}

@Composable
private fun Intro(
    state: VoiceSetupUiState,
    onRecordClick: () -> Unit,
    onSkipClick: () -> Unit,
    onDeleteClick: () -> Unit,
) {
    val saved = state.savedMs
    if (saved != null) {
        Text("A recording of your voice (${saved / 1000} s) is saved on this phone.")
    }
    Button(onClick = onRecordClick) {
        Text(if (saved == null) "Start recording" else "Record a new one")
    }
    if (saved == null) {
        TextButton(onClick = onSkipClick) { Text("Not now, use the phone's voice") }
    } else {
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            OutlinedButton(onClick = onSkipClick) { Text("Keep it") }
            TextButton(onClick = onDeleteClick) { Text("Delete my voice") }
        }
    }
}

@Composable
private fun Recording(state: VoiceSetupUiState, onStopClick: () -> Unit) {
    val enough = state.elapsedMs >= VoiceSampleRules.TARGET_MS
    Text(
        text = "Recording: ${state.elapsedMs / 1000} s",
        style = MaterialTheme.typography.titleMedium,
    )
    LinearProgressIndicator(
        progress = { (state.elapsedMs.toFloat() / VoiceSampleRules.TARGET_MS).coerceIn(0f, 1f) },
        modifier = Modifier.fillMaxWidth(),
    )
    Text(
        text = if (enough) "That is enough. You can stop" else "Read the passage aloud, at your normal pace",
        style = MaterialTheme.typography.bodySmall,
    )
    Button(onClick = onStopClick) { Text("Stop") }
}

@Composable
private fun Review(
    state: VoiceSetupUiState,
    onRecordClick: () -> Unit,
    onListenClick: () -> Unit,
    onUseClick: () -> Unit,
    onSkipClick: () -> Unit,
) {
    Text("Recorded ${state.recordedMs / 1000} seconds", style = MaterialTheme.typography.titleMedium)
    state.problem?.let {
        Text(text = it, color = MaterialTheme.colorScheme.error)
    }
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        OutlinedButton(onClick = onListenClick) {
            Text(if (state.playing) "Stop listening" else "Listen")
        }
        OutlinedButton(onClick = onRecordClick) { Text("Record again") }
    }
    Button(onClick = onUseClick, enabled = state.problem == null) {
        Text("Use this recording")
    }
    TextButton(onClick = onSkipClick) {
        Text(if (state.savedMs == null) "Not now, use the phone's voice" else "Cancel, keep my saved voice")
    }
}

@Preview(showBackground = true)
@Composable
private fun VoiceSetupRecordingPreview() {
    PtranslateTheme {
        VoiceSetupContent(
            state = VoiceSetupUiState(stage = VoiceSetupStage.RECORDING, elapsedMs = 12_000),
            onRecordClick = {}, onListenClick = {}, onUseClick = {}, onSkipClick = {}, onDeleteClick = {},
        )
    }
}

@Preview(showBackground = true)
@Composable
private fun VoiceSetupReviewPreview() {
    PtranslateTheme {
        VoiceSetupContent(
            state = VoiceSetupUiState(
                stage = VoiceSetupStage.REVIEW,
                recordedMs = 4_000,
                problem = "That was only 4 seconds. About 20 are needed: please read the whole passage",
            ),
            onRecordClick = {}, onListenClick = {}, onUseClick = {}, onSkipClick = {}, onDeleteClick = {},
        )
    }
}
