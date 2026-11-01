package com.example.ptranslate.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.clipPath
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import com.example.ptranslate.R
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PlexSans

/**
 * The logo's mark (docs/design/pearl, "Logo and splash"): two voices, mint
 * for them on the left and warm white for you on the right, overlapping above
 * a horizon line. The intro moves the circles [apart] (in the logo's units),
 * fades the overlap ([lens]) and sends a [ripple] out from each, 0 to 1; at
 * rest it is the logo.
 */
@Composable
fun ShinarMark(modifier: Modifier = Modifier, apart: Float = 0f, lens: Float = 1f, ripple: Float = 0f) {
    val colors = Pearl.colors
    Canvas(modifier.aspectRatio(108f / 50f)) {
        // The logo's viewBox is 108 by 50 units, from (6, 16).
        val unit = size.width / 108f
        fun point(x: Float, y: Float) = Offset((x - 6f) * unit, (y - 16f) * unit)

        drawLine(colors.fg.copy(alpha = 0.5f), point(8f, 62f), point(112f, 62f), strokeWidth = 2.5f * unit, cap = StrokeCap.Round)
        val radius = 22f * unit
        val left = point(46f - apart, 40f)
        val right = point(74f + apart, 40f)
        drawCircle(colors.them, radius, left)
        drawCircle(colors.you, radius, right)
        if (lens > 0f) {
            clipPath(Path().apply { addOval(Rect(left, radius)) }) {
                drawCircle(colors.lens, radius, right, alpha = lens)
            }
        }
        if (ripple > 0f && ripple < 1f) {
            val grown = radius * (1f + 0.6f * ripple)
            val ring = Stroke(width = 1.25f * unit)
            drawCircle(colors.them, grown, left, alpha = 0.6f * (1f - ripple), style = ring)
            drawCircle(colors.you, grown, right, alpha = 0.6f * (1f - ripple), style = ring)
        }
    }
}

/** The name as the logo writes it: lowercase, IBM Plex Sans Medium, tracked in. */
@Composable
fun Wordmark(size: TextUnit, modifier: Modifier = Modifier) {
    val name = stringResource(R.string.app_name)
    Text(
        name.lowercase(),
        style = TextStyle(fontFamily = PlexSans, fontWeight = FontWeight.Medium, fontSize = size, letterSpacing = (-0.04).em),
        color = Pearl.colors.fg,
        modifier = modifier.clearAndSetSemantics { contentDescription = name },
    )
}

/** The mark and the name side by side, as at the top of the welcome. */
@Composable
fun Lockup(modifier: Modifier = Modifier, markWidth: Dp = 40.dp, nameSize: TextUnit = 20.sp) {
    Row(modifier, horizontalArrangement = Arrangement.spacedBy(markWidth * 0.15f), verticalAlignment = Alignment.CenterVertically) {
        ShinarMark(Modifier.width(markWidth).clearAndSetSemantics {})
        Wordmark(nameSize)
    }
}
