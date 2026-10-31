package com.example.ptranslate.ui

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import android.view.HapticFeedbackConstants
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.speech.SpeechVoice
import com.example.ptranslate.ui.components.LanguageChips
import com.example.ptranslate.ui.components.LanguagePickerSheet
import com.example.ptranslate.ui.components.Notice
import com.example.ptranslate.ui.components.PearlIcons
import com.example.ptranslate.ui.components.RecordControl
import com.example.ptranslate.ui.components.RecordControlState
import com.example.ptranslate.ui.components.ownName
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType
import com.example.ptranslate.ui.theme.PtranslateTheme
import kotlinx.coroutines.delay

/**
 * The interpreter: speak in one language, and it is said in another, in your
 * own voice. Laid out as the design's interpreter: the two languages, what was
 * said and its translation, then the record control and a line saying what is
 * happening, so the screen never looks frozen.
 *
 * @param savedVoiceMs length of the user's saved voice recording, or null if
 *   they have not made one.
 */
@Composable
fun TranslateScreen(
    viewModel: TranslateViewModel,
    savedVoiceMs: Long?,
    onSettingsClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current

    val askForMicrophone = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        if (granted) viewModel.onRecordClicked() else viewModel.onPermissionDenied()
    }

    InterpreterContent(
        state = state,
        hasOwnVoice = savedVoiceMs != null,
        onRecordClick = {
            val granted = ContextCompat.checkSelfPermission(context, Manifest.permission.RECORD_AUDIO) ==
                PackageManager.PERMISSION_GRANTED
            if (granted) viewModel.onRecordClicked() else askForMicrophone.launch(Manifest.permission.RECORD_AUDIO)
        },
        onStopSpeakingClick = viewModel::onStopSpeakingClicked,
        onSourceSelect = viewModel::onSourceSelected,
        onTargetSelect = viewModel::onTargetSelected,
        onSwapClick = viewModel::onSwapLanguagesClicked,
        onSettingsClick = onSettingsClick,
        modifier = modifier,
    )
}

private enum class Side { SOURCE, TARGET }

