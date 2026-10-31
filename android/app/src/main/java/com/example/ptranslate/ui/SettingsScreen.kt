package com.example.ptranslate.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.RadioButton
import androidx.compose.material3.RadioButtonDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.example.ptranslate.BuildConfig
import com.example.ptranslate.core.model.ModelState
import com.example.ptranslate.core.stt.TranscriptionRoute
import com.example.ptranslate.ui.components.LinkButton
import com.example.ptranslate.ui.components.PearlIcons
import com.example.ptranslate.ui.components.PrimaryButton
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType
import java.util.Locale

/**
 * Settings: the user's voice, and under "Advanced" the controls that left the
 * interpreter for being about how it works rather than what it does.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(
    viewModel: TranslateViewModel,
    savedVoiceMs: Long?,
    onVoiceClick: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val colors = Pearl.colors

    Column(modifier.fillMaxSize()) {
        TopAppBar(
            title = { Text("Settings", style = PearlType.title) },
            navigationIcon = {
                IconButton(onClick = onBack) {
                    Icon(PearlIcons.Back, contentDescription = "Back", tint = colors.fg)
                }
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = colors.bg, titleContentColor = colors.fg),
        )
        Column(
            Modifier.verticalScroll(rememberScrollState()).padding(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            SectionTitle("Your voice")
            Card {
                Text(
                    if (savedVoiceMs != null) {
                        "Recorded, ${savedVoiceMs / 1000} seconds. Translations are spoken in your voice."
                    } else {
                        "Not recorded. Translations are spoken in the phone's voice."
                    },
                    style = PearlType.message,
                    color = colors.fg,
                )
                PrimaryButton(if (savedVoiceMs != null) "Record again or delete" else "Record your voice", onClick = onVoiceClick)
            }

            SectionTitle("Advanced")
            Card {
                Text("Turn speech into text", style = PearlType.label, color = colors.fg2)
                TranscriptionRoute.entries.forEach { route ->
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .clickable(role = Role.RadioButton) { viewModel.onRouteSelected(route) },
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        RadioButton(
                            selected = state.route == route,
                            onClick = { viewModel.onRouteSelected(route) },
                            colors = RadioButtonDefaults.colors(selectedColor = colors.them, unselectedColor = colors.fg3),
                        )
                        Text(route.label(), style = PearlType.message, color = colors.fg)
                    }
                }
                Text(state.route.description(), style = PearlType.small, color = colors.fg2)
            }

            Card {
                Text("Speech model on this phone", style = PearlType.label, color = colors.fg2)
                ModelStatus(state.model, onDownloadClick = viewModel::onDownloadModelClicked)
            }

            if (state.model is ModelState.Ready) {
                Card {
                    Text("This phone's speed", style = PearlType.label, color = colors.fg2)
                    val speed = state.deviceSpeed
                    Text(
                        when {
                            state.measuringSpeed -> "Measuring…"
                            speed == null -> "Not measured yet"
                            state.deviceTooSlow -> String.format(Locale.US, "%.2fx real time. Too slow, so the server is used", speed)
                            else -> String.format(Locale.US, "%.2fx real time", speed)
                        },
                        style = PearlType.message,
                        color = colors.fg,
                    )
                    if (!state.measuringSpeed) {
                        LinkButton("Measure again", onClick = viewModel::onMeasureSpeedClicked)
                    }
                }
            }

            if (state.details != null || state.routing != null) {
                Card {
                    Text("Last result", style = PearlType.label, color = colors.fg2)
                    state.details?.let { Text(it, style = PearlType.small, color = colors.fg) }
                    state.routing?.let { Text(it, style = PearlType.small, color = colors.fg) }
                }
            }

            SectionTitle("About")
            Text(
                "Version ${BuildConfig.VERSION_NAME}. Set in IBM Plex Sans, under the SIL Open Font License.",
                style = PearlType.small,
                color = colors.fg2,
                modifier = Modifier.padding(bottom = 24.dp),
            )
        }
    }
}

@Composable
private fun SectionTitle(text: String) {
    Text(text, style = PearlType.label, color = Pearl.colors.them, modifier = Modifier.padding(top = 10.dp, start = 4.dp))
}

@Composable
private fun Card(content: @Composable ColumnScope.() -> Unit) {
    Column(
        Modifier
            .fillMaxWidth()
            .background(Pearl.colors.bgRaised, RoundedCornerShape(12.dp))
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        content = content,
    )
}

/** Where the on-device speech model stands, and the download. */
@Composable
private fun ModelStatus(model: ModelState, onDownloadClick: () -> Unit) {
    val colors = Pearl.colors
    when (model) {
        ModelState.Missing -> {
            Text("Not on this phone yet. It is 57 MB; Wi-Fi is best.", style = PearlType.message, color = colors.fg)
            PrimaryButton("Download speech model", onClick = onDownloadClick)
        }
        is ModelState.Downloading -> {
            Text("Downloading: ${model.bytes.toMb()} of ${model.total.toMb()} MB", style = PearlType.message, color = colors.fg)
            LinearProgressIndicator(
                progress = { if (model.total > 0) model.bytes.toFloat() / model.total else 0f },
                modifier = Modifier.fillMaxWidth(),
                color = colors.them,
                trackColor = colors.bg,
            )
        }
        ModelState.Verifying -> {
            Text("Checking the download", style = PearlType.message, color = colors.fg)
            LinearProgressIndicator(modifier = Modifier.fillMaxWidth(), color = colors.them, trackColor = colors.bg)
        }
        is ModelState.Failed -> {
            Text(model.reason, style = PearlType.message, color = colors.fg)
            // The partial download is kept, so this continues, not restarts.
            PrimaryButton("Try again", onClick = onDownloadClick)
        }
        is ModelState.Ready -> Text("Ready. Speech can be turned into text on this phone, offline.", style = PearlType.message, color = colors.fg)
    }
}

private fun Long.toMb(): Long = this / (1024 * 1024)

private fun TranscriptionRoute.label(): String = when (this) {
    TranscriptionRoute.AUTO -> "Automatic"
    TranscriptionRoute.ON_DEVICE -> "This phone only"
    TranscriptionRoute.CLOUD -> "The server"
}

// The automatic route can upload audio without asking each time, so it says so.
private fun TranscriptionRoute.description(): String = when (this) {
    TranscriptionRoute.AUTO -> "On this phone when it can. Otherwise your recording is sent to the server."
    TranscriptionRoute.ON_DEVICE -> "On this phone only. Nothing is uploaded."
    TranscriptionRoute.CLOUD -> "Every recording is sent to the server."
}
