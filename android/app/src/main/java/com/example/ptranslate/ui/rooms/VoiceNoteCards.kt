package com.example.ptranslate.ui.rooms

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
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
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.scale
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.rooms.Message
import com.example.ptranslate.core.rooms.Rendition
import com.example.ptranslate.core.rooms.RenditionStatus
import com.example.ptranslate.core.rooms.RenditionVoice
import com.example.ptranslate.ui.components.DotWaveform
import com.example.ptranslate.ui.components.PearlIcons
import com.example.ptranslate.ui.components.localName
import com.example.ptranslate.ui.components.ownName
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType
import kotlin.math.abs
import kotlin.math.sin

// The voice note cards, in every state of the design's board: the user's in
// warm white whatever happens to them, the other person's in raised petrol
// with mint.

private val CardShape = RoundedCornerShape(12.dp)

/** Dots for a note whose own waveform is not known: steady, varied, the same each time for the same note. */
fun dotsFor(id: String): List<Float> {
    val seed = id.hashCode()
    return List(22) { i -> (0.25f + 0.75f * abs(sin((seed % 97 + i * 1.7f).toDouble())).toFloat()) }
}

/** The user's note, from the moment the server has it. */
@Composable
fun MyNoteCard(
    message: Message,
    playing: Boolean,
    playingMs: Long?,
    onPlay: () -> Unit,
    onRetry: () -> Unit,
    onRecordVoice: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val colors = Pearl.colors
    val delivery = message.delivery
    val listenerLanguage = delivery?.lang?.let { Language(it).localName() } ?: ""
    MyCard(modifier) {
        if (delivery?.status == RenditionStatus.READY && delivery.voice != RenditionVoice.PHONE) {
            PlayRow(
                playing = playing,
                durationMs = if (playing) playingMs ?: message.durationMs else message.durationMs,
                dots = dotsFor(message.id),
                fill = colors.onYou,
                icon = colors.you,
                dotColor = colors.onYou,
                onPlay = onPlay,
                label = "Play what they hear",
            )
        }
        Text(message.transcript, style = PearlType.message, color = colors.onYou)
        when {
            delivery == null -> Status("Sent")
            delivery.status == RenditionStatus.TRANSLATING -> {
                StepsBar(done = 1)
                Status("Translating into $listenerLanguage", strong = true)
            }
            delivery.status == RenditionStatus.SPEAKING -> Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                BreathingRing(colors.onYou)
                Status("Preparing your voice in $listenerLanguage", strong = true)
            }
            delivery.status == RenditionStatus.FAILED -> {
                HorizontalDivider(color = colors.onYou.copy(alpha = 0.15f))
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Icon(PearlIcons.Alert, contentDescription = null, tint = colors.onYou, modifier = Modifier.size(18.dp))
                    Text("Couldn't translate this. Your words are kept.", style = PearlType.small, color = colors.onYou, modifier = Modifier.weight(1f))
                    ActionText("Retry", colors.onYou, onRetry)
                }
            }
            delivery.voice == RenditionVoice.PHONE -> Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                Icon(PearlIcons.Phone, contentDescription = null, tint = colors.onYou.copy(alpha = 0.7f), modifier = Modifier.size(14.dp))
                Text("Delivered in the phone's voice · ", style = PearlType.meta, color = colors.onYou.copy(alpha = 0.7f))
                ActionText("Record yours", colors.onYou.copy(alpha = 0.7f), onRecordVoice, strong = false)
            }
            else -> Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                Icon(PearlIcons.Check, contentDescription = null, tint = colors.onYou.copy(alpha = 0.7f), modifier = Modifier.size(14.dp))
                Text(
                    (if (delivery.voice == RenditionVoice.ORIGINAL) "Delivered" else "Delivered in your voice") + " · ${timeOfDay(message.createdAt)}",
                    style = PearlType.meta,
                    color = colors.onYou.copy(alpha = 0.7f),
                )
            }
        }
    }
}

