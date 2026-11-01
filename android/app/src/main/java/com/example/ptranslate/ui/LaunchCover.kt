package com.example.ptranslate.ui

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.layout
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.example.ptranslate.ui.components.ShinarMark
import com.example.ptranslate.ui.components.Wordmark
import com.example.ptranslate.ui.components.rememberReduceMotion
import com.example.ptranslate.ui.theme.Pearl
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private val Ease = CubicBezierEasing(0.22f, 0.61f, 0.36f, 1f)
private val MarkWidth = 135.dp

/**
 * Covers the app as it opens, picking up where the splash screen leaves off:
 * the same mark, as wide, in the same place. The first time, on the way to
 * the welcome, it plays the intro: the two voices step apart, each speaks
 * once, they meet again and the name appears. Otherwise it only fades. A tap
 * skips it.
 */
@Composable
fun LaunchCover(playIntro: Boolean, onFinished: () -> Unit) {
    val reduceMotion = rememberReduceMotion()
    val apart = remember { Animatable(0f) }
    val lens = remember { Animatable(1f) }
    val ripple = remember { Animatable(0f) }
    val named = remember { Animatable(0f) }
    val shown = remember { Animatable(1f) }
    var skipped by remember { mutableStateOf(false) }

    LaunchedEffect(skipped) {
        if (playIntro && !skipped) {
            delay(200)
            if (!reduceMotion) {
                coroutineScope {
                    launch { apart.animateTo(9f, tween(300, easing = Ease)) }
                    launch { lens.animateTo(0f, tween(300, easing = Ease)) }
                }
                ripple.animateTo(1f, tween(400, easing = Ease))
                coroutineScope {
                    launch { apart.animateTo(0f, tween(300, easing = Ease)) }
                    launch { lens.animateTo(1f, tween(300, easing = Ease)) }
                }
            }
            named.animateTo(1f, tween(300, easing = Ease))
        }
        shown.animateTo(0f, tween(250))
        onFinished()
    }

    Box(
        Modifier
            .fillMaxSize()
            .graphicsLayer { alpha = shown.value }
            .background(Pearl.colors.bg)
            .pointerInput(Unit) { detectTapGestures { skipped = true } }
            .clearAndSetSemantics {},
        contentAlignment = Alignment.Center,
    ) {
        ShinarMark(Modifier.width(MarkWidth), apart = apart.value, lens = lens.value, ripple = ripple.value)
        Wordmark(
            40.sp,
            Modifier
                // Below the mark, leaving the mark where the splash screen put it.
                .layout { measurable, constraints ->
                    val placeable = measurable.measure(constraints)
                    val top = (MarkWidth * (50f / 108f) / 2 + 16.dp).roundToPx()
                    layout(placeable.width, placeable.height) { placeable.place(0, top + placeable.height / 2) }
                }
                .graphicsLayer {
                    alpha = named.value
                    translationY = (1f - named.value) * 8.dp.toPx()
                },
        )
    }
}
