package com.example.ptranslate

import android.content.Intent
import android.graphics.Color
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.SystemBarStyle
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import com.example.ptranslate.ui.AppRoot
import com.example.ptranslate.ui.LaunchCover
import com.example.ptranslate.ui.rooms.handleFromCode
import com.example.ptranslate.ui.theme.PtranslateTheme

class MainActivity : ComponentActivity() {
    /** An ID from a shinar://add link that opened the app, waiting to be offered. */
    private var addHandle by mutableStateOf<String?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        // Off the splash theme before anything is drawn, so its mark is not left behind the screens.
        setTheme(R.style.Theme_Ptranslate)
        super.onCreate(savedInstanceState)
        if (savedInstanceState == null) addHandle = handleIn(intent)
        val launch = getSharedPreferences("launch", MODE_PRIVATE)
        // The intro plays once, on the way to the welcome; anyone already signed in goes straight in.
        val playIntro = (application as PtranslateApp).container.account.session.value == null &&
            !launch.getBoolean(INTRO_SEEN, false)
        // Light status and navigation bar icons, over the dark design, whatever the phone's setting.
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(Color.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.dark(Color.TRANSPARENT),
        )
        setContent {
            PtranslateTheme {
                Box(Modifier.fillMaxSize()) {
                    Scaffold(modifier = Modifier.fillMaxSize()) { innerPadding ->
                        AppRoot(
                            addHandle = addHandle,
                            onAddHandleUsed = { addHandle = null },
                            modifier = Modifier.padding(innerPadding),
                        )
                    }
                    // The splash screen's mark, still showing until the app is ready to be seen.
                    var covering by rememberSaveable { mutableStateOf(true) }
                    if (covering) {
                        LaunchCover(playIntro) {
                            launch.edit().putBoolean(INTRO_SEEN, true).apply()
                            covering = false
                        }
                    }
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleIn(intent)?.let { addHandle = it }
    }

    private fun handleIn(intent: Intent?): String? = intent?.dataString?.let(::handleFromCode)

    private companion object {
        const val INTRO_SEEN = "intro_seen"
    }
}