/** The user's note before the server has it: being written down, sending, or not sent. */
@Composable
fun PendingNoteCard(note: PendingNote, onRetry: () -> Unit, modifier: Modifier = Modifier) {
    val colors = Pearl.colors
    MyCard(modifier) {
        when (note.stage) {
            PendingStage.WRITING -> {
                // Words arrive one by one; the last few are still being decided, so they are fainter.
                val words = note.text.split(" ").filter { it.isNotEmpty() }
                val settled = (words.size - 2).coerceAtLeast(0)
                Text(
                    buildAnnotatedString {
                        append(words.take(settled).joinToString(" "))
                        if (settled > 0 && words.size > settled) append(" ")
                        withStyle(SpanStyle(color = colors.onYou.copy(alpha = 0.44f))) { append(words.drop(settled).joinToString(" ")) }
                        append("▍")
                    },
                    style = PearlType.message,
                    color = colors.onYou,
                )
                Status("Writing down what you said…", strong = true)
            }
            PendingStage.SENDING -> {
                Text(note.text, style = PearlType.message, color = colors.onYou)
                StepsBar(done = 0)
                Status("Sending", strong = true)
            }
            PendingStage.FAILED -> {
                Text(note.text, style = PearlType.message, color = colors.onYou)
                HorizontalDivider(color = colors.onYou.copy(alpha = 0.15f))
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Icon(PearlIcons.Alert, contentDescription = null, tint = colors.onYou, modifier = Modifier.size(18.dp))
                    Text("Couldn't send this. Your words are kept.", style = PearlType.small, color = colors.onYou, modifier = Modifier.weight(1f))
                    ActionText("Retry", colors.onYou, onRetry)
                }
            }
        }
    }
}

/** The other person's note, as the user hears it: in their language, in the sender's voice. */
@Composable
fun TheirNoteCard(
    message: Message,
    myLanguage: String?,
    playing: Boolean,
    playingMs: Long?,
    fresh: Boolean,
    played: Boolean,
    showOriginal: Boolean,
    onPlay: () -> Unit,
    onToggleOriginal: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val colors = Pearl.colors
    val rendition: Rendition? = message.renditionFor(myLanguage)
    val ready = rendition?.status == RenditionStatus.READY
    val phoneVoice = ready && rendition?.voice == RenditionVoice.PHONE
    val text = rendition?.text?.takeIf { it.isNotEmpty() && ready }
    val originalLanguage = Language(message.lang).ownName()

    Column(
        modifier
            .widthIn(max = 290.dp)
            .clip(CardShape)
            .background(colors.bgRaised)
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        if (ready) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                PlayRow(
                    playing = playing,
                    durationMs = if (playing) playingMs ?: message.durationMs else message.durationMs,
                    dots = dotsFor(message.id),
                    fill = if (phoneVoice) Color.Transparent else colors.them,
                    icon = if (phoneVoice) colors.them else colors.onThem,
                    dotColor = if (phoneVoice) colors.fg.copy(alpha = 0.4f) else colors.them,
                    outline = phoneVoice,
                    onPlay = onPlay,
                    label = "Play",
                    dim = fresh && !playing,
                    modifier = Modifier.weight(1f),
                    showDuration = !fresh,
                )
                if (fresh && !playing) {
                    Box(Modifier.padding(start = 10.dp).size(10.dp).clip(CircleShape).background(colors.them))
                }
            }
        }
        when {
            text != null -> SpokenText(text, playing = playing, playingMs = playingMs)
            rendition?.status == RenditionStatus.FAILED || rendition == null -> Text(message.transcript, style = PearlType.message, color = colors.fg)
            rendition.status == RenditionStatus.SPEAKING -> Text("Preparing their voice…", style = PearlType.message, color = colors.fg2)
            else -> Text("Translating from $originalLanguage…", style = PearlType.message, color = colors.fg2)
        }
        if (showOriginal && text != null) {
            HorizontalDivider(color = colors.fg.copy(alpha = 0.08f))
            Text("Original · $originalLanguage", style = PearlType.label, color = colors.them)
            Text(message.transcript, style = PearlType.message.copy(fontSize = 15.sp), color = colors.fg.copy(alpha = 0.72f))
        }
        // The meta line: what state the note is in, and the original a tap away.
        when {
            rendition?.status == RenditionStatus.FAILED -> Meta("Couldn't be translated · Original · $originalLanguage")
            !ready -> Unit
            phoneVoice -> Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                Icon(PearlIcons.Phone, contentDescription = null, tint = colors.fg2, modifier = Modifier.size(14.dp))
                Meta("Spoken by your phone's voice")
            }
            playing -> Text("Playing in their voice", style = PearlType.meta, color = colors.them)
            else -> Row(verticalAlignment = Alignment.CenterVertically) {
                val state = when {
                    fresh -> "New · ${clock(message.durationMs)} · in their voice · "
                    played -> "Played · "
                    else -> "In their voice · "
                }
                Meta(state)
                if (rendition?.voice != RenditionVoice.ORIGINAL) {
                    ActionText(if (showOriginal) "Hide original" else "Show original", colors.them, onToggleOriginal, strong = false, small = true)
                }
            }
        }
    }
}

