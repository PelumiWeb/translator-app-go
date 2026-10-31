package com.example.ptranslate.ui

import android.Manifest
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.togetherWith
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
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.example.ptranslate.core.voice.VoiceSampleRules
import com.example.ptranslate.ui.components.DotWaveform
import com.example.ptranslate.ui.components.LinkButton
import com.example.ptranslate.ui.components.Notice
import com.example.ptranslate.ui.components.PearlIcons
import com.example.ptranslate.ui.components.PrimaryButton
import com.example.ptranslate.ui.components.RecordControl
import com.example.ptranslate.ui.components.RecordControlState
import com.example.ptranslate.ui.components.RoundIconButton
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType
import com.example.ptranslate.ui.theme.PtranslateTheme

/**
 * About twenty seconds when read aloud at an ordinary pace. Varied sounds and
 * a natural rhythm give a voice-cloning engine more to work with than a list
 * of words would. Split into the lines the teleprompter shows; joined, it must
 * stay word for word the same as on iOS and on the server
 * (`provider.OnboardingPassage`), which tells the voice engine what was said.
 */
private val PASSAGE_LINES = listOf(
    "When I travel, I like to talk with the people I meet",
    "about their food, their music, and the places they love.",
    "A good conversation makes the world feel smaller.",
    "I am recording this so that when my words are translated,",
    "they can still be spoken in my own voice.",
)

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
        modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 28.dp, vertical = 24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        when (state.stage) {
            VoiceSetupStage.INTRO -> Intro(state, onRecordClick, onSkipClick, onDeleteClick)
            VoiceSetupStage.RECORDING -> Recording(state, onRecordClick, onSkipClick)
            VoiceSetupStage.REVIEW -> Review(state, onRecordClick, onListenClick, onUseClick, onSkipClick)
        }
        state.error?.let { Notice(it) }
    }
}

@Composable
private fun Intro(state: VoiceSetupUiState, onRecordClick: () -> Unit, onSkipClick: () -> Unit, onDeleteClick: () -> Unit) {
    val colors = Pearl.colors
    val saved = state.savedMs
    Text("Your voice", style = PearlType.label, color = colors.fg)
    Text(
        if (saved == null) "Be heard in your own voice" else "Your voice is recorded",
        style = PearlType.title,
        color = colors.fg,
        textAlign = TextAlign.Center,
    )
    Text(
        if (saved == null) {
            "Read the lines below aloud, at your normal pace. About 20 seconds."
        } else {
            "${saved / 1000} seconds, kept on this phone. Record a new one, keep it, or delete it."
        },
        style = PearlType.body,
        color = colors.fg2,
        textAlign = TextAlign.Center,
    )
    Teleprompter(current = 0, modifier = Modifier.padding(vertical = 16.dp))
    RecordControl(
        state = RecordControlState.IDLE,
        label = if (saved == null) "Start recording" else "Record a new one",
        onClick = onRecordClick,
    )
    if (saved == null) {
        LinkButton("Not now, use the phone's voice", onClick = onSkipClick)
    } else {
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            LinkButton("Keep it", onClick = onSkipClick)
            LinkButton("Delete my voice", onClick = onDeleteClick)
        }
    }
    // Said before the user records, in plain words: what happens to it.
    Text(
        "The recording stays on this phone. When a translation is spoken, it goes with the text to " +
            "our server, and from there to Fish Audio, the service that makes the speech. Our server " +
            "deletes it straight after. You can delete it from this phone at any time.",
        style = PearlType.meta,
        color = colors.fg2,
        textAlign = TextAlign.Center,
        modifier = Modifier.padding(top = 8.dp),
    )
}

@Composable
private fun Recording(state: VoiceSetupUiState, onStopClick: () -> Unit, onSkipClick: () -> Unit) {
    val colors = Pearl.colors
    val elapsed = state.elapsedMs
    Text("Listening", style = PearlType.label, color = colors.fg)
    Text(clock(elapsed), style = PearlType.timer, color = colors.fg)
    Text(
        when {
            elapsed < VoiceSampleRules.MIN_MS -> "Keep going. A few more seconds."
            elapsed < VoiceSampleRules.TARGET_MS -> "Keep going. Read to the end."
            else -> "That's enough. Tap to stop."
        },
        style = PearlType.small,
        color = colors.fg2,
    )
    Teleprompter(current = lineAt(elapsed), modifier = Modifier.padding(vertical = 24.dp))
    RecordControl(state = RecordControlState.RECORDING, label = "Stop recording", onClick = onStopClick, value = clock(elapsed))
    LinkButton("Not now, use the phone's voice", onClick = onSkipClick)
}

