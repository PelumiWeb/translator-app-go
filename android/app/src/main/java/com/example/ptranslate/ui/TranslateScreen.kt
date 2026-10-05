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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Switch
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
import com.example.ptranslate.ui.theme.PtranslateTheme
import java.util.Locale

@Composable
fun TranslateScreen(
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
        onRouteChange = viewModel::onRouteChanged,
        onSourceSelect = viewModel::onSourceSelected,
        onTargetSelect = viewModel::onTargetSelected,
        modifier = modifier,
    )
}

/** Stateless, so it can be previewed without a ViewModel. */
@Composable
private fun TranslateContent(
    state: TranslateUiState,
    onRecordClick: () -> Unit,
    onRouteChange: (onDevice: Boolean) -> Unit,
    onSourceSelect: (Language) -> Unit,
    onTargetSelect: (Language) -> Unit,
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
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Switch(checked = state.onDevice, onCheckedChange = onRouteChange, enabled = idle)
            Text(if (state.onDevice) "Transcribe on this device" else "Transcribe on the server")
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
        state.error?.let {
            Text(text = it, color = MaterialTheme.colorScheme.error)
        }
    }
}

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
            ),
            onRecordClick = {},
            onRouteChange = {},
            onSourceSelect = {},
            onTargetSelect = {},
        )
    }
}
