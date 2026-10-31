package com.example.ptranslate.ui.onboarding

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.account.Nonce
import com.example.ptranslate.core.stt.WhisperLanguages
import com.example.ptranslate.ui.components.GroupRow
import com.example.ptranslate.ui.components.Notice
import com.example.ptranslate.ui.components.OnboardingHeader
import com.example.ptranslate.ui.components.PearlIcons
import com.example.ptranslate.ui.components.PearlTextField
import com.example.ptranslate.ui.components.PrimaryButton
import com.example.ptranslate.ui.components.SuggestionChip
import com.example.ptranslate.ui.components.TwoVoices
import com.example.ptranslate.ui.components.localName
import com.example.ptranslate.ui.components.ownName
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType
import java.util.Locale

/** The steps of signing up shown with a progress bar. The speech model step is not designed yet. */
const val ONBOARDING_STEPS = 4

/** The first screen: the promise, and a way in. */
@Composable
fun WelcomeScreen(onGetStarted: () -> Unit, modifier: Modifier = Modifier) {
    val colors = Pearl.colors
    Column(modifier.fillMaxSize().padding(horizontal = 28.dp, vertical = 24.dp)) {
        Text("timbre", style = PearlType.headline.copy(fontSize = 18.sp), color = colors.fg)
        Box(Modifier.weight(1f).fillMaxWidth(), contentAlignment = Alignment.Center) {
            TwoVoices()
        }
        Text(
            "Talk to anyone. Be heard in your own voice.",
            style = PearlType.hero,
            color = colors.fg,
            modifier = Modifier.semantics { heading() },
        )
        Text(
            "You speak your language. They hear theirs, in your voice.",
            style = PearlType.message.copy(fontSize = 17.sp),
            color = colors.fg2,
            modifier = Modifier.padding(top = 12.dp, bottom = 28.dp),
        )
        PrimaryButton("Get started", onClick = onGetStarted)
    }
}

/** Sign in with Google: Android's own sheet does the choosing. */
@Composable
fun SignInScreen(
    state: OnboardingUiState,
    googleSignIn: GoogleSignIn,
    onSignIn: (getToken: suspend (Nonce) -> String) -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val colors = Pearl.colors
    val context = LocalContext.current
    Column(modifier.fillMaxSize().padding(horizontal = 28.dp, vertical = 16.dp)) {
        IconButton(onClick = onBack, modifier = Modifier.padding(start = 0.dp)) {
            Icon(PearlIcons.Back, contentDescription = "Back", tint = colors.fg, modifier = Modifier.size(28.dp))
        }
        Text(
            "Sign in to keep your conversations.",
            style = PearlType.screenTitle,
            color = colors.fg,
            modifier = Modifier.padding(top = 24.dp).semantics { heading() },
        )
        Text(
            "This is only to find your account on any phone. We never post anything.",
            style = PearlType.message.copy(fontSize = 17.sp),
            color = colors.fg2,
            modifier = Modifier.padding(top = 12.dp),
        )
        Spacer(Modifier.weight(1f))
        state.signInProblem?.let { Notice(it, Modifier.padding(bottom = 16.dp)) }
        if (!googleSignIn.configured) {
            Notice("Sign in with Google isn't set up in this build yet.", Modifier.padding(bottom = 16.dp))
        }
        GoogleButton(
            busy = state.signingIn,
            enabled = googleSignIn.configured,
            onClick = { onSignIn { nonce -> googleSignIn.idToken(context, nonce) } },
        )
        Text(
            "Email sign-in is coming soon.",
            style = PearlType.body,
            color = colors.fg3,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth().padding(top = 18.dp),
        )
        Text(
            "By continuing you agree to the Terms and Privacy Policy.",
            style = PearlType.small,
            color = colors.fg2,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth().padding(top = 20.dp, bottom = 8.dp),
        )
    }
}

/**
 * Google's dark button: near-black, outlined, "Sign in with Google". Google's
 * branding asks for its "G" mark beside the words; it comes with the release
 * assets (milestone 18).
 */
@Composable
private fun GoogleButton(busy: Boolean, enabled: Boolean, onClick: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 52.dp)
            .clip(CircleShape)
            .background(Color(0xFF131314))
            .border(1.dp, Color(0xFF8E918F), CircleShape)
            .clickable(enabled = enabled && !busy, role = Role.Button, onClick = onClick)
            .padding(horizontal = 24.dp, vertical = 14.dp),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (busy) {
            CircularProgressIndicator(color = Color(0xFFE3E3E3), strokeWidth = 2.dp, modifier = Modifier.size(18.dp))
        } else {
            Text(
                "Sign in with Google",
                style = PearlType.messageStrong.copy(fontSize = 17.sp),
                color = if (enabled) Color(0xFFE3E3E3) else Color(0xFFE3E3E3).copy(alpha = 0.4f),
            )
        }
    }
}