/** The translation, lighting up word by word as it plays. */
@Composable
private fun SpokenText(text: String, playing: Boolean, playingMs: Long?) {
    val colors = Pearl.colors
    val progress = remember(text) { Animatable(1f) }
    LaunchedEffect(playing, playingMs) {
        if (playing && playingMs != null) {
            progress.snapTo(0f)
            progress.animateTo(1f, tween(playingMs.toInt().coerceAtLeast(1), easing = LinearEasing))
        } else if (!playing) {
            progress.snapTo(1f)
        }
    }
    if (!playing) {
        Text(text, style = PearlType.message, color = colors.fg)
        return
    }
    val words = text.split(" ")
    val lit = (words.size * progress.value).toInt().coerceAtLeast(1)
    Text(
        buildAnnotatedString {
            append(words.take(lit).joinToString(" "))
            withStyle(SpanStyle(color = colors.fg.copy(alpha = 0.45f))) { append(" " + words.drop(lit).joinToString(" ")) }
        },
        style = PearlType.message,
        color = colors.fg,
    )
}

@Composable
private fun MyCard(modifier: Modifier, content: @Composable ColumnScope.() -> Unit) {
    Column(
        modifier
            .widthIn(max = 270.dp)
            .clip(CardShape)
            .background(Pearl.colors.you)
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        content = content,
    )
}

@Composable
private fun PlayRow(
    playing: Boolean,
    durationMs: Long,
    dots: List<Float>,
    fill: Color,
    icon: Color,
    dotColor: Color,
    onPlay: () -> Unit,
    label: String,
    modifier: Modifier = Modifier,
    outline: Boolean = false,
    dim: Boolean = false,
    showDuration: Boolean = true,
) {
    val colors = Pearl.colors
    val progress = remember { Animatable(1f) }
    LaunchedEffect(playing, durationMs) {
        if (playing) {
            progress.snapTo(0f)
            progress.animateTo(1f, tween(durationMs.toInt().coerceAtLeast(1), easing = LinearEasing))
        } else {
            progress.snapTo(1f)
        }
    }
    Row(modifier, verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        Box(
            Modifier
                .size(44.dp)
                .clip(CircleShape)
                .background(fill)
                .then(if (outline) Modifier.border(1.5.dp, colors.them, CircleShape) else Modifier)
                .clickable(role = Role.Button, onClickLabel = if (playing) "Stop" else label, onClick = onPlay),
            contentAlignment = Alignment.Center,
        ) {
            Icon(if (playing) PearlIcons.Pause else PearlIcons.Play, contentDescription = if (playing) "Stop" else label, tint = icon, modifier = Modifier.size(15.dp))
        }
        DotWaveform(
            levels = dots,
            color = dotColor.copy(alpha = if (dim) dotColor.alpha * 0.45f else dotColor.alpha),
            played = if (playing) progress.value else 1f,
            modifier = Modifier.weight(1f),
        )
        if (showDuration) Text(clock(durationMs), style = PearlType.meta, color = if (fill == colors.onYou) colors.onYou else colors.fg2)
    }
}

/** The design's progress bar of a note on its way: sent, translating, voice, delivered. */
@Composable
private fun StepsBar(done: Int) {
    val colors = Pearl.colors
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        repeat(4) { index ->
            Box(
                Modifier
                    .weight(1f)
                    .height(3.dp)
                    .clip(CircleShape)
                    .background(if (index <= done) colors.onYou else colors.onYou.copy(alpha = 0.18f)),
            )
        }
    }
}

/** A dot in a ring, breathing slowly while the voice is made, so the card never looks frozen. */
@Composable
private fun BreathingRing(color: Color) {
    val breath by rememberInfiniteTransition(label = "breath").animateFloat(
        initialValue = 0.85f,
        targetValue = 1.1f,
        animationSpec = infiniteRepeatable(tween(1600), RepeatMode.Reverse),
        label = "scale",
    )
    Box(
        Modifier.size(28.dp).clip(CircleShape).border(1.5.dp, color.copy(alpha = 0.5f), CircleShape),
        contentAlignment = Alignment.Center,
    ) {
        Box(Modifier.size(14.dp).scale(breath).clip(CircleShape).background(color))
    }
}

@Composable
private fun Status(text: String, strong: Boolean = false) {
    Text(text, style = if (strong) PearlType.smallStrong else PearlType.small, color = Pearl.colors.onYou)
}

@Composable
private fun Meta(text: String) {
    Text(text, style = PearlType.meta, color = Pearl.colors.fg2)
}

@Composable
private fun ActionText(text: String, color: Color, onClick: () -> Unit, strong: Boolean = true, small: Boolean = false) {
    Text(
        text,
        style = (if (small) PearlType.meta else if (strong) PearlType.smallStrong else PearlType.small).copy(textDecoration = TextDecoration.Underline),
        color = color,
        modifier = Modifier.clickable(role = Role.Button, onClick = onClick).padding(vertical = 4.dp),
    )
}
