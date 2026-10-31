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
    val Alert = outline("M12 2a10 10 0 1 0 0 20a10 10 0 1 0 0-20Z", "M12 8v4", "M12 16h.01")
    val Plus = outline("M5 12h14", "M12 5v14")
    val Close = outline("M18 6 6 18", "m6 6 12 12")
    val ChevronRight = outline("m9 18 6-6-6-6")
    val Rooms = outline("M7.9 20A9 9 0 1 0 4 16.1L2 22Z")
    val Person = outline("M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2", "M12 3a4 4 0 1 0 0 8a4 4 0 1 0 0-8Z")
    val Phone = outline("M7 2h10a2 2 0 0 1 2 2v16a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2Z", "M12 18h.01")
    val Scan = outline("M3 7V5a2 2 0 0 1 2-2h2", "M17 3h2a2 2 0 0 1 2 2v2", "M21 17v2a2 2 0 0 1-2 2h-2", "M7 21H5a2 2 0 0 1-2-2v-2")
    val MicOff = outline(
        "m2 2 20 20",
        "M18.9 13.4A7 7 0 0 0 19 12v-2",
        "M5 10v2a7 7 0 0 0 12 5",
        "M15 9.3V5a3 3 0 0 0-5.7-1.3",
        "M9 9v3a3 3 0 0 0 5.1 2.1",
        "M12 19v3",
    )
    val Ear = outline("M6 8.5a6.5 6.5 0 1 1 13 0c0 6-6 6-6 10a3.5 3.5 0 1 1-7 0", "M15 8.5a2.5 2.5 0 0 0-5 0v1a2 2 0 1 1 0 4")
    val Retry = outline("M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8", "M3 3v5h5")
    val SpeakerOff = outline(
        "M11 4.7a.7.7 0 0 0-1.2-.5L6.4 7.6A1.4 1.4 0 0 1 5.4 8H3a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2.4a1.4 1.4 0 0 1 1 .4l3.4 3.4a.7.7 0 0 0 1.2-.5Z",
        "m22 9-6 6",
        "m16 9 6 6",
    )

    /** Filled, as the design's pause button. */
    val Pause = ImageVector.Builder("Pause", 24.dp, 24.dp, 24f, 24f)
        .addPath(addPathNodes("M7 4h3.5v16H7Z"), fill = SolidColor(androidx.compose.ui.graphics.Color.Black))
        .addPath(addPathNodes("M13.5 4H17v16h-3.5Z"), fill = SolidColor(androidx.compose.ui.graphics.Color.Black))
        .build()

    /** Filled, the stop square inside the record button. */
    val Stop = ImageVector.Builder("Stop", 24.dp, 24.dp, 24f, 24f)
        .addPath(addPathNodes("M6 4h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2Z"), fill = SolidColor(androidx.compose.ui.graphics.Color.Black))
        .build()

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
