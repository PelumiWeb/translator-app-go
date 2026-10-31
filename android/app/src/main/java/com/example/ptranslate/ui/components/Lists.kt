package com.example.ptranslate.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType

/**
 * A group of rows on raised petrol, as in the design's settings and language
 * list: a small title above, an optional footer below, hairlines between.
 */
@Composable
fun GroupedSection(
    title: String?,
    modifier: Modifier = Modifier,
    footer: String? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    val colors = Pearl.colors
    Column(modifier.fillMaxWidth()) {
        title?.let { Text(it, style = PearlType.label, color = colors.fg2, modifier = Modifier.padding(start = 16.dp, bottom = 8.dp)) }
        Column(Modifier.clip(RoundedCornerShape(12.dp)).background(colors.bgRaised), content = content)
        footer?.let { Text(it, style = PearlType.small, color = colors.fg2, modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 8.dp)) }
    }
}

/**
 * One row of a [GroupedSection]: a title with an optional line under it, an
 * optional value on the right, and a chevron when it leads somewhere.
 */
@Composable
fun GroupRow(
    title: String,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    value: String? = null,
    titleColor: Color = Pearl.colors.fg,
    chevron: Boolean = false,
    divider: Boolean = true,
    leading: (@Composable () -> Unit)? = null,
    trailing: (@Composable () -> Unit)? = null,
    onClick: (() -> Unit)? = null,
) {
    val colors = Pearl.colors
    Column {
        Row(
            modifier = modifier
                .fillMaxWidth()
                .heightIn(min = 52.dp)
                .then(if (onClick != null) Modifier.clickable(role = Role.Button, onClick = onClick) else Modifier)
                .padding(horizontal = 16.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            leading?.invoke()
            Column(Modifier.weight(1f)) {
                Text(title, style = PearlType.message.copy(fontSize = 17.sp), color = titleColor)
                subtitle?.let { Text(it, style = PearlType.small, color = colors.fg2) }
            }
            value?.let { Text(it, style = PearlType.message, color = colors.fg2) }
            trailing?.invoke()
            if (chevron) Icon(PearlIcons.ChevronRight, contentDescription = null, tint = colors.fg2, modifier = Modifier.size(18.dp))
        }
        if (divider) HorizontalDivider(color = colors.fg.copy(alpha = 0.08f), modifier = Modifier.padding(start = 16.dp))
    }
}

/** A person's initial in a circle: raised petrol and mint for others, warm white for the user. */
@Composable
fun Avatar(
    name: String,
    modifier: Modifier = Modifier,
    size: Dp = 48.dp,
    fill: Color = Pearl.colors.bgRaised,
    content: Color = Pearl.colors.them,
    style: TextStyle = PearlType.headline,
) {
    Box(modifier.size(size).clip(CircleShape).background(fill), contentAlignment = Alignment.Center) {
        Text(name.trim().firstOrNull()?.uppercase() ?: "?", style = style, color = content)
    }
}
