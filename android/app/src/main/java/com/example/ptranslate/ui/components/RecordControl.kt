package com.example.ptranslate.ui.components

import android.provider.Settings
import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.example.ptranslate.ui.theme.Pearl

/** What the big circle is doing. Each look comes from the design handoff. */
enum class RecordControlState {
    /** Waiting: a microphone, in a still halo. */
    IDLE,

    /** Listening: a stop square, with rings spreading out. */
    RECORDING,

    /** Writing down or translating: calm rings, a spinner in the circle. */
    WORKING,

    /** Making the voice, 2 to 5 seconds: calm rings around a speaker. */
    PREPARING_VOICE,

    /** The translation can be heard: a speaker, in a still halo. The sound is the feedback. */
    SPEAKING,
}

/**
 * The large record control of the design: a circle in the colour that means
 * "you", inside an area where rings show that something is happening. The
 * rings are a fixed loop for now; the design asks for them to follow the
 * microphone's level later.
 *
 * @param label what the control does now, for TalkBack ("Start recording").
 * @param value its state in words, for TalkBack ("0:12").
 */
@Composable
fun RecordControl(
    state: RecordControlState,
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    value: String? = null,
    area: Dp = 200.dp,
    circle: Dp = 96.dp,
) {
    val colors = Pearl.colors
    val reduceMotion = rememberReduceMotion()
    val animated = !reduceMotion && state != RecordControlState.IDLE && state != RecordControlState.SPEAKING

    Box(
        modifier = modifier
            .size(area)
            .semantics(mergeDescendants = true) {
                contentDescription = label
                value?.let { stateDescription = it }
            },
        contentAlignment = Alignment.Center,
    ) {
        if (animated) {
            Rings(
                color = colors.youRing,
                periodMs = if (state == RecordControlState.RECORDING) 2_400 else 3_200,
            )
        } else {
            Halo(color = colors.youHalo, size = circle + (area - circle) * 0.45f, breathing = reduceMotion && state != RecordControlState.IDLE)
        }
        Box(
            modifier = Modifier
                .size(circle)
                .clip(CircleShape)
                .background(colors.you)
                .clickable(enabled = enabled, role = Role.Button, onClickLabel = label, onClick = onClick),
            contentAlignment = Alignment.Center,
        ) {
            when (state) {
                RecordControlState.IDLE -> Glyph(PearlIcons.Mic, colors.onYou, 28.dp)
                RecordControlState.RECORDING -> Box(
                    Modifier
                        .size(22.dp)
                        .background(colors.onYou, RoundedCornerShape(4.dp)),
                )
                RecordControlState.WORKING -> CircularProgressIndicator(
                    modifier = Modifier.size(26.dp),
                    color = colors.onYou,
                    strokeWidth = 2.dp,
                )
                RecordControlState.PREPARING_VOICE, RecordControlState.SPEAKING -> Glyph(PearlIcons.Speaker, colors.onYou, 28.dp)
            }
        }
    }
}

@Composable
private fun Glyph(icon: ImageVector, tint: Color, size: Dp) {
    Icon(icon, contentDescription = null, tint = tint, modifier = Modifier.size(size))
}

/** Three filled rings that grow and fade, one after another. */
@Composable
private fun Rings(color: Color, periodMs: Int) {
    val easing = remember { CubicBezierEasing(0.22f, 0.61f, 0.36f, 1f) }
    val transition = rememberInfiniteTransition(label = "rings")
    val phase by transition.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(periodMs, easing = LinearEasing)),
        label = "phase",
    )
    Canvas(Modifier.fillMaxSize()) {
        val radius = size.minDimension / 2
        for (ring in 0 until 3) {
            val progress = easing.transform((phase + ring / 3f) % 1f)
            drawCircle(
                color = color,
                radius = radius * (0.48f + 0.52f * progress),
                alpha = 0.9f * (1f - progress),
            )
        }
    }
}

/** A still halo; with reduced motion it fades slowly in and out instead of rings. */
@Composable
private fun Halo(color: Color, size: Dp, breathing: Boolean) {
    val alpha = if (breathing) {
        val transition = rememberInfiniteTransition(label = "halo")
        val value by transition.animateFloat(
            initialValue = 0.5f,
            targetValue = 1f,
            animationSpec = infiniteRepeatable(tween(1_600), RepeatMode.Reverse),
            label = "alpha",
        )
        value
    } else {
        1f
    }
    Canvas(Modifier.size(size)) {
        drawCircle(color = color, alpha = alpha)
    }
}

/** True when the phone's animations are turned off, the Android form of "reduce motion". */
@Composable
fun rememberReduceMotion(): Boolean {
    val resolver = LocalContext.current.contentResolver
    return remember(resolver) {
        Settings.Global.getFloat(resolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f) == 0f
    }
}
