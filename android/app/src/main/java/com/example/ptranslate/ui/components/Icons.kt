package com.example.ptranslate.ui.components

import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.addPathNodes
import androidx.compose.ui.unit.dp

/**
 * The few icons the design uses, as outlines on a 24-unit grid with a 1.75
 * stroke. Drawn from Lucide's shapes (lucide.dev, ISC licence) rather than
 * adding a Material icon library for six glyphs. Icon's tint recolours them.
 */
object PearlIcons {
    val Mic = outline(
        "M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z",
        "M19 10v2a7 7 0 0 1-14 0v-2",
        "M12 19v3",
    )
    val Speaker = outline(
        "M11 4.7a.7.7 0 0 0-1.2-.5L6.4 7.6A1.4 1.4 0 0 1 5.4 8H3a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2.4a1.4 1.4 0 0 1 1 .4l3.4 3.4a.7.7 0 0 0 1.2-.5Z",
        "M16 9a5 5 0 0 1 0 6",
        "M19.4 18.4a9 9 0 0 0 0-12.8",
    )
    val Swap = outline("M8 3 4 7l4 4", "M4 7h16", "m16 21 4-4-4-4", "M20 17H4")
    val Back = outline("m15 18-6-6 6-6")
    val Settings = outline(
        "M20 7h-9",
        "M14 17H5",
        "M17 14a3 3 0 1 0 0 6a3 3 0 1 0 0-6Z",
        "M7 4a3 3 0 1 0 0 6a3 3 0 1 0 0-6Z",
    )
    val Search = outline("M11 3a8 8 0 1 0 0 16a8 8 0 1 0 0-16Z", "m21 21-4.3-4.3")
    val Check = outline("M20 6 9 17l-5-5")
    val Info = outline("M12 2a10 10 0 1 0 0 20a10 10 0 1 0 0-20Z", "M12 16v-4", "M12 8h.01")

    /** Filled, as the design's play button. */
    val Play = ImageVector.Builder("Play", 24.dp, 24.dp, 24f, 24f)
        .addPath(addPathNodes("M7 4.5v15a1 1 0 0 0 1.5.86l12.5-7.5a1 1 0 0 0 0-1.72L8.5 3.64A1 1 0 0 0 7 4.5Z"), fill = SolidColor(androidx.compose.ui.graphics.Color.Black))
        .build()

    private fun outline(vararg paths: String): ImageVector {
        val builder = ImageVector.Builder(defaultWidth = 24.dp, defaultHeight = 24.dp, viewportWidth = 24f, viewportHeight = 24f)
        for (path in paths) {
            builder.addPath(
                pathData = addPathNodes(path),
                stroke = SolidColor(androidx.compose.ui.graphics.Color.Black),
                strokeLineWidth = 1.75f,
                strokeLineCap = StrokeCap.Round,
                strokeLineJoin = StrokeJoin.Round,
            )
        }
        return builder.build()
    }
}