/**
 * The frame of steps 1 to 3: header, question, content, and the button kept above the keyboard. With no [step], as
 * when editing from the profile, the header is a back button alone.
 */
@Composable
private fun StepFrame(
    step: Int?,
    title: String,
    subtitle: String?,
    button: String,
    buttonEnabled: Boolean,
    saving: Boolean,
    problem: String?,
    onBack: () -> Unit,
    onContinue: () -> Unit,
    modifier: Modifier = Modifier,
    scrolls: Boolean = true,
    content: @Composable ColumnScope.() -> Unit,
) {
    val colors = Pearl.colors
    Column(modifier.fillMaxSize().imePadding().padding(horizontal = 16.dp, vertical = 8.dp)) {
        if (step != null) {
            OnboardingHeader(step = step, total = ONBOARDING_STEPS, onBack = onBack, modifier = Modifier.padding(horizontal = 4.dp))
        } else {
            IconButton(onClick = onBack) {
                Icon(PearlIcons.Back, contentDescription = "Back", tint = colors.fg, modifier = Modifier.size(28.dp))
            }
        }
        Column(
            Modifier
                .weight(1f)
                .then(if (scrolls) Modifier.verticalScroll(rememberScrollState()) else Modifier)
                .padding(horizontal = 12.dp),
        ) {
            Text(title, style = PearlType.screenTitle, color = colors.fg, modifier = Modifier.padding(top = 8.dp).semantics { heading() })
            subtitle?.let { Text(it, style = PearlType.message.copy(fontSize = 17.sp), color = colors.fg2, modifier = Modifier.padding(top = 8.dp)) }
            Spacer(Modifier.size(20.dp))
            content()
            problem?.let { Notice(it, Modifier.padding(top = 16.dp)) }
        }
        PrimaryButton(if (saving) "Saving…" else button, onClick = onContinue, enabled = buttonEnabled && !saving, modifier = Modifier.padding(top = 12.dp, bottom = 8.dp))
    }
}

@Composable
fun NameScreen(
    state: OnboardingUiState,
    onNameChange: (String) -> Unit,
    onContinue: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
    step: Int? = 1,
    button: String = "Continue",
) {
    StepFrame(
        step = step,
        title = "What should people call you?",
        subtitle = null,
        button = button,
        buttonEnabled = state.name.isNotBlank(),
        saving = state.saving,
        problem = state.problem,
        onBack = onBack,
        onContinue = onContinue,
        modifier = modifier,
    ) {
        PearlTextField(
            value = state.name,
            onValueChange = onNameChange,
            label = "Your name",
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Words, imeAction = ImeAction.Done),
            keyboardActions = KeyboardActions(onDone = { onContinue() }),
        )
        Text("Shown to the people you talk to.", style = PearlType.body, color = Pearl.colors.fg2, modifier = Modifier.padding(top = 14.dp))
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun HandleScreen(
    state: OnboardingUiState,
    onHandleChange: (String) -> Unit,
    onContinue: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
    step: Int? = 2,
    button: String = "Continue",
) {
    val colors = Pearl.colors
    StepFrame(
        step = step,
        title = "Pick your ID.",
        subtitle = "This is how people find you.",
        button = button,
        buttonEnabled = state.handleState == HandleState.Available,
        saving = state.saving,
        problem = state.problem,
        onBack = onBack,
        onContinue = onContinue,
        modifier = modifier,
    ) {
        PearlTextField(
            value = state.handle,
            onValueChange = onHandleChange,
            label = "Your ID",
            prefix = "@",
            keyboardOptions = KeyboardOptions(
                capitalization = KeyboardCapitalization.None,
                autoCorrectEnabled = false,
                keyboardType = KeyboardType.Ascii,
                imeAction = ImeAction.Done,
            ),
            keyboardActions = KeyboardActions(onDone = { onContinue() }),
            trailing = {
                when (state.handleState) {
                    HandleState.Checking -> CircularProgressIndicator(color = colors.fg2, strokeWidth = 1.5.dp, modifier = Modifier.size(18.dp))
                    HandleState.Available -> Icon(PearlIcons.Check, contentDescription = null, tint = colors.fg, modifier = Modifier.size(22.dp))
                    else -> Unit
                }
            },
        )
        when (val handle = state.handleState) {
            HandleState.Available -> Text("Available", style = PearlType.body.copy(fontWeight = FontWeight.Medium), color = colors.fg, modifier = Modifier.padding(top = 12.dp))
            is HandleState.Invalid -> Text(handle.rule, style = PearlType.body, color = colors.fg, modifier = Modifier.padding(top = 12.dp))
            is HandleState.Unchecked -> Text(handle.reason, style = PearlType.body, color = colors.fg, modifier = Modifier.padding(top = 12.dp))
            is HandleState.Taken -> {
                Text("@${state.handle} is taken. These are free:", style = PearlType.body, color = colors.fg2, modifier = Modifier.padding(top = 12.dp, bottom = 10.dp))
                FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    handle.suggestions.forEach { suggestion -> SuggestionChip("@$suggestion", onClick = { onHandleChange(suggestion) }) }
                }
            }
            HandleState.Checking, HandleState.Empty -> Unit
        }
        Text("Letters, numbers, dots and underscores.", style = PearlType.body, color = colors.fg2, modifier = Modifier.padding(top = 24.dp))
    }
}

