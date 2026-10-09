package com.example.ptranslate.ui

import android.Manifest
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FilterChip
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.TextButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.model.ModelState
import com.example.ptranslate.core.stt.TranscriptionRoute
import com.example.ptranslate.ui.theme.PtranslateTheme
import java.util.Locale

/**
 * @param savedVoiceMs length of the user's saved voice recording, or null if
 *   they have not made one.
 * @param onVoiceClick opens the screen where it is recorded or changed.
 */
@Composable
fun TranslateScreen(
    savedVoiceMs: Long?,
    onVoiceClick: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: TranslateViewModel = viewModel(factory = TranslateViewModel.Factory),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current

    val askForMicrophone = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        if (granted) viewModel.onRecordClicked() else viewModel.onPermissionDenied()
    }

    TranslateContent(
        state = state,
        onRecordClick = {
            val granted = ContextCompat.checkSelfPermission(context, Manifest.permission.RECORD_AUDIO) ==
                PackageManager.PERMISSION_GRANTED
            if (granted) viewModel.onRecordClicked() else askForMicrophone.launch(Manifest.permission.RECORD_AUDIO)
        },
        onRouteSelect = viewModel::onRouteSelected,
        onMeasureSpeedClick = viewModel::onMeasureSpeedClicked,
        onSourceSelect = viewModel::onSourceSelected,
        onTargetSelect = viewModel::onTargetSelected,
        onDownloadModelClick = viewModel::onDownloadModelClicked,
        savedVoiceMs = savedVoiceMs,
        onVoiceClick = onVoiceClick,
        modifier = modifier,
    )
}

/** Stateless, so it can be previewed without a ViewModel. */
@Composable
private fun TranslateContent(
    state: TranslateUiState,
    onRecordClick: () -> Unit,
    onRouteSelect: (TranscriptionRoute) -> Unit,
    onMeasureSpeedClick: () -> Unit,
    onSourceSelect: (Language) -> Unit,
    onTargetSelect: (Language) -> Unit,
    onDownloadModelClick: () -> Unit,
    savedVoiceMs: Long?,
    onVoiceClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val idle = state.phase == Phase.IDLE

    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            LanguagePicker("From", state.source, state.sourceLanguages, enabled = idle, onSelect = onSourceSelect)
            LanguagePicker("To", state.target, state.targetLanguages, enabled = idle, onSelect = onTargetSelect)
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(
                text = if (savedVoiceMs == null) {
                    "Your voice is not recorded. Translations use the phone's voice"
                } else {
                    "Your voice is recorded (${savedVoiceMs / 1000} s)"
                },
                style = MaterialTheme.typography.bodySmall,
                modifier = Modifier.weight(1f),
            )
            TextButton(onClick = onVoiceClick, enabled = idle) {
                Text(if (savedVoiceMs == null) "Record" else "Change")
            }
        }
        RoutePicker(state.route, enabled = idle, onSelect = onRouteSelect)

        // The model and its speed only matter when this device may be used.
        if (state.route != TranscriptionRoute.CLOUD) {
            ModelStatus(state.model, onDownloadClick = onDownloadModelClick)
            if (state.model is ModelState.Ready) {
                DeviceSpeedStatus(state, enabled = idle, onMeasureClick = onMeasureSpeedClick)
            }
        }
        Button(onClick = onRecordClick, enabled = state.phase != Phase.WORKING) {
            Text(if (state.phase == Phase.RECORDING) "Stop" else "Record")
        }
        Text(text = state.status, style = MaterialTheme.typography.labelLarge)

        if (state.text.isNotEmpty()) {
            Text(
                text = state.text,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        if (state.translation.isNotEmpty()) {
            Text(text = state.translation, style = MaterialTheme.typography.headlineSmall)
        }
        state.details?.let {
            Text(text = it, style = MaterialTheme.typography.bodySmall)
        }
        state.routing?.let {
            Text(text = it, style = MaterialTheme.typography.bodySmall)
        }
        state.error?.let {
            Text(text = it, color = MaterialTheme.colorScheme.error)
        }
    }
}

/** The three routes, with one line saying what the chosen one does. */
@Composable
private fun RoutePicker(selected: TranscriptionRoute, enabled: Boolean, onSelect: (TranscriptionRoute) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            TranscriptionRoute.entries.forEach { route ->
                FilterChip(
                    selected = route == selected,
                    onClick = { onSelect(route) },
                    enabled = enabled,
                    label = { Text(route.label()) },
                )
            }
        }
        Text(text = selected.description(), style = MaterialTheme.typography.bodySmall)
    }
}

