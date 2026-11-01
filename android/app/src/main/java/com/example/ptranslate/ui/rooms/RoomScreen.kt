package com.example.ptranslate.ui.rooms

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.provider.Settings
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.CenterAlignedTopAppBar
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.example.ptranslate.R
import com.example.ptranslate.ui.components.Banner
import com.example.ptranslate.ui.components.DotWaveform
import com.example.ptranslate.ui.components.Hint
import com.example.ptranslate.ui.components.Notice
import com.example.ptranslate.ui.components.PearlIcons
import com.example.ptranslate.ui.components.Toast
import com.example.ptranslate.ui.components.localName
import com.example.ptranslate.ui.components.ownName
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType

/** A conversation: the notes, newest at the bottom, and the mic. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RoomScreen(
    roomId: String,
    onBack: () -> Unit,
    onRecordVoice: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: RoomViewModel = viewModel(key = "room-$roomId", factory = RoomViewModel.factory(roomId)),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val colors = Pearl.colors
    val context = LocalContext.current
    val haptics = LocalHapticFeedback.current
    val other = state.room?.other
    val name = other?.displayName?.ifBlank { null } ?: other?.handle?.let { "@$it" } ?: if (state.room != null) "Deleted account" else ""

    val askForMicrophone = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (granted) {
            viewModel.onPermissionGranted()
            viewModel.onMicClicked()
        } else {
            viewModel.onPermissionDenied()
        }
    }
    // Coming back from Settings with the microphone allowed puts the mic back.
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    DisposableEffect(lifecycle) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_RESUME && hasMicrophone(context)) viewModel.onPermissionGranted()
        }
        lifecycle.addObserver(observer)
        onDispose { lifecycle.removeObserver(observer) }
    }
    LaunchedEffect(state.recording) {
        haptics.performHapticFeedback(HapticFeedbackType.LongPress)
    }

    Column(modifier.fillMaxSize()) {
        CenterAlignedTopAppBar(
            title = {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(name, style = PearlType.title, color = colors.fg)
                    other?.lang?.let { Text("hears ${it.ownName()}", style = PearlType.meta, color = colors.them) }
                }
            },
            navigationIcon = {
                IconButton(onClick = onBack) {
                    Icon(PearlIcons.Back, contentDescription = "Back", tint = colors.fg, modifier = Modifier.size(24.dp))
                }
            },
            colors = TopAppBarDefaults.centerAlignedTopAppBarColors(containerColor = colors.bg),
        )

        Box(Modifier.weight(1f).fillMaxWidth()) {
            val list = rememberLazyListState()
            val count = state.messages.size + state.pending.size
            LaunchedEffect(count) {
                if (count > 0) list.animateScrollToItem(count - 1)
            }
            if (!state.loading && count == 0) {
                Text(
                    if (other != null) {
                        "Say something to ${other.displayName.ifBlank { "them" }}. They'll hear it in ${other.lang?.localName().orEmpty()}, in your voice."
                    } else {
                        "No one else is in this room."
                    },
                    style = PearlType.message,
                    color = colors.fg2,
                    textAlign = TextAlign.Center,
                    modifier = Modifier.align(Alignment.Center).padding(horizontal = 40.dp),
                )
            }
            LazyColumn(
                state = list,
                contentPadding = PaddingValues(start = 12.dp, end = 12.dp, top = 16.dp, bottom = 16.dp),
                verticalArrangement = Arrangement.spacedBy(14.dp),
                modifier = Modifier.fillMaxSize(),
            ) {
                items(state.messages, key = { it.id }) { message ->
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = if (message.mine) Arrangement.End else Arrangement.Start) {
                        if (message.mine) {
                            MyNoteCard(
                                message = message,
                                playing = state.playing == message.id,
                                playingMs = state.playingMs,
                                onPlay = { viewModel.onPlayClicked(message) },
                                onRetry = { viewModel.onRetryClicked(message) },
                                onRecordVoice = onRecordVoice,
                            )
                        } else {
                            TheirNoteCard(
                                message = message,
                                myLanguage = viewModel.myLanguage,
                                playing = state.playing == message.id,
                                playingMs = state.playingMs,
                                fresh = message.id in state.fresh,
                                played = message.id in state.played,
                                showOriginal = message.id in state.originals,
                                onPlay = { viewModel.onPlayClicked(message) },
                                onToggleOriginal = { viewModel.onOriginalToggled(message) },
                            )
                        }
                    }
                }
                items(state.pending, key = { it.id }) { note ->
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                        PendingNoteCard(note, onRetry = { viewModel.onRetryPending(note) })
                    }
                }
            }
            if (!state.connected) {
                Toast("Connection lost. Reconnecting…", Modifier.align(Alignment.TopCenter).padding(top = 8.dp))
            }
        }

        Column(
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            state.problem?.let { Notice(it) }
            state.hint?.let { Hint(it.words(), icon = if (it == RecordingHint.SILENT) PearlIcons.Ear else PearlIcons.Retry) }
            when {
                other == null && state.room != null -> Unit
                state.micDenied -> Banner(
                    icon = PearlIcons.MicOff,
                    title = "${stringResource(R.string.app_name)} needs your microphone",
                    body = "To record, allow microphone access in Settings.",
                    action = "Open Settings",
                    onAction = {
                        context.startActivity(
                            Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.fromParts("package", context.packageName, null)),
                        )
                    },
                )
                state.recording -> RecordingComposer(state.elapsedMs, onStop = viewModel::onMicClicked, onCancel = viewModel::onCancelClicked)
                else -> IdleMic(onClick = {
                    if (hasMicrophone(context)) viewModel.onMicClicked() else askForMicrophone.launch(Manifest.permission.RECORD_AUDIO)
                })
            }
        }
    }
}

private fun hasMicrophone(context: android.content.Context) =
    ContextCompat.checkSelfPermission(context, Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED

/** The design's words for each recording that was not sent. No blame, and what to try next. */
private fun RecordingHint.words() = when (this) {
    RecordingHint.TOO_SHORT -> "That was very short. Tap, say something, then tap again to send."
    RecordingHint.SILENT -> "We didn't hear anything. Check nothing is covering the microphone, then try again."
    RecordingHint.NO_SPEECH -> "No words came through. Try again, a little closer to the phone."
    RecordingHint.UNCLEAR -> "We couldn't quite make that out. Please say it again."
}

