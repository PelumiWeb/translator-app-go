package com.example.ptranslate.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.unit.dp
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType

/** The main action of a screen: a capsule in the colour that means "you". */
@Composable
fun PrimaryButton(text: String, onClick: () -> Unit, modifier: Modifier = Modifier, enabled: Boolean = true) {
    val colors = Pearl.colors
    Box(
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 52.dp)
            .clip(CircleShape)
            .background(if (enabled) colors.you else colors.you.copy(alpha = 0.3f))
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick)
            .padding(horizontal = 24.dp, vertical = 14.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(text, style = PearlType.chip.copy(fontSize = PearlType.message.fontSize), color = colors.onYou)
    }
}

/** A quiet action: underlined text, as "Not now, use the phone's voice" in the design. */
@Composable
fun LinkButton(text: String, onClick: () -> Unit, modifier: Modifier = Modifier, color: Color = Pearl.colors.fg.copy(alpha = 0.72f)) {
    Text(
        text = text,
        style = PearlType.body.copy(textDecoration = TextDecoration.Underline),
        color = color,
        textAlign = TextAlign.Center,
        modifier = modifier
            .clip(CircleShape)
            .clickable(role = Role.Button, onClick = onClick)
            .padding(horizontal = 12.dp, vertical = 10.dp),
    )
}

/**
 * Something the person should know, never styled as an alarm: warm text with
 * an outline icon, as the handoff suggests for errors.
 */
@Composable
fun Notice(text: String, modifier: Modifier = Modifier) {
    Row(modifier = modifier, horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.Top) {
        Icon(PearlIcons.Info, contentDescription = null, tint = Pearl.colors.fg, modifier = Modifier.padding(top = 1.dp).size(18.dp))
        Text(text, style = PearlType.body, color = Pearl.colors.fg)
    }
}

/**
 * The design's dot waveform: 22 dots whose size follows [levels], the played
 * part ([played], 0 to 1) at full strength and the rest faded.
 */
@Composable
fun DotWaveform(levels: List<Float>, color: Color, modifier: Modifier = Modifier, played: Float = 1f) {
    Canvas(modifier.heightIn(min = 20.dp).fillMaxWidth()) {
        if (levels.isEmpty()) return@Canvas
        val step = size.width / levels.size
        val smallest = 3.dp.toPx()
        val largest = 10.dp.toPx()
        levels.forEachIndexed { index, level ->
            val diameter = smallest + (largest - smallest) * level.coerceIn(0f, 1f)
            val isPlayed = (index + 0.5f) / levels.size <= played
            drawCircle(
                color = color,
                radius = diameter / 2,
                center = Offset(step * (index + 0.5f), size.height / 2),
                alpha = if (isPlayed) 1f else 0.3f,
            )
        }
    }
}

/** A round button in the colour that means "you", for playing back a recording. */
@Composable
fun RoundIconButton(icon: androidx.compose.ui.graphics.vector.ImageVector, label: String, onClick: () -> Unit, modifier: Modifier = Modifier) {
    Box(
        modifier = modifier
            .size(44.dp)
            .clip(CircleShape)
            .background(Pearl.colors.you)
            .clickable(role = Role.Button, onClickLabel = label, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Icon(icon, contentDescription = label, tint = Pearl.colors.onYou, modifier = Modifier.size(16.dp))
    }
}