private fun TranscriptionRoute.label(): String = when (this) {
    TranscriptionRoute.AUTO -> "Automatic"
    TranscriptionRoute.ON_DEVICE -> "This device"
    TranscriptionRoute.CLOUD -> "Server"
}

// The automatic route can upload audio without asking each time, so it says so.
private fun TranscriptionRoute.description(): String = when (this) {
    TranscriptionRoute.AUTO ->
        "Transcribes on this device when it can. Otherwise your recording is sent to the server."
    TranscriptionRoute.ON_DEVICE -> "Transcribes on this device only. Nothing is uploaded."
    TranscriptionRoute.CLOUD -> "Every recording is sent to the server."
}

/** How fast this device runs the model, and what that means for routing. */
@Composable
private fun DeviceSpeedStatus(state: TranslateUiState, enabled: Boolean, onMeasureClick: () -> Unit) {
    val speed = state.deviceSpeed
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(
            text = when {
                state.measuringSpeed -> "Measuring this device's speed"
                speed == null -> "This device's speed has not been measured"
                state.deviceTooSlow && state.route == TranscriptionRoute.AUTO ->
                    String.format(Locale.US, "Speed: %.2fx real time. Too slow, so the server is used", speed)
                else -> String.format(Locale.US, "Speed: %.2fx real time", speed)
            },
            style = MaterialTheme.typography.bodySmall,
            modifier = Modifier.weight(1f),
        )
        TextButton(onClick = onMeasureClick, enabled = enabled && !state.measuringSpeed) {
            Text("Measure again")
        }
    }
}

/** Says where the on-device speech model stands, and offers the download. */
@Composable
private fun ModelStatus(model: ModelState, onDownloadClick: () -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        when (model) {
            ModelState.Missing -> {
                Text("The speech model is not on this device yet.", style = MaterialTheme.typography.bodyMedium)
                OutlinedButton(onClick = onDownloadClick) { Text("Download speech model") }
            }

            is ModelState.Downloading -> {
                Text(
                    "Downloading speech model: ${model.bytes.toMb()} of ${model.total.toMb()} MB",
                    style = MaterialTheme.typography.bodyMedium,
                )
                LinearProgressIndicator(
                    progress = { if (model.total > 0) model.bytes.toFloat() / model.total else 0f },
                    modifier = Modifier.fillMaxWidth(),
                )
            }

            ModelState.Verifying -> {
                Text("Checking the download", style = MaterialTheme.typography.bodyMedium)
                LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
            }

            is ModelState.Failed -> {
                Text(model.reason, color = MaterialTheme.colorScheme.error)
                // The partial download is kept, so this continues, not restarts.
                OutlinedButton(onClick = onDownloadClick) { Text("Try again") }
            }

            is ModelState.Ready -> Text("Speech model ready", style = MaterialTheme.typography.bodySmall)
        }
    }
}

private fun Long.toMb(): Long = this / (1024 * 1024)

@Composable
private fun LanguagePicker(
    label: String,
    selected: Language,
    options: List<Language>,
    enabled: Boolean,
    onSelect: (Language) -> Unit,
) {
    var expanded by remember { mutableStateOf(false) }
    // Sorted by the name the user reads, which depends on the phone's language.
    val sorted = remember(options) { options.sortedBy { it.displayName() } }

    // The Box anchors the menu to the button that opens it.
    Box {
        OutlinedButton(onClick = { expanded = true }, enabled = enabled) {
            Text("$label: ${selected.displayName()}")
        }
        DropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }) {
            sorted.forEach { language ->
                DropdownMenuItem(
                    text = { Text(language.displayName()) },
                    onClick = {
                        expanded = false
                        onSelect(language)
                    },
                )
            }
        }
    }
}

/** "es" becomes "Spanish" on an English phone, "espagnol" on a French one. */
private fun Language.displayName(): String =
    Locale.forLanguageTag(tag).displayLanguage
        .replaceFirstChar { it.titlecase() }
        .ifBlank { tag }

@Preview(showBackground = true)
@Composable
private fun TranslateContentPreview() {
    PtranslateTheme {
        TranslateContent(
            state = TranslateUiState(
                status = "Done",
                text = "Good morning, how are you?",
                translation = "Buenos días, ¿cómo estás?",
                details = "On device, 1.2 s for 4.0 s of audio (0.30x real time), confidence 0.87",
                routing = "Sent to the server: this device was not confident in its own result",
                model = ModelState.Ready(java.io.File("model.bin")),
                deviceSpeed = 0.35f,
            ),
            onRecordClick = {},
            onRouteSelect = {},
            onMeasureSpeedClick = {},
            onSourceSelect = {},
            onTargetSelect = {},
            onDownloadModelClick = {},
            savedVoiceMs = 21_000,
            onVoiceClick = {},
        )
    }
}
