package com.example.ptranslate.ui.components

import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType

// The design's three forms of notice. None is red, and none blames the person:
// each says what happened and what to do next.

/** For a problem that fixes itself: a capsule at the top with a spinner. */
@Composable
fun Toast(text: String, modifier: Modifier = Modifier) {
    val colors = Pearl.colors
    Row(
        modifier
            .clip(CircleShape)
            .background(colors.bgRaised)
            .border(1.dp, colors.fg.copy(alpha = 0.08f), CircleShape)
            .padding(horizontal = 18.dp, vertical = 10.dp)
            .semantics { liveRegion = LiveRegionMode.Polite },
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        CircularProgressIndicator(color = colors.fg2, strokeWidth = 1.5.dp, modifier = Modifier.size(16.dp))
        Text(text, style = PearlType.body, color = colors.fg)
    }
}

/** For something the person must do: an icon, a title, a line, and the action. */
@Composable
fun Banner(icon: ImageVector, title: String, body: String, action: String, onAction: () -> Unit, modifier: Modifier = Modifier) {
    val colors = Pearl.colors
    Column(
        modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .background(colors.bgRaised)
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Icon(icon, contentDescription = null, tint = colors.fg, modifier = Modifier.size(22.dp))
            Column(Modifier.weight(1f)) {
                Text(title, style = PearlType.messageStrong, color = colors.fg)
                Text(body, style = PearlType.body, color = colors.fg2)
            }
        }
        PrimaryButton(action, onClick = onAction)
    }
}

/** For a recording that did not work: a line above the mic, with what to try. */
@Composable
fun Hint(text: String, modifier: Modifier = Modifier, icon: ImageVector? = PearlIcons.Retry) {
    val colors = Pearl.colors
    Row(
        modifier
            .clip(RoundedCornerShape(12.dp))
            .background(colors.bgRaised)
            .padding(horizontal = 16.dp, vertical = 12.dp)
            .semantics { liveRegion = LiveRegionMode.Polite },
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        icon?.let { Icon(it, contentDescription = null, tint = colors.fg, modifier = Modifier.padding(top = 1.dp).size(18.dp)) }
        Text(text, style = PearlType.body, color = colors.fg)
    }
}

/**
 * The welcome's two voices: a warm white set of rings for you and a mint one
 * for them, overlapping, each breathing on a three-second loop.
 */
@Composable
fun TwoVoices(modifier: Modifier = Modifier) {
    val colors = Pearl.colors
    val transition = rememberInfiniteTransition(label = "voices")
    val phase by transition.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(3000, easing = androidx.compose.animation.core.LinearEasing), RepeatMode.Restart),
        label = "phase",
    )
    Box(modifier.size(width = 260.dp, height = 180.dp), contentAlignment = Alignment.Center) {
        Canvas(Modifier.size(width = 260.dp, height = 180.dp)) {
            val ringMax = 90.dp.toPx()
            val core = 26.dp.toPx()
            val gap = 40.dp.toPx()
            fun voice(center: Offset, color: Color, offset: Float) {
                // Three rings a second apart, each growing and fading.
                for (i in 0 until 3) {
                    val t = ((phase + offset + i / 3f) % 1f)
                    drawCircle(color, radius = core + (ringMax - core) * t, center = center, alpha = 0.22f * (1 - t))
                }
                drawCircle(color, radius = core, center = center)
            }
            val mid = Offset(size.width / 2, size.height / 2)
            voice(mid.copy(x = mid.x - gap), colors.you, 0f)
            voice(mid.copy(x = mid.x + gap), colors.them, 0.5f / 3f)
        }
    }
}
