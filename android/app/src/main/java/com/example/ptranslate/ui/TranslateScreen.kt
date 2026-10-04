package com.example.ptranslate.ui

import android.Manifest
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.example.ptranslate.ui.theme.PtranslateTheme

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
        modifier = modifier,
    )
}

/** Stateless, so it can be previewed without a ViewModel. */
@Composable
private fun TranslateContent(
    state: TranslateUiState,
    onRecordClick: () -> Unit,
    onRouteChange: (onDevice: Boolean) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Switch(
                checked = state.onDevice,
                onCheckedChange = onRouteChange,
                enabled = state.phase == Phase.IDLE,
            )
            Text(if (state.onDevice) "Transcribe on this device" else "Transcribe on the server")
        }
        Button(onClick = onRecordClick, enabled = state.phase != Phase.WORKING) {
            Text(if (state.phase == Phase.RECORDING) "Stop" else "Record")
        }
        Text(text = state.status, style = MaterialTheme.typography.labelLarge)
        Text(text = state.text, style = MaterialTheme.typography.headlineSmall)
        state.details?.let {
            Text(text = it, style = MaterialTheme.typography.bodySmall)
        }
        state.error?.let {
            Text(text = it, color = MaterialTheme.colorScheme.error)
        }
    }
}

@Preview(showBackground = true)
@Composable
private fun TranslateContentPreview() {
    PtranslateTheme {
        TranslateContent(
            state = TranslateUiState(
                status = "Done",
                text = "ask not what your country can do for you",
                details = "On device, 1.2 s for 4.0 s of audio (0.30x real time), confidence 0.87",
            ),
            onRecordClick = {},
            onRouteChange = {},
        )
    }
}
