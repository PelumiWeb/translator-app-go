package com.example.ptranslate.ui.rooms

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.rooms.Message
import com.example.ptranslate.core.rooms.Rendition
import com.example.ptranslate.core.rooms.RenditionStatus
import com.example.ptranslate.core.rooms.RenditionVoice
import com.example.ptranslate.ui.components.PearlIcons
import com.example.ptranslate.ui.components.localName
import com.example.ptranslate.ui.components.ownName
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType

// Text messages, in the voice notes' colours: the user's in warm white, the
// other person's in raised petrol with mint. The words come first; the voice
// is a tap away, made the first time anyone asks for it.

/** Where hearing a text message stands, for the listen control. */
private enum class Listening { IDLE, PREPARING, PLAYING }

private fun listening(rendition: Rendition?, playing: Boolean, waiting: Boolean) = when {
    playing -> Listening.PLAYING
    waiting || rendition?.status == RenditionStatus.SPEAKING -> Listening.PREPARING
    else -> Listening.IDLE
}

/** The user's text message, from the moment the server has it. */
@Composable
fun MyTextCard(
    message: Message,
    playing: Boolean,
    waiting: Boolean,
    onListen: () -> Unit,
    onRetry: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val colors = Pearl.colors
    val delivery = message.delivery
    val listenerLanguage = delivery?.lang?.let { Language(it).localName() } ?: ""
    MyCard(modifier) {
        Text(message.transcript, style = PearlType.message, color = colors.onYou)
        when {
            delivery == null -> Status("Sent")
            delivery.status == RenditionStatus.TRANSLATING -> Status("Translating into $listenerLanguage", strong = true)
            delivery.status == RenditionStatus.FAILED -> {
                HorizontalDivider(color = colors.onYou.copy(alpha = 0.15f))
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Icon(PearlIcons.Alert, contentDescription = null, tint = colors.onYou, modifier = Modifier.size(18.dp))
                    Text("Couldn't translate this. Your words are kept.", style = PearlType.small, color = colors.onYou, modifier = Modifier.weight(1f))
                    ActionText("Retry", colors.onYou, onRetry)
                }
            }
            else -> Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                val dim = colors.onYou.copy(alpha = 0.7f)
                Icon(PearlIcons.Check, contentDescription = null, tint = dim, modifier = Modifier.size(14.dp))
                Text("Delivered · ${timeOfDay(message.createdAt)} · ", style = PearlType.meta, color = dim)
                when (listening(delivery, playing, waiting)) {
                    Listening.PLAYING -> ActionText("Stop", dim, onListen, strong = false, small = true)
                    Listening.PREPARING -> Text("Preparing your voice…", style = PearlType.meta, color = dim)
                    Listening.IDLE -> ActionText("Hear it in $listenerLanguage", dim, onListen, strong = false, small = true)
                }
            }
        }
    }
}

/** The other person's text message, as the user reads it: in their language, with the sender's voice a tap away. */
@Composable
fun TheirTextCard(
    message: Message,
    myLanguage: String?,
    playing: Boolean,
    playingMs: Long?,
    waiting: Boolean,
    showOriginal: Boolean,
    onListen: () -> Unit,
    onToggleOriginal: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val colors = Pearl.colors
    val rendition = message.renditionFor(myLanguage)
    val words = message.wordsFor(myLanguage)
    val originalLanguage = Language(message.lang).ownName()
    val translated = rendition != null && rendition.lang != message.lang

    Column(
        modifier
            .widthIn(max = 290.dp)
            .clip(RoundedCornerShape(12.dp))
            .background(colors.bgRaised)
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        when {
            words != null -> SpokenText(words, playing = playing, playingMs = playingMs)
            rendition == null || rendition.status == RenditionStatus.FAILED -> Text(message.transcript, style = PearlType.message, color = colors.fg)
            else -> Text("Translating from $originalLanguage…", style = PearlType.message, color = colors.fg2)
        }
        if (showOriginal && words != null && translated) {
            HorizontalDivider(color = colors.fg.copy(alpha = 0.08f))
            Text("Original · $originalLanguage", style = PearlType.label, color = colors.them)
            Text(message.transcript, style = PearlType.message.copy(fontSize = 15.sp), color = colors.fg.copy(alpha = 0.72f))
        }
        when {
            rendition?.status == RenditionStatus.FAILED -> Meta("Couldn't be translated · Original · $originalLanguage")
            words == null -> Unit
            else -> Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                val state = listening(rendition, playing, waiting)
                ListenButton(state, onListen)
                val phoneVoice = rendition?.voice == RenditionVoice.PHONE
                Text(
                    when (state) {
                        Listening.PLAYING -> if (phoneVoice) "Spoken by your phone's voice" else "Playing in their voice"
                        Listening.PREPARING -> "Preparing their voice…"
                        Listening.IDLE -> if (phoneVoice) "Listen · your phone's voice" else "Listen in their voice"
                    },
                    style = PearlType.meta,
                    color = if (state == Listening.PLAYING) colors.them else colors.fg2,
                    modifier = Modifier.weight(1f, fill = false),
                )
                if (translated) {
                    ActionText(if (showOriginal) "Hide original" else "Show original", colors.them, onToggleOriginal, strong = false, small = true)
                }
            }
        }
    }
}

/** The mint outline button that plays a text message in its sender's voice, breathing while the voice is made. */
@Composable
private fun ListenButton(state: Listening, onClick: () -> Unit) {
    val colors = Pearl.colors
    val label = when (state) {
        Listening.PLAYING -> "Stop"
        Listening.PREPARING -> "Preparing their voice"
        Listening.IDLE -> "Listen"
    }
    Box(
        Modifier
            .size(36.dp)
            .clip(CircleShape)
            .background(if (state == Listening.PLAYING) colors.them else Color.Transparent)
            .border(1.5.dp, colors.them, CircleShape)
            .clickable(role = Role.Button, onClickLabel = label, onClick = onClick)
            .semantics { contentDescription = label },
        contentAlignment = Alignment.Center,
    ) {
        when (state) {
            Listening.PLAYING -> Icon(PearlIcons.Pause, contentDescription = null, tint = colors.onThem, modifier = Modifier.size(13.dp))
            Listening.PREPARING -> BreathingRing(colors.them)
            Listening.IDLE -> Icon(PearlIcons.Speaker, contentDescription = null, tint = colors.them, modifier = Modifier.size(17.dp))
        }
    }
}
