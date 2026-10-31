package com.example.ptranslate.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.example.ptranslate.core.Language
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType
import java.util.Locale

/** The language's name in itself, as the design asks: "Español", not "Spanish". */
fun Language.ownName(): String {
    val locale = Locale.forLanguageTag(tag)
    return locale.getDisplayLanguage(locale).replaceFirstChar { it.titlecase(locale) }.ifBlank { tag }
}

/** The language's name in the phone's language, to help find it in a list. */
fun Language.localName(): String =
    Locale.forLanguageTag(tag).displayLanguage.replaceFirstChar { it.titlecase() }.ifBlank { tag }

/**
 * The two languages of the interpreter: the speaker's in the colour that means
 * "you", the listener's in the colour that means "them", with a swap between.
 */
@Composable
fun LanguageChips(
    source: Language,
    target: Language,
    onSourceClick: () -> Unit,
    onTargetClick: () -> Unit,
    onSwapClick: () -> Unit,
    enabled: Boolean,
    swapEnabled: Boolean,
    modifier: Modifier = Modifier,
) {
    val colors = Pearl.colors
    Row(modifier = modifier, verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Chip(source.ownName(), "You speak ${source.localName()}. Change", colors.you, colors.onYou, enabled, onSourceClick)
        IconButton(onClick = onSwapClick, enabled = enabled && swapEnabled) {
            Icon(PearlIcons.Swap, contentDescription = "Swap the languages", tint = colors.fg, modifier = Modifier.size(18.dp))
        }
        Chip(target.ownName(), "They hear ${target.localName()}. Change", colors.bgRaised, colors.them, enabled, onTargetClick)
    }
}

@Composable
private fun Chip(text: String, label: String, fill: Color, content: Color, enabled: Boolean, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .clip(CircleShape)
            .background(fill)
            .clickable(enabled = enabled, role = Role.Button, onClickLabel = label, onClick = onClick)
            .padding(horizontal = 14.dp, vertical = 8.dp),
    ) {
        Text(text, style = PearlType.chip, color = content)
    }
}

/** A searchable list of languages in a sheet. Each shows its own name, and the phone's name for it. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LanguagePickerSheet(
    title: String,
    options: List<Language>,
    selected: Language,
    onSelect: (Language) -> Unit,
    onDismiss: () -> Unit,
) {
    val colors = Pearl.colors
    var query by remember { mutableStateOf("") }
    val sorted = remember(options) { options.sortedBy { it.localName() } }
    val shown = sorted.filter {
        query.isBlank() || it.ownName().contains(query, ignoreCase = true) || it.localName().contains(query, ignoreCase = true)
    }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
        containerColor = colors.bgRaised,
    ) {
        Column(Modifier.padding(horizontal = 16.dp)) {
            Text(title, style = PearlType.title, color = colors.fg, modifier = Modifier.padding(bottom = 12.dp))
            OutlinedTextField(
                value = query,
                onValueChange = { query = it },
                placeholder = { Text("Search", style = PearlType.message) },
                leadingIcon = { Icon(PearlIcons.Search, contentDescription = null, modifier = Modifier.size(18.dp)) },
                singleLine = true,
                textStyle = PearlType.message,
                modifier = Modifier.fillMaxWidth(),
            )
        }
        LazyColumn(Modifier.padding(top = 8.dp)) {
            items(shown, key = { it.tag }) { language ->
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable(role = Role.Button) { onSelect(language) }
                        .padding(horizontal = 16.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(Modifier.weight(1f)) {
                        Text(language.ownName(), style = PearlType.message, color = colors.fg)
                        if (language.localName() != language.ownName()) {
                            Text(language.localName(), style = PearlType.small, color = colors.fg2)
                        }
                    }
                    if (language == selected) {
                        Icon(PearlIcons.Check, contentDescription = "Selected", tint = colors.them, modifier = Modifier.size(20.dp))
                    }
                }
            }
        }
    }
}
