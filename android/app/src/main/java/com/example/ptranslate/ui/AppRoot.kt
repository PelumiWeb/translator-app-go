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
import com.example.ptranslate.BuildConfig
import com.example.ptranslate.PtranslateApp
import com.example.ptranslate.core.account.OnboardingStep
import com.example.ptranslate.ui.onboarding.GoogleSignIn
import com.example.ptranslate.ui.onboarding.HandleScreen
import com.example.ptranslate.ui.onboarding.LanguageScreen
import com.example.ptranslate.ui.onboarding.NameScreen
import com.example.ptranslate.ui.onboarding.OnboardingViewModel
import com.example.ptranslate.ui.onboarding.SignInScreen
import com.example.ptranslate.ui.onboarding.WelcomeScreen

/**
 * Chooses what the person sees from where they are: not signed in (welcome,
 * then sign in), part way through signing up (the step they reached), or in
 * the app. The account is the one source of truth, so signing out or a
 * session the server ended brings them back to the welcome from anywhere.
 *
 * @param addHandle an ID from a timbre://add link, offered once signed up.
 */
@Composable
fun AppRoot(addHandle: String?, onAddHandleUsed: () -> Unit, modifier: Modifier = Modifier) {
    val container = (LocalContext.current.applicationContext as PtranslateApp).container
    val session by container.account.session.collectAsStateWithLifecycle()
    val step by container.account.step.collectAsStateWithLifecycle()

    if (session == null) {
        var signingIn by rememberSaveable { mutableStateOf(false) }
        // Keyed to "signed out", so a new sign-in starts with fresh fields.
        val onboarding: OnboardingViewModel = viewModel(key = "onboarding-signed-out", factory = OnboardingViewModel.Factory)
        val state by onboarding.state.collectAsStateWithLifecycle()
        if (!signingIn) {
            WelcomeScreen(onGetStarted = { signingIn = true }, modifier = modifier)
        } else {
            BackHandler { signingIn = false }
            SignInScreen(
                state = state,
                googleSignIn = GoogleSignIn(BuildConfig.GOOGLE_WEB_CLIENT_ID),
                onSignIn = onboarding::onSignInClicked,
                onBack = { signingIn = false },
                modifier = modifier,
            )
        }
        return
    }

    if (step == OnboardingStep.DONE) {
        MainScreen(addHandle = addHandle, onAddHandleUsed = onAddHandleUsed, modifier = modifier)
        return
    }

    val onboarding: OnboardingViewModel = viewModel(key = "onboarding-${session?.profile?.id}", factory = OnboardingViewModel.Factory)
    val state by onboarding.state.collectAsStateWithLifecycle()
    val back = { onboarding.onBack(step) }
    BackHandler { back() }
    when (step) {
        OnboardingStep.NAME -> NameScreen(state, onboarding::onNameChanged, onboarding::onNameContinue, back, modifier)
        OnboardingStep.HANDLE -> HandleScreen(state, onboarding::onHandleChanged, onboarding::onHandleContinue, back, modifier)
        OnboardingStep.LANGUAGE -> LanguageScreen(state, container.phoneLanguage, onboarding::onLanguagePicked, onboarding::onLanguageContinue, back, modifier)
        OnboardingStep.VOICE -> VoiceSetupScreen(
            onDone = { container.account.moveTo(OnboardingStep.DONE) },
            modifier = modifier,
            step = 4,
            onBack = back,
        )
        OnboardingStep.DONE -> Unit
    }
}
