package com.example.ptranslate.ui.rooms

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.rooms.Message
import com.example.ptranslate.core.rooms.Rendition
import com.example.ptranslate.core.rooms.RenditionStatus
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.time.format.TextStyle
import java.time.temporal.ChronoUnit
import java.util.Locale

/** "0:14". */
fun clock(ms: Long): String = "%d:%02d".format(ms / 60_000, ms / 1000 % 60)

/** When something was sent, as a list shows it: the time today, "Yesterday", the weekday this week, else the date. */
fun roomTime(createdAt: String, zone: ZoneId = ZoneId.systemDefault(), today: LocalDate = LocalDate.now(zone)): String {
    val instant = runCatching { Instant.parse(createdAt) }.getOrNull() ?: return ""
    val sent = instant.atZone(zone)
    val days = ChronoUnit.DAYS.between(sent.toLocalDate(), today)
    return when {
        days == 0L -> sent.format(DateTimeFormatter.ofLocalizedTime(FormatStyle.SHORT))
        days == 1L -> "Yesterday"
        days in 2..6 -> sent.dayOfWeek.getDisplayName(TextStyle.SHORT, Locale.getDefault())
        else -> sent.format(DateTimeFormatter.ofPattern("d MMM"))
    }
}

/** The time of day a message was sent, for its meta line. */
fun timeOfDay(createdAt: String, zone: ZoneId = ZoneId.systemDefault()): String =
    runCatching { Instant.parse(createdAt).atZone(zone).format(DateTimeFormatter.ofLocalizedTime(FormatStyle.SHORT)) }.getOrDefault("")

/** What a message became for [language], the user's: theirs to hear. */
fun Message.renditionFor(language: String?): Rendition? = renditions.firstOrNull { it.lang == language }

/** The rendition of the user's own message that the listener gets; in a room of two, there is one. */
val Message.delivery: Rendition? get() = renditions.firstOrNull()

/**
 * The words the user reads for someone else's message, once there are words to read: a voice note's when it can be
 * heard, a text message's as soon as it is translated (its voice comes later, if asked for).
 */
fun Message.wordsFor(language: String?): String? {
    val rendition = renditionFor(language) ?: return null
    val readable = rendition.status == RenditionStatus.READY || (isText && rendition.status == RenditionStatus.SPEAKING)
    return rendition.text.takeIf { readable && it.isNotEmpty() }
}

/** A room's last message, as one line of the list. */
fun Message.preview(myLanguage: String?): String {
    if (mine) return "You: $transcript"
    return wordsFor(myLanguage) ?: if (isText) "Message" else "Voice note · ${clock(durationMs)}"
}

val String.asLanguage: Language get() = Language(this)