@Composable
private fun Review(
    state: VoiceSetupUiState,
    onRecordClick: () -> Unit,
    onListenClick: () -> Unit,
    onUseClick: () -> Unit,
    onSkipClick: () -> Unit,
) {
    val colors = Pearl.colors
    // While it plays, the played part of the waveform fills in.
    val played = remember { Animatable(1f) }
    LaunchedEffect(state.playing) {
        if (state.playing) {
            played.snapTo(0f)
            played.animateTo(1f, tween(state.recordedMs.toInt().coerceAtLeast(1), easing = LinearEasing))
        } else {
            played.snapTo(1f)
        }
    }

    Text("Recorded", style = PearlType.label, color = colors.fg)
    Text(clock(state.recordedMs), style = PearlType.timer, color = colors.fg)
    Text(
        if (state.problem == null) "Listen back, then keep it." else "This one cannot be kept.",
        style = PearlType.small,
        color = colors.fg2,
    )
    Spacer(Modifier.height(16.dp))
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        RoundIconButton(
            icon = PearlIcons.Play,
            label = if (state.playing) "Stop listening" else "Listen",
            onClick = onListenClick,
        )
        DotWaveform(levels = state.levels, color = colors.you, played = played.value, modifier = Modifier.weight(1f))
    }
    state.problem?.let { Notice(it, Modifier.padding(top = 8.dp)) }
    Spacer(Modifier.height(16.dp))
    PrimaryButton("Use this recording", onClick = onUseClick, enabled = state.problem == null)
    LinkButton("Record again", onClick = onRecordClick)
    LinkButton(if (state.savedMs == null) "Not now, use the phone's voice" else "Cancel, keep my saved voice", onClick = onSkipClick)
}

/**
 * The passage as a teleprompter: the line to read now, large, between the
 * line just read and the one coming next.
 */
@Composable
private fun Teleprompter(current: Int, modifier: Modifier = Modifier) {
    val colors = Pearl.colors
    AnimatedContent(
        targetState = current,
        transitionSpec = {
            (slideInVertically { it / 3 } + fadeIn()) togetherWith (slideOutVertically { -it / 3 } + fadeOut())
        },
        label = "teleprompter",
        modifier = modifier.fillMaxWidth(),
    ) { line ->
        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(14.dp)) {
            Box(Modifier.height(48.dp), contentAlignment = Alignment.BottomCenter) {
                PASSAGE_LINES.getOrNull(line - 1)?.let {
                    Text(it, style = PearlType.passageOther, color = colors.fg4, textAlign = TextAlign.Center)
                }
            }
            Text(PASSAGE_LINES[line], style = PearlType.passageCurrent, color = colors.fg, textAlign = TextAlign.Center)
            Box(Modifier.height(48.dp), contentAlignment = Alignment.TopCenter) {
                PASSAGE_LINES.getOrNull(line + 1)?.let {
                    Text(it, style = PearlType.passageOther, color = colors.fg3, textAlign = TextAlign.Center)
                }
            }
        }
    }
}

/**
 * Which line the reader has probably reached: the passage is paced to take
 * the target time, each line its share by length. There is no listening for
 * the words; a reader ahead or behind still has the lines around it.
 */
private fun lineAt(elapsedMs: Long): Int {
    val total = PASSAGE_LINES.sumOf { it.length }
    val reached = elapsedMs.toDouble() / VoiceSampleRules.TARGET_MS * total
    var end = 0
    PASSAGE_LINES.forEachIndexed { index, line ->
        end += line.length
        if (reached < end) return index
    }
    return PASSAGE_LINES.lastIndex
}

private fun clock(ms: Long): String = "%d:%02d".format(ms / 60_000, ms / 1000 % 60)

@Preview
@Composable
private fun VoiceSetupRecordingPreview() {
    PtranslateTheme(darkTheme = true) {
        VoiceSetupContent(
            state = VoiceSetupUiState(stage = VoiceSetupStage.RECORDING, elapsedMs = 12_000),
            onRecordClick = {}, onListenClick = {}, onUseClick = {}, onSkipClick = {}, onDeleteClick = {},
        )
    }
}

@Preview
@Composable
private fun VoiceSetupReviewPreview() {
    PtranslateTheme(darkTheme = true) {
        VoiceSetupContent(
            state = VoiceSetupUiState(
                stage = VoiceSetupStage.REVIEW,
                recordedMs = 4_000,
                levels = List(22) { (it % 5) / 4f },
                problem = "That was only 4 seconds. About 20 are needed: please read the whole passage",
            ),
            onRecordClick = {}, onListenClick = {}, onUseClick = {}, onSkipClick = {}, onDeleteClick = {},
        )
    }
}
