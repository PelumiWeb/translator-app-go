package com.example.ptranslate.ui.theme

import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

/**
 * The "Pearl" palette from the design handoff (docs/design). Two colours carry
 * meaning, and only two: [you] for everything that is the person holding the
 * phone, [them] for the person they talk to. Layers are told apart by colour,
 * not by shadows.
 */
@Immutable
data class PearlColors(
    val bg: Color,
    val bgRaised: Color,
    val bgDeep: Color,
    val fg: Color,
    /** Secondary text, meta, timestamps. */
    val fg2: Color,
    /** Upcoming lines of the passage. */
    val fg3: Color,
    /** Lines of the passage already read. */
    val fg4: Color,
    val you: Color,
    val onYou: Color,
    val youHalo: Color,
    val youRing: Color,
    val them: Color,
    val onThem: Color,
)

private val Petrol = Color(0xFF0E2426)
private val WarmWhite = Color(0xFFF3EEE4)

/** As designed. */
val DarkPearlColors = PearlColors(
    bg = Petrol,
    bgRaised = Color(0xFF173539),
    bgDeep = Color(0xFF071416),
    fg = WarmWhite,
    fg2 = WarmWhite.copy(alpha = 0.64f),
    fg3 = WarmWhite.copy(alpha = 0.44f),
    fg4 = WarmWhite.copy(alpha = 0.32f),
    you = WarmWhite,
    onYou = Petrol,
    youHalo = WarmWhite.copy(alpha = 0.12f),
    youRing = WarmWhite.copy(alpha = 0.28f),
    them = Color(0xFF7FD1C1),
    onThem = Petrol,
)

/**
 * Proposed by the handoff but not yet designed: the dark palette turned inside
 * out, since warm white disappears on a light page. Not shown until light mode
 * is designed (see PtranslateTheme).
 */
val LightPearlColors = PearlColors(
    bg = WarmWhite,
    bgRaised = Color(0xFFDCEFE9),
    bgDeep = Color(0xFFE5DFD3),
    fg = Petrol,
    fg2 = Petrol.copy(alpha = 0.64f),
    fg3 = Petrol.copy(alpha = 0.44f),
    fg4 = Petrol.copy(alpha = 0.32f),
    you = Petrol,
    onYou = WarmWhite,
    youHalo = Petrol.copy(alpha = 0.10f),
    youRing = Petrol.copy(alpha = 0.22f),
    them = Color(0xFF1C6E62),
    onThem = WarmWhite,
)

val LocalPearlColors = staticCompositionLocalOf { DarkPearlColors }
