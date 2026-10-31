package com.example.ptranslate.ui.profile

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.example.ptranslate.core.model.ModelState
import com.example.ptranslate.core.stt.TranscriptionRoute
import com.example.ptranslate.ui.TranslateUiState
import com.example.ptranslate.ui.components.Avatar
import com.example.ptranslate.ui.components.GroupRow
import com.example.ptranslate.ui.components.GroupedSection
import com.example.ptranslate.ui.components.Notice
import com.example.ptranslate.ui.components.PearlIcons
import com.example.ptranslate.ui.components.localName
import com.example.ptranslate.ui.rooms.clock
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

/** Where the profile leads. */
enum class ProfileDestination { NAME, HANDLE, LANGUAGE, PRIVACY, ADVANCED, RECORD_VOICE }

private enum class Confirm { DELETE_VOICE, DELETE_ACCOUNT }

/** Who the user is, their voice, privacy, the advanced controls, and the account. */
@Composable
fun ProfileScreen(
    viewModel: ProfileViewModel,
    translate: TranslateUiState,
    onOpen: (ProfileDestination) -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val colors = Pearl.colors
    val profile = state.profile ?: return
    var confirm by rememberSaveable { mutableStateOf<Confirm?>(null) }
    val hasVoice = state.keptVoice != null || state.voiceMs != null

    Column(
        modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = 16.dp, vertical = 24.dp),
        verticalArrangement = Arrangement.spacedBy(24.dp),
    ) {
        Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
            Avatar(profile.displayName, size = 76.dp, fill = colors.you, content = colors.onYou, style = PearlType.screenTitle)
            Text(profile.displayName, style = PearlType.title, color = colors.fg, modifier = Modifier.padding(top = 14.dp))
            Text("@${profile.handle} · speaks ${profile.lang?.localName().orEmpty()}", style = PearlType.message, color = colors.fg2, modifier = Modifier.padding(top = 4.dp))
        }
        state.problem?.let { Notice(it) }

        GroupedSection(
            "Your voice",
            footer = "Used only to speak your own words. Without it, your messages use the phone's voice.",
        ) {
            if (hasVoice) {
                GroupRow(
                    title = if (state.playingVoice) "Playing your voice" else "Play your voice",
                    subtitle = listOfNotNull(state.voiceMs?.let(::clock), state.keptVoice?.let { "recorded ${recordedOn(it.consentedAt)}" }).joinToString(" · ").ifEmpty { null },
                    leading = {
                        Box(Modifier.size(44.dp).clip(CircleShape).background(colors.you), contentAlignment = Alignment.Center) {
                            Icon(if (state.playingVoice) PearlIcons.Pause else PearlIcons.Play, contentDescription = null, tint = colors.onYou, modifier = Modifier.size(15.dp))
                        }
                    },
                    onClick = viewModel::onPlayVoiceClicked,
                )
                GroupRow("Record again", onClick = { onOpen(ProfileDestination.RECORD_VOICE) })
                GroupRow("Delete my voice", titleColor = colors.destructive, divider = false, onClick = { confirm = Confirm.DELETE_VOICE })
            } else {
                GroupRow("Record your voice", chevron = true, divider = false, onClick = { onOpen(ProfileDestination.RECORD_VOICE) })
            }
        }

        GroupedSection("You") {
            GroupRow("Name", value = profile.displayName, chevron = true, onClick = { onOpen(ProfileDestination.NAME) })
            GroupRow("ID", value = "@${profile.handle}", chevron = true, onClick = { onOpen(ProfileDestination.HANDLE) })
            GroupRow("Language", value = profile.lang?.localName(), chevron = true, divider = false, onClick = { onOpen(ProfileDestination.LANGUAGE) })
        }

        GroupedSection("Privacy") {
            GroupRow("What we keep, and why", chevron = true, divider = false, onClick = { onOpen(ProfileDestination.PRIVACY) })
        }

        GroupedSection("Advanced") {
            GroupRow(
                "Speech to text",
                subtitle = "Where your words are written down",
                value = translate.route.shortLabel(),
                chevron = true,
                onClick = { onOpen(ProfileDestination.ADVANCED) },
            )
            GroupRow(
                "Speech model",
                subtitle = translate.model.summary(),
                chevron = true,
                onClick = { onOpen(ProfileDestination.ADVANCED) },
            )
            GroupRow(
                "This phone's speed",
                value = when {
                    translate.deviceSpeed == null -> "Not measured"
                    translate.deviceTooSlow -> "Slow"
                    else -> "Fast"
                },
                divider = false,
            )
        }

        GroupedSection("Account") {
            GroupRow("Sign out", onClick = viewModel::onSignOutClicked)
            GroupRow("Delete account", titleColor = colors.destructive, divider = false, onClick = { confirm = Confirm.DELETE_ACCOUNT })
        }
    }

    when (confirm) {
        Confirm.DELETE_VOICE -> Confirmation(
            title = "Delete your voice?",
            text = "Your recording is deleted from our server and this phone. The people you talk to will hear your messages in the phone's voice. You can record again at any time.",
            action = "Delete my voice",
            onConfirm = {
                confirm = null
                viewModel.onDeleteVoiceConfirmed()
            },
            onDismiss = { confirm = null },
        )
        Confirm.DELETE_ACCOUNT -> Confirmation(
            title = "Delete your account?",
            text = "Everything goes: your profile, your voice, and everything you said in your rooms. This can't be undone.",
            action = "Delete account",
            onConfirm = {
                confirm = null
                viewModel.onDeleteAccountConfirmed()
            },
            onDismiss = { confirm = null },
        )
        null -> Unit
    }
}

/** A native confirmation that says plainly what the action means. */
@Composable
private fun Confirmation(title: String, text: String, action: String, onConfirm: () -> Unit, onDismiss: () -> Unit) {
    val colors = Pearl.colors
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title, style = PearlType.title) },
        text = { Text(text, style = PearlType.message) },
        confirmButton = { TextButton(onClick = onConfirm) { Text(action, color = colors.destructive, style = PearlType.chip) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel", color = colors.fg, style = PearlType.chip) } },
        containerColor = colors.bgRaised,
        titleContentColor = colors.fg,
        textContentColor = colors.fg2,
    )
}

private fun recordedOn(consentedAt: String): String =
    runCatching { Instant.parse(consentedAt).atZone(ZoneId.systemDefault()).format(DateTimeFormatter.ofPattern("d MMM")) }.getOrDefault("")

private fun TranscriptionRoute.shortLabel(): String = when (this) {
    TranscriptionRoute.AUTO -> "Automatic"
    TranscriptionRoute.ON_DEVICE -> "This device"
    TranscriptionRoute.CLOUD -> "Server"
}

private fun ModelState.summary(): String = when (this) {
    is ModelState.Ready -> "Ready · ${file.length() / (1024 * 1024)} MB on this phone"
    is ModelState.Downloading -> "Downloading"
    else -> "Not on this phone"
}