/** The composer mic: a still halo around the circle that means "you". Tap to start. */
@Composable
private fun IdleMic(onClick: () -> Unit) {
    val colors = Pearl.colors
    Box(Modifier.size(96.dp).clip(CircleShape).background(colors.youHalo), contentAlignment = Alignment.Center) {
        Box(
            Modifier
                .size(68.dp)
                .clip(CircleShape)
                .background(colors.you)
                .clickable(role = Role.Button, onClickLabel = "Record a voice note", onClick = onClick)
                .semantics { contentDescription = "Record a voice note" },
            contentAlignment = Alignment.Center,
        ) {
            Icon(PearlIcons.Mic, contentDescription = null, tint = colors.onYou, modifier = Modifier.size(26.dp))
        }
    }
}

/** Recording: cancel, the live dots, and stop to send, with the time and the 30-second limit. */
@Composable
private fun RecordingComposer(elapsedMs: Long, onStop: () -> Unit, onCancel: () -> Unit) {
    val colors = Pearl.colors
    // The dots move while recording. They do not yet follow the microphone's level.
    val phase by rememberInfiniteTransition(label = "live").animateFloat(
        initialValue = 0f,
        targetValue = 6.28f,
        animationSpec = infiniteRepeatable(tween(1400), RepeatMode.Restart),
        label = "phase",
    )
    val levels = List(16) { i -> 0.3f + 0.7f * ((kotlin.math.sin(phase + i * 0.9f) + 1f) / 2f) }
    val left = (RoomViewModel.MAX_NOTE_MS - elapsedMs).coerceAtLeast(0)
    val time = if (elapsedMs >= RoomViewModel.COUNTDOWN_FROM_MS) "${(left + 999) / 1000} s left" else clock(elapsedMs)

    Column(Modifier.fillMaxWidth()) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Box(
                Modifier
                    .size(44.dp)
                    .clip(CircleShape)
                    .border(1.dp, colors.fg.copy(alpha = 0.28f), CircleShape)
                    .clickable(role = Role.Button, onClickLabel = "Cancel", onClick = onCancel)
                    .semantics { contentDescription = "Cancel" },
                contentAlignment = Alignment.Center,
            ) {
                Icon(PearlIcons.Close, contentDescription = null, tint = colors.fg, modifier = Modifier.size(18.dp))
            }
            DotWaveform(levels = levels, color = colors.fg, modifier = Modifier.weight(1f))
            Box(Modifier.size(84.dp).clip(CircleShape).background(colors.youHalo), contentAlignment = Alignment.Center) {
                Box(
                    Modifier
                        .size(64.dp)
                        .clip(CircleShape)
                        .background(colors.you)
                        .clickable(role = Role.Button, onClickLabel = "Send", onClick = onStop)
                        .semantics {
                            contentDescription = "Stop and send"
                            stateDescription = time
                        },
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(PearlIcons.Stop, contentDescription = null, tint = colors.onYou, modifier = Modifier.size(22.dp))
                }
            }
        }
        Row(Modifier.fillMaxWidth().padding(top = 4.dp)) {
            Text("Cancel", style = PearlType.meta, color = colors.fg2, modifier = Modifier.weight(1f))
            Text("$time · tap to send", style = PearlType.meta, color = colors.fg)
        }
    }
}
