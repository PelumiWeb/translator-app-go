package com.example.ptranslate.ui.theme

import androidx.compose.material3.Typography
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import com.example.ptranslate.R

/**
 * IBM Plex Sans, bundled (SIL Open Font License, in assets/licenses). Scripts
 * it does not cover, such as Arabic or Thai, fall back to the system's fonts.
 */
val PlexSans = FontFamily(
    Font(R.font.ibm_plex_sans_regular, FontWeight.Normal),
    Font(R.font.ibm_plex_sans_medium, FontWeight.Medium),
)

private fun style(size: Float, weight: FontWeight, lineHeight: Float? = null, tracking: Float = 0f) = TextStyle(
    fontFamily = PlexSans,
    fontWeight = weight,
    fontSize = size.sp,
    lineHeight = (lineHeight ?: (size * 1.3f)).sp,
    letterSpacing = tracking.em,
)

/**
 * The design's type roles, in sp so they grow with the phone's font size.
 * Text that holds a translation must never be cut short, so screens let it
 * wrap and scroll.
 */
object PearlType {
    /** Timers and durations use tabular figures, so they do not jitter as they count. */
    val timer = style(46f, FontWeight.Medium, lineHeight = 46f, tracking = -0.02f).copy(fontFeatureSettings = "tnum")
    val passageCurrent = style(25f, FontWeight.Medium, lineHeight = 31.25f, tracking = -0.01f)
    val passageOther = style(16f, FontWeight.Normal, lineHeight = 22.4f)
    val translation = style(28f, FontWeight.Medium, lineHeight = 33.6f, tracking = -0.015f)
    val title = style(22f, FontWeight.Medium, lineHeight = 26.4f)
    val headline = style(19f, FontWeight.Medium, lineHeight = 24f)
    val message = style(16f, FontWeight.Normal, lineHeight = 24f)
    val body = style(14f, FontWeight.Normal, lineHeight = 20f)
    val small = style(13f, FontWeight.Normal, lineHeight = 18f)
    val smallStrong = style(13f, FontWeight.Medium, lineHeight = 18f)
    val chip = style(14f, FontWeight.Medium, lineHeight = 18f)
    val label = style(12f, FontWeight.Medium, lineHeight = 16f)
    val meta = style(12f, FontWeight.Normal, lineHeight = 16f).copy(fontFeatureSettings = "tnum")
}

/** Material's roles in Plex, so standard components (fields, lists, sheets) match. */
val PearlTypography = Typography().let { base ->
    Typography(
        displayLarge = base.displayLarge.copy(fontFamily = PlexSans),
        displayMedium = base.displayMedium.copy(fontFamily = PlexSans),
        displaySmall = base.displaySmall.copy(fontFamily = PlexSans),
        headlineLarge = base.headlineLarge.copy(fontFamily = PlexSans),
        headlineMedium = base.headlineMedium.copy(fontFamily = PlexSans),
        headlineSmall = base.headlineSmall.copy(fontFamily = PlexSans),
        titleLarge = PearlType.title,
        titleMedium = base.titleMedium.copy(fontFamily = PlexSans),
        titleSmall = base.titleSmall.copy(fontFamily = PlexSans),
        bodyLarge = PearlType.message,
        bodyMedium = PearlType.body,
        bodySmall = PearlType.small,
        labelLarge = PearlType.chip,
        labelMedium = PearlType.label,
        labelSmall = base.labelSmall.copy(fontFamily = PlexSans),
    )
}