/** Which language the person speaks: theirs, suggested from the phone, or any of Whisper's 99. */
@Composable
fun LanguageScreen(
    state: OnboardingUiState,
    phoneLanguage: Language?,
    onPick: (Language) -> Unit,
    onContinue: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
    step: Int? = 3,
    button: String? = null,
) {
    val colors = Pearl.colors
    var query by remember { mutableStateOf("") }
    val all = remember { WhisperLanguages.all.sortedBy { it.localName() } }
    val shown = all.filter {
        query.isBlank() || it.ownName().contains(query, ignoreCase = true) || it.localName().contains(query, ignoreCase = true)
    }
    StepFrame(
        step = step,
        title = "Which language do you speak?",
        subtitle = null,
        button = button ?: state.language?.let { "Continue in ${it.ownName()}" } ?: "Continue",
        buttonEnabled = state.language != null,
        saving = state.saving,
        problem = state.problem,
        onBack = onBack,
        onContinue = onContinue,
        modifier = modifier,
        scrolls = false,
    ) {
        PearlTextField(
            value = query,
            onValueChange = { query = it },
            label = "Search languages",
            placeholder = "Search ${all.size} languages",
            leading = PearlIcons.Search,
            compact = true,
        )
        LazyColumn(Modifier.padding(top = 16.dp)) {
            if (phoneLanguage != null && query.isBlank()) {
                item { SectionLabel("From your phone") }
                item {
                    LanguageRow(
                        language = phoneLanguage,
                        detail = Locale.getDefault().displayCountry.ifBlank { null },
                        selected = state.language == phoneLanguage,
                        onClick = { onPick(phoneLanguage) },
                        divider = false,
                        roundTop = true,
                        roundBottom = true,
                    )
                }
                item { SectionLabel("All languages", Modifier.padding(top = 16.dp)) }
            }
            items(shown, key = { it.tag }) { language ->
                LanguageRow(
                    language = language,
                    detail = null,
                    selected = state.language == language,
                    onClick = { onPick(language) },
                    divider = language != shown.last(),
                    roundTop = language == shown.first(),
                    roundBottom = language == shown.last(),
                )
            }
        }
    }
}

@Composable
private fun SectionLabel(text: String, modifier: Modifier = Modifier) {
    Text(text, style = PearlType.label, color = Pearl.colors.fg2, modifier = modifier.padding(start = 16.dp, bottom = 8.dp))
}

/** A row of the language list. The rows are drawn one by one, so each rounds its own corners. */
@Composable
private fun LanguageRow(
    language: Language,
    detail: String?,
    selected: Boolean,
    onClick: () -> Unit,
    divider: Boolean,
    roundTop: Boolean,
    roundBottom: Boolean,
) {
    val colors = Pearl.colors
    val corner = 12.dp
    val shape = androidx.compose.foundation.shape.RoundedCornerShape(
        topStart = if (roundTop) corner else 0.dp,
        topEnd = if (roundTop) corner else 0.dp,
        bottomStart = if (roundBottom) corner else 0.dp,
        bottomEnd = if (roundBottom) corner else 0.dp,
    )
    Column(Modifier.clip(shape).background(colors.bgRaised)) {
        GroupRow(
            title = language.ownName(),
            subtitle = detail,
            value = if (detail == null && language.localName() != language.ownName()) language.localName() else null,
            divider = divider,
            trailing = if (selected) {
                { Icon(PearlIcons.Check, contentDescription = "Selected", tint = colors.fg, modifier = Modifier.size(22.dp)) }
            } else {
                null
            },
            onClick = onClick,
        )
    }
}
