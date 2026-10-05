package com.example.ptranslate.core.translate

import androidx.test.ext.junit.runners.AndroidJUnit4
import com.example.ptranslate.core.Language
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Runs ML Kit for real. The first run downloads the Spanish language pack
 * (about 30 MB), so the device needs a network connection.
 */
@RunWith(AndroidJUnit4::class)
class MlKitTranslatorTest {

    private val english = Language("en")
    private val spanish = Language("es")

    @Test
    fun listsTheLanguagesItSupports() {
        val tags = MlKitTranslator().supportedLanguages.map { it.tag }

        assertTrue("languages: $tags", tags.containsAll(listOf("en", "es", "fr", "ar", "zh")))
        assertEquals("sorted", tags.sorted(), tags)
    }

    @Test
    fun translatesEnglishToSpanish() = runBlocking {
        val translator = MlKitTranslator()

        translator.prepare(english, spanish)
        assertTrue("ready after prepare", translator.isReady(english, spanish))
        val translated = translator.translate("Good morning, how are you?", english, spanish)
        translator.close()

        assertTrue("translation: $translated", translated.contains("Buenos días", ignoreCase = true))
    }

    @Test
    fun rejectsALanguageMlKitDoesNotHave() {
        val error = assertThrows(TranslationException::class.java) {
            runBlocking { MlKitTranslator().translate("hello", english, Language("yo")) }
        }

        assertEquals("Translation does not support yo", error.message)
    }
}
