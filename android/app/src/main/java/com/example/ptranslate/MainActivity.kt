package com.example.ptranslate

import android.content.Intent
import android.graphics.Color
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.SystemBarStyle
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import com.example.ptranslate.ui.AppRoot
import com.example.ptranslate.ui.rooms.handleFromCode
import com.example.ptranslate.ui.theme.PtranslateTheme

class MainActivity : ComponentActivity() {
    /** An ID from a timbre://add link that opened the app, waiting to be offered. */
    private var addHandle by mutableStateOf<String?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (savedInstanceState == null) addHandle = handleIn(intent)
        // Light status and navigation bar icons, over the dark design, whatever the phone's setting.
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(Color.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.dark(Color.TRANSPARENT),
        )
        setContent {
            PtranslateTheme {
                Scaffold(modifier = Modifier.fillMaxSize()) { innerPadding ->
                    AppRoot(
                        addHandle = addHandle,
                        onAddHandleUsed = { addHandle = null },
                        modifier = Modifier.padding(innerPadding),
                    )
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleIn(intent)?.let { addHandle = it }
    }

    private fun handleIn(intent: Intent?): String? = intent?.dataString?.let(::handleFromCode)
}
