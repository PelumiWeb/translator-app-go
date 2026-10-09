package com.example.ptranslate.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.example.ptranslate.PtranslateApp
import com.example.ptranslate.core.voice.VoiceSampleState

/**
 * Chooses between the app's two screens. With only two, a boolean does the
 * job a navigation library would.
 */
@Composable
fun AppRoot(modifier: Modifier = Modifier) {
    val container = (LocalContext.current.applicationContext as PtranslateApp).container
    val voice by container.voice.state.collectAsStateWithLifecycle()

    // On first launch the user is taken to record their voice. They are not
    // taken there again once they have recorded one, or said "not now".
    // rememberSaveable keeps the choice of screen across a rotation.
    var settingUpVoice by rememberSaveable {
        mutableStateOf(voice is VoiceSampleState.None && !container.voiceSetup.skipped)
    }

    if (settingUpVoice) {
        VoiceSetupScreen(onDone = { settingUpVoice = false }, modifier = modifier)
    } else {
        TranslateScreen(
            savedVoiceMs = (voice as? VoiceSampleState.Present)?.durationMs,
            onVoiceClick = { settingUpVoice = true },
            modifier = modifier,
        )
    }
}
