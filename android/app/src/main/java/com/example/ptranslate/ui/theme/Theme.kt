package com.example.ptranslate.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.ReadOnlyComposable

/** The app's colours, read from anywhere in a [PtranslateTheme]. */
object Pearl {
    val colors: PearlColors
        @Composable
        @ReadOnlyComposable
        get() = LocalPearlColors.current
}

/**
 * The Pearl design. Only dark is designed, so the app is dark whatever the
 * phone's setting; the light palette is the handoff's proposal, kept for when
 * light mode is designed; nothing asks for it yet. No dynamic colour: the
 * brand's two colours carry meaning (you, them) and must not change with the
 * wallpaper. Material's scheme is mapped onto the palette so standard
 * components (sheets, fields, menus) sit in the same world.
 */
@Composable
fun PtranslateTheme(darkTheme: Boolean = true, content: @Composable () -> Unit) {
    val colors = if (darkTheme) DarkPearlColors else LightPearlColors
    val scheme = if (darkTheme) {
        darkColorScheme(
            primary = colors.them, onPrimary = colors.onThem,
            secondary = colors.you, onSecondary = colors.onYou,
            background = colors.bg, onBackground = colors.fg,
            surface = colors.bg, onSurface = colors.fg,
            surfaceVariant = colors.bgRaised, onSurfaceVariant = colors.fg2,
            surfaceContainerLowest = colors.bg, surfaceContainerLow = colors.bgRaised,
            surfaceContainer = colors.bgRaised, surfaceContainerHigh = colors.bgRaised,
            surfaceContainerHighest = colors.bgRaised,
            outline = colors.fg3, outlineVariant = colors.fg4,
        )
    } else {
        lightColorScheme(
            primary = colors.them, onPrimary = colors.onThem,
            secondary = colors.you, onSecondary = colors.onYou,
            background = colors.bg, onBackground = colors.fg,
            surface = colors.bg, onSurface = colors.fg,
            surfaceVariant = colors.bgRaised, onSurfaceVariant = colors.fg2,
            surfaceContainerLowest = colors.bg, surfaceContainerLow = colors.bgRaised,
            surfaceContainer = colors.bgRaised, surfaceContainerHigh = colors.bgRaised,
            surfaceContainerHighest = colors.bgRaised,
            outline = colors.fg3, outlineVariant = colors.fg4,
        )
    }
    CompositionLocalProvider(LocalPearlColors provides colors) {
        MaterialTheme(colorScheme = scheme, typography = PearlTypography, content = content)
    }
}
