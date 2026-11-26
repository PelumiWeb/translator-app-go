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
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
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
import com.example.ptranslate.PtranslateApp
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

/** A conversation: the messages, newest at the bottom, and the composer: type, or record a voice note. */
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
    // While the room is on the screen its notes appear as they arrive, so they need no notification, and the one
    // that may have brought the person here is done with.
    val notifications = (context.applicationContext as PtranslateApp).container.notifications
    DisposableEffect(lifecycle, roomId) {
        val observer = LifecycleEventObserver { _, event ->
            when (event) {
                Lifecycle.Event.ON_RESUME -> {
                    notifications.roomOnScreen = roomId
                    notifications.cancel(roomId)
                }
                Lifecycle.Event.ON_PAUSE -> notifications.roomOnScreen = null
                else -> Unit
            }
        }
        lifecycle.addObserver(observer)
        onDispose {
            lifecycle.removeObserver(observer)
            notifications.roomOnScreen = null
        }
    }
    val recording = state.note.phase == NotePhase.RECORDING
    LaunchedEffect(recording) {
        haptics.performHapticFeedback(HapticFeedbackType.LongPress)
    }
    // The composer rides above the keyboard while the user types.
    Column(modifier.fillMaxSize().imePadding()) {
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
                        "Say something to ${other.displayName.ifBlank { "them" }}, or write it. They'll get it in ${other.lang?.localName().orEmpty()}, in your voice."
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
                        if (message.isText && message.mine) {
                            MyTextCard(
                                message = message,
                                playing = state.playing == message.id,
                                waiting = message.id in state.waitingToHear,
                                onListen = { viewModel.onListenClicked(message) },
                                onRetry = { viewModel.onRetryClicked(message) },
                            )
                        } else if (message.isText) {
                            TheirTextCard(
                                message = message,
                                myLanguage = viewModel.myLanguage,
                                playing = state.playing == message.id,
                                playingMs = state.playingMs,
                                waiting = message.id in state.waitingToHear,
                                showOriginal = message.id in state.originals,
                                onListen = { viewModel.onListenClicked(message) },
                                onToggleOriginal = { viewModel.onOriginalToggled(message) },
                            )
                        } else if (message.mine) {
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
            val askOrRecord = {
                if (hasMicrophone(context)) viewModel.onMicClicked() else askForMicrophone.launch(Manifest.permission.RECORD_AUDIO)
            }
            when {
                other == null && state.room != null -> Unit
                state.note.phase == NotePhase.RECORDING || state.note.phase == NotePhase.PAUSED -> RecordingComposer(
                    note = state.note,
                    onPause = viewModel::onPauseClicked,
                    onResume = viewModel::onResumeClicked,
                    onStop = viewModel::onStopClicked,
                    onSend = viewModel::onSendNoteClicked,
                    onCancel = viewModel::onCancelClicked,
                )
                state.note.phase == NotePhase.STOPPED -> ReviewComposer(
                    levels = state.reviewLevels,
                    durationMs = state.note.elapsedMs,
                    playing = state.reviewing,
                    onPlay = viewModel::onReviewPlayClicked,
                    onDelete = viewModel::onCancelClicked,
                    onSend = viewModel::onSendNoteClicked,
                )
                else -> {
                    // Without the microphone the user can still write; the banner says how to record.
                    if (state.micDenied) {
                        Banner(
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
                    }
                    TextComposer(
                        draft = state.draft,
                        showMic = !state.micDenied,
                        onDraftChanged = viewModel::onDraftChanged,
                        onSend = viewModel::onSendTextClicked,
                        onMic = askOrRecord,
                    )
                }
            }
        }
    }
}

private fun hasMicrophone(context: android.content.Context) =
    ContextCompat.checkSelfPermission(context, Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED

/** The design's words for each recording that was not sent. No blame, and what to try next. */
private fun RecordingHint.words() = when (this) {
    RecordingHint.TOO_SHORT -> "That was very short. Tap the mic, say something, then send."
    RecordingHint.SILENT -> "We didn't hear anything. Check nothing is covering the microphone, then try again."
    RecordingHint.NO_SPEECH -> "No words came through. Try again, a little closer to the phone."
    RecordingHint.UNCLEAR -> "We couldn't quite make that out. Please say it again."
}

/** Typing a message, with the mic beside it; once there are words, the mic becomes send. */
@Composable
private fun TextComposer(draft: String, showMic: Boolean, onDraftChanged: (String) -> Unit, onSend: () -> Unit, onMic: () -> Unit) {
    val colors = Pearl.colors
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        BasicTextField(
            value = draft,
            onValueChange = onDraftChanged,
            textStyle = PearlType.message.copy(color = colors.fg),
            cursorBrush = SolidColor(colors.fg),
            maxLines = 6,
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences),
            modifier = Modifier.weight(1f).semantics { contentDescription = "Message" },
            decorationBox = { field ->
                Box(
                    Modifier
                        .fillMaxWidth()
                        .heightIn(min = 52.dp)
                        .clip(RoundedCornerShape(26.dp))
                        .background(colors.bgRaised)
                        .padding(horizontal = 18.dp, vertical = 14.dp),
                    contentAlignment = Alignment.CenterStart,
                ) {
                    if (draft.isEmpty()) Text("Message", style = PearlType.message, color = colors.fg3)
                    field()
                }
            },
        )
        when {
            draft.isNotBlank() -> YouButton(PearlIcons.Send, "Send", onSend)
            showMic -> YouButton(PearlIcons.Mic, "Record a voice note", onMic)
        }
    }
}

