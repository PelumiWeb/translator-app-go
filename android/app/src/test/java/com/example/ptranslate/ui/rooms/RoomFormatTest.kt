package com.example.ptranslate.ui.rooms

import com.example.ptranslate.core.rooms.Message
import com.example.ptranslate.core.rooms.Rendition
import com.example.ptranslate.core.rooms.RenditionStatus
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import java.time.LocalDate
import java.time.ZoneOffset

class RoomFormatTest {
    private fun message(id: String, createdAt: String, mine: Boolean = false, renditions: List<Rendition> = emptyList()) =
        Message(id = id, roomId = "r", senderId = "s", mine = mine, lang = "es", transcript = "hola", durationMs = 12_400, createdAt = createdAt, renditions = renditions)

    @Test
    fun `a newer copy of a note replaces the older, and notes stay in the order sent`() {
        val first = message("a", "2026-10-31T09:00:00Z")
        val second = message("b", "2026-10-31T09:05:00Z")
        val translated = second.copy(renditions = listOf(Rendition("en", RenditionStatus.READY, "hello")))

        val merged = RoomViewModel.merge(listOf(second, first), listOf(translated))

        assertEquals(listOf("a", "b"), merged.map { it.id })
        assertEquals("hello", merged[1].renditions.single().text)
    }

    @Test
    fun `the room list shows a translation when it is ready, and the length until then`() {
        val ready = message("a", "x", renditions = listOf(Rendition("en", RenditionStatus.READY, "hello")))
        val waiting = message("b", "x", renditions = listOf(Rendition("en", RenditionStatus.SPEAKING, "hello")))
        val mine = message("c", "x", mine = true)

        assertEquals("hello", ready.preview("en"))
        assertEquals("Voice note · 0:12", waiting.preview("en"))
        assertEquals("You: hola", mine.preview("en"))
        assertNull(ready.renditionFor("fr"))
    }

    @Test
    fun `times read as a list shows them`() {
        val today = LocalDate.of(2026, 10, 31)
        assertEquals("Yesterday", roomTime("2026-10-30T20:00:00Z", ZoneOffset.UTC, today))
        assertEquals("12 Oct", roomTime("2026-10-12T20:00:00Z", ZoneOffset.UTC, today))
        assertEquals("", roomTime("not a time", ZoneOffset.UTC, today))
        assertEquals("0:07", clock(7_400))
    }

    @Test
    fun `a scanned code gives an ID only when it is ours`() {
        assertEquals("rosa.lm", handleFromCode("shinar://add/Rosa.LM"))
        assertNull(handleFromCode("https://example.com/rosa"))
        assertNull(handleFromCode("shinar://add/"))
    }
}
