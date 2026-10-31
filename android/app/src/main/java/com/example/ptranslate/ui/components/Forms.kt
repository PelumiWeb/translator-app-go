package com.example.ptranslate.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType

/** The second action of a screen: the same capsule as [PrimaryButton], outlined. */
@Composable
fun SecondaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    icon: ImageVector? = null,
) {
    val colors = Pearl.colors
    Row(
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 52.dp)
            .clip(CircleShape)
            .border(1.dp, colors.fg.copy(alpha = 0.28f), CircleShape)
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick)
            .padding(horizontal = 24.dp, vertical = 14.dp),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        icon?.let { Icon(it, contentDescription = null, tint = colors.fg, modifier = Modifier.padding(end = 10.dp).size(20.dp)) }
        Text(text, style = PearlType.messageStrong, color = if (enabled) colors.fg else colors.fg3)
    }
}

/**
 * The design's text field: raised petrol, rounded, large text, outlined in
 * warm white while it has focus. [prefix] is shown dim before the text, as
 * the "@" of an ID.
 */
@Composable
fun PearlTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    modifier: Modifier = Modifier,
    placeholder: String = "",
    prefix: String? = null,
    trailing: @Composable (() -> Unit)? = null,
    leading: ImageVector? = null,
    keyboardOptions: KeyboardOptions = KeyboardOptions.Default,
    keyboardActions: KeyboardActions = KeyboardActions.Default,
    compact: Boolean = false,
) {
    val colors = Pearl.colors
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    val shape = RoundedCornerShape(12.dp)
    val textStyle = if (compact) PearlType.message else PearlType.field
    BasicTextField(
        value = value,
        onValueChange = onValueChange,
        singleLine = true,
        textStyle = textStyle.copy(color = colors.fg),
        cursorBrush = SolidColor(colors.fg),
        keyboardOptions = keyboardOptions,
        keyboardActions = keyboardActions,
        interactionSource = interaction,
        modifier = modifier
            .fillMaxWidth()
            .semantics { contentDescription = label },
        decorationBox = { field ->
            Row(
                Modifier
                    .heightIn(min = if (compact) 48.dp else 58.dp)
                    .clip(shape)
                    .background(colors.bgRaised)
                    .border(if (focused && !compact) 1.5.dp else 0.dp, if (focused && !compact) colors.fg else colors.bgRaised, shape)
                    .padding(horizontal = 16.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                leading?.let { Icon(it, contentDescription = null, tint = colors.fg2, modifier = Modifier.padding(end = 10.dp).size(20.dp)) }
                prefix?.let { Text(it, style = textStyle, color = colors.fg3) }
                Box(Modifier.weight(1f)) {
                    if (value.isEmpty() && placeholder.isNotEmpty()) {
                        Text(placeholder, style = textStyle, color = colors.fg3)
                    }
                    field()
                }
                trailing?.invoke()
            }
        },
    )
}

/**
 * The top of an onboarding step: back, then a bar of [total] segments with the
 * first [step] filled, then "2 of 5".
 */
@Composable
fun OnboardingHeader(step: Int, total: Int, onBack: (() -> Unit)?, modifier: Modifier = Modifier) {
    val colors = Pearl.colors
    Column(modifier.fillMaxWidth()) {
        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.heightIn(min = 48.dp)) {
            if (onBack != null) {
                IconButton(onClick = onBack) {
                    Icon(PearlIcons.Back, contentDescription = "Back", tint = colors.fg, modifier = Modifier.size(28.dp))
                }
            } else {
                Spacer(Modifier.size(48.dp))
            }
            Row(Modifier.weight(1f).padding(start = 8.dp), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                repeat(total) { index ->
                    Box(
                        Modifier
                            .weight(1f)
                            .height(3.dp)
                            .clip(CircleShape)
                            .background(if (index < step) colors.fg else colors.fg.copy(alpha = 0.2f)),
                    )
                }
            }
        }
        Text("$step of $total", style = PearlType.label, color = colors.fg2, modifier = Modifier.padding(top = 20.dp, start = 4.dp))
    }
}

/** A suggested value to pick, as the free IDs offered when one is taken. */
@Composable
fun SuggestionChip(text: String, onClick: () -> Unit, modifier: Modifier = Modifier) {
    Text(
        text,
        style = PearlType.body,
        color = Pearl.colors.fg,
        modifier = modifier
            .clip(CircleShape)
            .background(Pearl.colors.bgRaised)
            .clickable(role = Role.Button, onClick = onClick)
            .padding(horizontal = 14.dp, vertical = 10.dp),
    )
}