/** A round button in the colour that means "you". */
@Composable
private fun YouButton(icon: ImageVector, label: String, onClick: () -> Unit, size: Dp = 52.dp) {
    val colors = Pearl.colors
    Box(
        Modifier
            .size(size)
            .clip(CircleShape)
            .background(colors.you)
            .clickable(role = Role.Button, onClickLabel = label, onClick = onClick)
            .semantics { contentDescription = label },
        contentAlignment = Alignment.Center,
    ) {
        Icon(icon, contentDescription = null, tint = colors.onYou, modifier = Modifier.size(size * 0.42f))
    }
}

/**
 * Recording, or paused: the live dots and the time, with the 30-second limit; then cancel, pause or resume, stop
 * (to listen before sending), and send.
 */
@Composable
private fun RecordingComposer(
    note: NoteState,
    onPause: () -> Unit,
    onResume: () -> Unit,
    onStop: () -> Unit,
    onSend: () -> Unit,
    onCancel: () -> Unit,
) {
    val colors = Pearl.colors
    val paused = note.phase == NotePhase.PAUSED
    // The dots move while recording and rest while paused. They do not yet follow the microphone's level.
    val phase by rememberInfiniteTransition(label = "live").animateFloat(
        initialValue = 0f,
        targetValue = 6.28f,
        animationSpec = infiniteRepeatable(tween(1400), RepeatMode.Restart),
        label = "phase",
    )
    val levels = List(16) { i -> if (paused) 0.3f else 0.3f + 0.7f * ((kotlin.math.sin(phase + i * 0.9f) + 1f) / 2f) }
    val left = (RoomViewModel.MAX_NOTE_MS - note.elapsedMs).coerceAtLeast(0)
    val time = if (note.elapsedMs >= RoomViewModel.COUNTDOWN_FROM_MS) "${(left + 999) / 1000} s left" else clock(note.elapsedMs)

    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            DotWaveform(levels = levels, color = colors.fg.copy(alpha = if (paused) 0.4f else 1f), modifier = Modifier.weight(1f))
            Text(
                if (paused) "Paused · $time" else time,
                style = PearlType.meta,
                color = colors.fg,
                modifier = Modifier.semantics { stateDescription = if (paused) "Paused, $time" else "Recording, $time" },
            )
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.Bottom) {
            ComposerControl(PearlIcons.Close, "Cancel", onCancel)
            if (paused) ComposerControl(PearlIcons.Mic, "Resume", onResume) else ComposerControl(PearlIcons.Pause, "Pause", onPause)
            ComposerControl(PearlIcons.Stop, "Stop", onStop)
            SendControl(onSend)
        }
    }
}

/** A stopped note, to listen to before sending: play, its dots and length; then delete or send. */
@Composable
private fun ReviewComposer(
    levels: List<Float>,
    durationMs: Long,
    playing: Boolean,
    onPlay: () -> Unit,
    onDelete: () -> Unit,
    onSend: () -> Unit,
) {
    val colors = Pearl.colors
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Row(
            Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(12.dp))
                .background(colors.bgRaised)
                .padding(horizontal = 12.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            val label = if (playing) "Stop" else "Listen to your note"
            Box(
                Modifier
                    .size(44.dp)
                    .clip(CircleShape)
                    .background(colors.you)
                    .clickable(role = Role.Button, onClickLabel = label, onClick = onPlay)
                    .semantics { contentDescription = label },
                contentAlignment = Alignment.Center,
            ) {
                Icon(if (playing) PearlIcons.Pause else PearlIcons.Play, contentDescription = null, tint = colors.onYou, modifier = Modifier.size(15.dp))
            }
            DotWaveform(levels = levels.ifEmpty { List(22) { 0.3f } }, color = colors.you, modifier = Modifier.weight(1f))
            Text(clock(durationMs), style = PearlType.meta, color = colors.fg2)
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.Bottom) {
            ComposerControl(PearlIcons.Trash, "Delete", onDelete)
            SendControl(onSend)
        }
    }
}

/** An outlined round control with its name under it. */
@Composable
private fun ComposerControl(icon: ImageVector, label: String, onClick: () -> Unit) {
    val colors = Pearl.colors
    Column(
        Modifier
            .clip(RoundedCornerShape(12.dp))
            .clickable(role = Role.Button, onClickLabel = label, onClick = onClick)
            .semantics(mergeDescendants = true) { contentDescription = label }
            .padding(4.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Box(Modifier.size(52.dp).clip(CircleShape).border(1.dp, colors.fg.copy(alpha = 0.28f), CircleShape), contentAlignment = Alignment.Center) {
            Icon(icon, contentDescription = null, tint = colors.fg, modifier = Modifier.size(20.dp))
        }
        Text(label, style = PearlType.meta, color = colors.fg2)
    }
}

/** Send, the composer's main action: the circle that means "you", in its halo. */
@Composable
private fun SendControl(onClick: () -> Unit) {
    val colors = Pearl.colors
    Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp), modifier = Modifier.padding(4.dp)) {
        Box(Modifier.size(72.dp).clip(CircleShape).background(colors.youHalo), contentAlignment = Alignment.Center) {
            Box(
                Modifier
                    .size(56.dp)
                    .clip(CircleShape)
                    .background(colors.you)
                    .clickable(role = Role.Button, onClickLabel = "Send", onClick = onClick)
                    .semantics { contentDescription = "Send" },
                contentAlignment = Alignment.Center,
            ) {
                Icon(PearlIcons.Send, contentDescription = null, tint = colors.onYou, modifier = Modifier.size(24.dp))
            }
        }
        Text("Send", style = PearlType.meta, color = colors.fg)
    }
}