/** Stateless, so it can be previewed without a ViewModel. */
@Composable
private fun InterpreterContent(
    state: TranslateUiState,
    hasOwnVoice: Boolean,
    onRecordClick: () -> Unit,
    onStopSpeakingClick: () -> Unit,
    onSourceSelect: (Language) -> Unit,
    onTargetSelect: (Language) -> Unit,
    onSwapClick: () -> Unit,
    onSettingsClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val colors = Pearl.colors
    val idle = state.phase == Phase.IDLE
    var picking by remember { mutableStateOf<Side?>(null) }
    val elapsedMs = rememberRecordingTime(state.stage == InterpreterStage.RECORDING)
    RecordingHaptics(state.stage)

    Column(modifier.fillMaxSize().padding(horizontal = 28.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Row(Modifier.fillMaxWidth().padding(top = 4.dp), horizontalArrangement = Arrangement.End) {
            IconButton(onClick = onSettingsClick, enabled = idle) {
                Icon(PearlIcons.Settings, contentDescription = "Settings", tint = colors.fg2)
            }
        }
        LanguageChips(
            source = state.source,
            target = state.target,
            onSourceClick = { picking = Side.SOURCE },
            onTargetClick = { picking = Side.TARGET },
            onSwapClick = onSwapClick,
            enabled = idle,
            swapEnabled = state.target in state.sourceLanguages && state.source in state.targetLanguages,
        )

        // What was said and what it became. Long translations scroll; they are never cut short.
        Box(Modifier.weight(1f).fillMaxWidth(), contentAlignment = Alignment.Center) {
            Column(
                Modifier.verticalScroll(rememberScrollState()).padding(vertical = 24.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                if (state.text.isEmpty() && state.translation.isEmpty() && state.stage == InterpreterStage.IDLE) {
                    Text(
                        "What you say appears here, and under it what they will hear.",
                        style = PearlType.message,
                        color = colors.fg3,
                        textAlign = TextAlign.Center,
                    )
                }
                if (state.text.isNotEmpty()) {
                    Text(state.text, style = PearlType.message, color = colors.fg2, textAlign = TextAlign.Center)
                }
                if (state.translation.isNotEmpty()) {
                    Text(state.translation, style = PearlType.translation, color = colors.them, textAlign = TextAlign.Center)
                }
                state.error?.let { Notice(it) }
            }
        }

        val (controlState, controlLabel) = control(state)
        RecordControl(
            state = controlState,
            label = controlLabel,
            onClick = if (state.stage == InterpreterStage.SPEAKING) onStopSpeakingClick else onRecordClick,
            enabled = state.phase != Phase.WORKING || state.stage == InterpreterStage.SPEAKING,
            value = if (state.stage == InterpreterStage.RECORDING) clock(elapsedMs) else null,
            area = 220.dp,
            circle = 88.dp,
        )
        val (headline, subline) = words(state, hasOwnVoice, elapsedMs)
        Text(headline, style = PearlType.headline, color = colors.fg, textAlign = TextAlign.Center)
        Spacer(Modifier.height(4.dp))
        Text(subline.orEmpty(), style = PearlType.small, color = colors.fg2, textAlign = TextAlign.Center)
        Spacer(Modifier.height(24.dp))
    }

    when (picking) {
        Side.SOURCE -> LanguagePickerSheet(
            title = "You speak",
            options = state.sourceLanguages,
            selected = state.source,
            onSelect = { onSourceSelect(it); picking = null },
            onDismiss = { picking = null },
        )
        Side.TARGET -> LanguagePickerSheet(
            title = "They hear",
            options = state.targetLanguages,
            selected = state.target,
            onSelect = { onTargetSelect(it); picking = null },
            onDismiss = { picking = null },
        )
        null -> Unit
    }
}

private fun control(state: TranslateUiState): Pair<RecordControlState, String> = when (state.stage) {
    InterpreterStage.RECORDING -> RecordControlState.RECORDING to "Stop recording"
    InterpreterStage.PREPARING_SPEECH -> RecordControlState.PREPARING_VOICE to "Preparing the voice"
    InterpreterStage.SPEAKING -> RecordControlState.SPEAKING to "Stop speaking"
    InterpreterStage.IDLE, InterpreterStage.DONE, InterpreterStage.FAILED ->
        RecordControlState.IDLE to "Start recording"
    else -> RecordControlState.WORKING to "Working"
}

/** The headline and the line under it, for each stage of a run. */
private fun words(state: TranslateUiState, hasOwnVoice: Boolean, elapsedMs: Long): Pair<String, String?> {
    val source = state.source.ownName()
    val target = state.target.ownName()
    return when (state.stage) {
        InterpreterStage.IDLE -> "Tap to speak" to "You speak $source. They hear $target."
        InterpreterStage.RECORDING -> "Listening · ${clock(elapsedMs)}" to "Tap to stop when you are done"
        InterpreterStage.PREPARING_AUDIO, InterpreterStage.TRANSCRIBING ->
            "Writing down what you said…" to "Usually a second or two"
        InterpreterStage.UPLOADING -> "Writing down what you said…" to "Sending it to the server"
        InterpreterStage.QUEUED -> "Writing down what you said…" to "Waiting for the server"
        InterpreterStage.RECONNECTING -> "Writing down what you said…" to "Connection lost. Reconnecting"
        InterpreterStage.DOWNLOADING_LANGUAGE -> "Getting $target ready…" to "Only the first time"
        InterpreterStage.TRANSLATING -> "Translating into $target…" to null
        InterpreterStage.PREPARING_SPEECH ->
            (if (hasOwnVoice) "Preparing your voice…" else "Preparing speech…") to "Usually a few seconds"
        InterpreterStage.SPEAKING -> when (state.speakingVoice) {
            SpeechVoice.OWN -> "Speaking in your voice" to "Tap to stop"
            else -> "Speaking in the phone's voice" to
                (if (hasOwnVoice) "Your voice could not be made this time" else "Record your voice in Settings to hear yourself")
        }
        InterpreterStage.DONE -> "Tap to speak again" to "You speak $source. They hear $target."
        InterpreterStage.FAILED -> "Tap to try again" to null
    }
}

/** "0:07", with tabular figures in the type styles so it does not jitter. */
private fun clock(ms: Long): String = "%d:%02d".format(ms / 60_000, ms / 1000 % 60)

/** Counts the time since recording started, for the timer; 0 when not recording. */
@Composable
private fun rememberRecordingTime(recording: Boolean): Long {
    var elapsed by remember { mutableLongStateOf(0L) }
    LaunchedEffect(recording) {
        elapsed = 0
        if (!recording) return@LaunchedEffect
        val started = System.nanoTime()
        while (true) {
            elapsed = (System.nanoTime() - started) / 1_000_000
            delay(200)
        }
    }
    return elapsed
}

/** A light tap when recording starts, a confirmation when it stops, as the design asks. */
@Composable
private fun RecordingHaptics(stage: InterpreterStage) {
    val view = LocalView.current
    var previous by remember { mutableStateOf(stage) }
    LaunchedEffect(stage) {
        if (stage == InterpreterStage.RECORDING && previous != InterpreterStage.RECORDING) {
            view.performHapticFeedback(HapticFeedbackConstants.KEYBOARD_TAP)
        } else if (previous == InterpreterStage.RECORDING && stage != InterpreterStage.RECORDING) {
            val confirm = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) HapticFeedbackConstants.CONFIRM else HapticFeedbackConstants.VIRTUAL_KEY
            view.performHapticFeedback(confirm)
        }
        previous = stage
    }
}

@Preview
@Composable
private fun InterpreterPreparingPreview() {
    PtranslateTheme(darkTheme = true) {
        androidx.compose.foundation.layout.Box(Modifier.fillMaxSize().padding(0.dp)) {
            InterpreterContent(
                state = TranslateUiState(
                    phase = Phase.WORKING,
                    stage = InterpreterStage.PREPARING_SPEECH,
                    text = "Where can I buy a ticket for the night train to Seville?",
                    translation = "¿Dónde puedo comprar un billete para el tren nocturno a Sevilla?",
                ),
                hasOwnVoice = true,
                onRecordClick = {}, onStopSpeakingClick = {}, onSourceSelect = {}, onTargetSelect = {},
                onSwapClick = {}, onSettingsClick = {},
            )
        }
    }
}
