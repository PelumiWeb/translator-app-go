package com.example.ptranslate.core.voice

import android.content.SharedPreferences

class SharedPreferencesVoiceSetup(private val prefs: SharedPreferences) : VoiceSetupPreference {
    override var skipped: Boolean
        get() = prefs.getBoolean(KEY, false)
        set(value) = prefs.edit().putBoolean(KEY, value).apply()

    private companion object {
        const val KEY = "voice_setup_skipped"
    }
}
