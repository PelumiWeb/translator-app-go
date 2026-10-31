package com.example.ptranslate.ui

import androidx.activity.compose.BackHandler
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.example.ptranslate.PtranslateApp
import com.example.ptranslate.core.voice.VoiceSampleState

private enum class Screen { INTERPRETER, SETTINGS, VOICE }

/**
 * Chooses between the app's three screens. With so few, a state value does the
 * job a navigation library would; the back button is handled by hand.
 */
@Composable
fun AppRoot(modifier: Modifier = Modifier) {
    val container = (LocalContext.current.applicationContext as PtranslateApp).container
    val voice by container.voice.state.collectAsStateWithLifecycle()
    // Shared by the interpreter and settings, which shows its advanced state.
    val translate: TranslateViewModel = viewModel(factory = TranslateViewModel.Factory)
    val savedVoiceMs = (voice as? VoiceSampleState.Present)?.durationMs

    // On first launch the user is taken to record their voice. They are not
    // taken there again once they have recorded one, or said "not now".
    // rememberSaveable keeps the screen across a rotation.
    var screen by rememberSaveable {
        mutableStateOf(if (voice is VoiceSampleState.None && !container.voiceSetup.skipped) Screen.VOICE else Screen.INTERPRETER)
    }
    var voiceOpenedFromSettings by rememberSaveable { mutableStateOf(false) }

    when (screen) {
        Screen.INTERPRETER -> TranslateScreen(
            viewModel = translate,
            savedVoiceMs = savedVoiceMs,
            onSettingsClick = { screen = Screen.SETTINGS },
            modifier = modifier,
        )
        Screen.SETTINGS -> {
            BackHandler { screen = Screen.INTERPRETER }
            SettingsScreen(
                viewModel = translate,
                savedVoiceMs = savedVoiceMs,
                onVoiceClick = {
                    voiceOpenedFromSettings = true
                    screen = Screen.VOICE
                },
                onBack = { screen = Screen.INTERPRETER },
                modifier = modifier,
            )
        }
        Screen.VOICE -> {
            // On first launch, back leaves the app as usual; from settings, it goes back there.
            BackHandler(enabled = voiceOpenedFromSettings) { screen = Screen.SETTINGS }
            VoiceSetupScreen(
                onDone = {
                    screen = if (voiceOpenedFromSettings) Screen.SETTINGS else Screen.INTERPRETER
                    voiceOpenedFromSettings = false
                },
                modifier = modifier,
            )
        }
    }
}
