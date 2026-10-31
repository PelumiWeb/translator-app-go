package com.example.ptranslate.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.example.ptranslate.PtranslateApp
import com.example.ptranslate.core.voice.VoiceSampleState
import com.example.ptranslate.ui.components.PearlIcons
import com.example.ptranslate.ui.onboarding.HandleScreen
import com.example.ptranslate.ui.onboarding.LanguageScreen
import com.example.ptranslate.ui.onboarding.NameScreen
import com.example.ptranslate.ui.onboarding.OnboardingViewModel
import com.example.ptranslate.ui.profile.PrivacyScreen
import com.example.ptranslate.ui.profile.ProfileDestination
import com.example.ptranslate.ui.profile.ProfileScreen
import com.example.ptranslate.ui.profile.ProfileViewModel
import com.example.ptranslate.ui.rooms.RoomScreen
import com.example.ptranslate.ui.rooms.RoomsScreen
import com.example.ptranslate.ui.rooms.RoomsViewModel
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType
import androidx.compose.ui.platform.LocalContext

private enum class Tab(val label: String, val icon: ImageVector) {
    ROOMS("Rooms", PearlIcons.Rooms),
    INTERPRETER("Interpreter", PearlIcons.Swap),
    PROFILE("Profile", PearlIcons.Person),
}

/**
 * The signed-up app: Rooms, the Interpreter and Profile in a Material
 * navigation bar, as the design asks on Android. A room, and the screens the
 * profile leads to, cover the bar; with this few, a state value does the job
 * of a navigation library, and back is handled by hand.
 *
 * @param addHandle an ID from a timbre://add link, to offer to add.
 */
@Composable
fun MainScreen(addHandle: String?, onAddHandleUsed: () -> Unit, modifier: Modifier = Modifier) {
    val container = (LocalContext.current.applicationContext as PtranslateApp).container
    val colors = Pearl.colors
    val translate: TranslateViewModel = viewModel(factory = TranslateViewModel.Factory)
    val rooms: RoomsViewModel = viewModel(factory = RoomsViewModel.Factory)
    val profile: ProfileViewModel = viewModel(factory = ProfileViewModel.Factory)
    val translateState by translate.state.collectAsStateWithLifecycle()
    val voice by container.voice.state.collectAsStateWithLifecycle()

    var tab by rememberSaveable { mutableStateOf(Tab.ROOMS) }
    var openRoom by rememberSaveable { mutableStateOf<String?>(null) }
    var destination by rememberSaveable { mutableStateOf<ProfileDestination?>(null) }

    LaunchedEffect(addHandle) {
        if (addHandle != null) {
            tab = Tab.ROOMS
            openRoom = null
        }
    }

    // What covers the tabs, if anything.
    val room = openRoom
    val covering = destination
    when {
        covering != null -> {
            BackHandler { destination = null }
            val close = { destination = null }
            when (covering) {
                ProfileDestination.NAME, ProfileDestination.HANDLE, ProfileDestination.LANGUAGE -> {
                    val edit: OnboardingViewModel = viewModel(key = "edit-$covering", factory = OnboardingViewModel.EditFactory)
                    val state by edit.state.collectAsStateWithLifecycle()
                    LaunchedEffect(state.saved) { if (state.saved) close() }
                    when (covering) {
                        ProfileDestination.NAME -> NameScreen(state, edit::onNameChanged, edit::onNameContinue, close, modifier, step = null, button = "Save")
                        ProfileDestination.HANDLE -> HandleScreen(state, edit::onHandleChanged, edit::onHandleContinue, close, modifier, step = null, button = "Save")
                        else -> LanguageScreen(state, container.phoneLanguage, edit::onLanguagePicked, edit::onLanguageContinue, close, modifier, step = null, button = "Save")
                    }
                }
                ProfileDestination.PRIVACY -> PrivacyScreen(onBack = close, modifier = modifier)
                ProfileDestination.ADVANCED -> SettingsScreen(viewModel = translate, onBack = close, modifier = modifier)
                ProfileDestination.RECORD_VOICE -> VoiceSetupScreen(onDone = {
                    close()
                    profile.refresh()
                }, modifier = modifier)
            }
        }
        room != null -> {
            BackHandler { openRoom = null }
            RoomScreen(
                roomId = room,
                onBack = { openRoom = null },
                onRecordVoice = { destination = ProfileDestination.RECORD_VOICE },
                modifier = modifier,
            )
        }
        else -> Column(modifier.fillMaxSize()) {
            Box(Modifier.weight(1f)) {
                when (tab) {
                    Tab.ROOMS -> RoomsScreen(
                        viewModel = rooms,
                        onOpenRoom = { openRoom = it.id },
                        addHandle = addHandle,
                        onAddHandleUsed = onAddHandleUsed,
                    )
                    Tab.INTERPRETER -> TranslateScreen(
                        viewModel = translate,
                        savedVoiceMs = (voice as? VoiceSampleState.Present)?.durationMs,
                    )
                    Tab.PROFILE -> ProfileScreen(
                        viewModel = profile,
                        translate = translateState,
                        onOpen = { destination = it },
                    )
                }
            }
            NavigationBar(containerColor = colors.bgRaised) {
                Tab.entries.forEach { item ->
                    NavigationBarItem(
                        selected = tab == item,
                        onClick = { tab = item },
                        icon = { Icon(item.icon, contentDescription = null, modifier = Modifier.size(24.dp)) },
                        label = { Text(item.label, style = PearlType.label) },
                        colors = NavigationBarItemDefaults.colors(
                            selectedIconColor = colors.fg,
                            selectedTextColor = colors.fg,
                            unselectedIconColor = colors.fg2,
                            unselectedTextColor = colors.fg2,
                            indicatorColor = colors.fg.copy(alpha = 0.12f),
                        ),
                    )
                }
            }
        }
    }
}
