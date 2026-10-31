package com.example.ptranslate.ui.profile

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.example.ptranslate.ui.components.PearlIcons
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType

/** What is kept, and why, in plain words. The same text on iOS. */
@Composable
fun PrivacyScreen(onBack: () -> Unit, modifier: Modifier = Modifier) {
    val colors = Pearl.colors
    Column(modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = 16.dp, vertical = 8.dp)) {
        IconButton(onClick = onBack) {
            Icon(PearlIcons.Back, contentDescription = "Back", tint = colors.fg, modifier = Modifier.size(28.dp))
        }
        Column(Modifier.padding(horizontal = 12.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
            Text("What we keep, and why", style = PearlType.screenTitle, color = colors.fg, modifier = Modifier.padding(top = 8.dp).semantics { heading() })
            PRIVACY_SECTIONS.forEach { (title, body) ->
                Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text(title, style = PearlType.messageStrong, color = colors.fg)
                    Text(body, style = PearlType.message, color = colors.fg2)
                }
            }
        }
    }
}

private val PRIVACY_SECTIONS = listOf(
    "Your account" to
        "Your name, your ID and the language you speak, so people can find you and hear you in their language. " +
        "We keep your email address only if Google has confirmed it, to recognise your account.",
    "Your voice" to
        "The recording you made, on our server, with the date you agreed. It is used only to speak your own words " +
        "when someone plays a message you sent. Fish Audio makes the speech for us. It is never given to other people.",
    "Your messages" to
        "What you record in a room, the words written down from it, its translation, and the translation spoken in " +
        "your voice. Google Cloud Translation translates the words. They are kept so the people in the room can play them.",
    "Deleting" to
        "Delete your voice in Profile, under Your voice: your messages then use the phone's voice. Delete your account, " +
        "and your profile, your voice and everything you said in your rooms are deleted with it.",
)
