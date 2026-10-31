package com.example.ptranslate.ui.rooms

import android.content.Context
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import com.example.ptranslate.core.account.Profile
import com.example.ptranslate.ui.components.Avatar
import com.example.ptranslate.ui.components.Notice
import com.example.ptranslate.ui.components.PearlIcons
import com.example.ptranslate.ui.components.PearlTextField
import com.example.ptranslate.ui.components.PrimaryButton
import com.example.ptranslate.ui.components.SecondaryButton
import com.example.ptranslate.ui.components.localName
import com.example.ptranslate.ui.components.ownName
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.codescanner.GmsBarcodeScannerOptions
import com.google.mlkit.vision.codescanner.GmsBarcodeScanning
import com.google.zxing.BarcodeFormat
import com.google.zxing.EncodeHintType
import com.google.zxing.qrcode.QRCodeWriter
import kotlinx.coroutines.launch
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlin.coroutines.resume

private enum class AddTab { BY_ID, QR }

/** Adding someone: by their ID, or by scanning their QR code, with the user's own code to show. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AddSomeoneSheet(
    state: RoomsUiState,
    me: Profile,
    onHandleChange: (String) -> Unit,
    onAdd: () -> Unit,
    onShare: () -> Unit,
    onDismiss: () -> Unit,
) {
    val colors = Pearl.colors
    var tab by rememberSaveable { mutableStateOf(AddTab.BY_ID) }
    var scanProblem by remember { mutableStateOf<String?>(null) }
    val context = LocalContext.current
    val scope = rememberCoroutineScope()

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
        containerColor = colors.bg,
    ) {
        Column(Modifier.padding(horizontal = 16.dp).imePadding().navigationBarsPadding(), verticalArrangement = Arrangement.spacedBy(20.dp)) {
            Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                Box(
                    Modifier
                        .align(Alignment.CenterStart)
                        .size(44.dp)
                        .clip(CircleShape)
                        .background(colors.bgRaised)
                        .clickable(role = Role.Button, onClickLabel = "Close", onClick = onDismiss)
                        .semantics { contentDescription = "Close" },
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(PearlIcons.Close, contentDescription = null, tint = colors.fg, modifier = Modifier.size(20.dp))
                }
                Text("Add someone", style = PearlType.headline, color = colors.fg)
            }
            Segmented(
                options = listOf("By ID", "QR code"),
                selected = tab.ordinal,
                onSelect = { tab = AddTab.entries[it] },
            )
            when (tab) {
                AddTab.BY_ID -> ById(state, me, onHandleChange, onAdd)
                AddTab.QR -> MyCode(
                    me = me,
                    problem = scanProblem,
                    onScan = {
                        scope.launch {
                            scanProblem = null
                            when (val result = scanCode(context)) {
                                is ScanResult.Found -> {
                                    tab = AddTab.BY_ID
                                    onHandleChange(result.handle)
                                }
                                ScanResult.NotOurs -> scanProblem = "That code isn't a Timbre ID. Ask them to open Add someone, then QR code."
                                ScanResult.Failed -> scanProblem = "The camera couldn't be opened. You can add them by ID instead."
                                ScanResult.Cancelled -> Unit
                            }
                        }
                    },
                    onShare = onShare,
                )
            }
            Spacer(Modifier.size(8.dp))
        }
    }
}

@Composable
private fun Segmented(options: List<String>, selected: Int, onSelect: (Int) -> Unit) {
    val colors = Pearl.colors
    Row(
        Modifier
            .fillMaxWidth()
            .clip(CircleShape)
            .background(colors.bgRaised)
            .padding(3.dp),
    ) {
        options.forEachIndexed { index, option ->
            val chosen = index == selected
            Box(
                Modifier
                    .weight(1f)
                    .heightIn(min = 36.dp)
                    .clip(CircleShape)
                    .background(if (chosen) colors.you else Color.Transparent)
                    .clickable(role = Role.Tab, onClick = { onSelect(index) })
                    .padding(vertical = 8.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text(option, style = PearlType.chip.copy(fontSize = PearlType.message.fontSize), color = if (chosen) colors.onYou else colors.fg2)
            }
        }
    }
}

@Composable
private fun ById(state: RoomsUiState, me: Profile, onHandleChange: (String) -> Unit, onAdd: () -> Unit) {
    val colors = Pearl.colors
    PearlTextField(
        value = state.lookupHandle,
        onValueChange = onHandleChange,
        label = "Their ID",
        placeholder = "their.id",
        prefix = "@",
        keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.None, autoCorrectEnabled = false, keyboardType = KeyboardType.Ascii),
        trailing = {
            if (state.lookup == Lookup.Searching) CircularProgressIndicator(color = colors.fg2, strokeWidth = 1.5.dp, modifier = Modifier.size(18.dp))
        },
    )
    when (val lookup = state.lookup) {
        is Lookup.Found -> PersonCard(lookup.person, me, opening = state.opening, onAdd = onAdd)
        is Lookup.NotFound -> Notice(lookup.reason)
        Lookup.Idle, Lookup.Searching -> Unit
    }
}

/** Who was found, what each will hear, and the one action. */
@Composable
private fun PersonCard(person: Profile, me: Profile, opening: Boolean, onAdd: () -> Unit) {
    val colors = Pearl.colors
    val first = person.displayName.trim().split(" ").firstOrNull()?.ifBlank { null } ?: "@${person.handle}"
    Column(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .background(colors.bgRaised)
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            Avatar(person.displayName.ifBlank { person.handle }, size = 52.dp, fill = colors.bg)
            Column {
                Text(person.displayName.ifBlank { "@${person.handle}" }, style = PearlType.title.copy(fontSize = PearlType.field.fontSize), color = colors.fg)
                Text(
                    buildAnnotatedString {
                        append("@${person.handle} · speaks ")
                        withStyle(SpanStyle(color = colors.them)) { append(person.lang?.ownName().orEmpty()) }
                    },
                    style = PearlType.body,
                    color = colors.fg2,
                )
            }
        }
        // Pronouns are not known, so the sentence is written around them.
        Text(
            "You'll hear $first in ${me.lang?.localName().orEmpty()}, in their own voice. " +
                "They'll hear you in ${person.lang?.localName().orEmpty()}, in yours.",
            style = PearlType.message,
            color = colors.fg2,
        )
        PrimaryButton(if (opening) "Opening…" else "Add $first", onClick = onAdd, enabled = !opening)
    }
}

@Composable
private fun MyCode(me: Profile, problem: String?, onScan: () -> Unit, onShare: () -> Unit) {
    val colors = Pearl.colors
    Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Box(
            Modifier
                .size(236.dp)
                .clip(RoundedCornerShape(20.dp))
                .background(colors.you)
                .padding(18.dp)
                .semantics { contentDescription = "Your QR code, for @${me.handle}" },
        ) {
            QrCode(addLink(me.handle), color = colors.onYou)
        }
        Text(me.displayName, style = PearlType.title, color = colors.fg, modifier = Modifier.padding(top = 16.dp))
        Text("@${me.handle} · speaks ${me.lang?.localName().orEmpty()}", style = PearlType.message, color = colors.fg2)
        Text(
            "Let them scan this with their phone's camera, or the app.",
            style = PearlType.message,
            color = colors.fg2,
            textAlign = TextAlign.Center,
            modifier = Modifier.padding(top = 8.dp),
        )
        problem?.let { Notice(it, Modifier.padding(top = 4.dp)) }
        PrimaryButton("Scan their code", onClick = onScan, modifier = Modifier.padding(top = 16.dp))
        SecondaryButton("Share my code", onClick = onShare, modifier = Modifier.padding(top = 4.dp))
    }
}

/** A QR code of [content], drawn module by module in [color]: dark on the light tile, as scanners expect. */
@Composable
private fun QrCode(content: String, color: Color) {
    val matrix = remember(content) {
        QRCodeWriter().encode(content, BarcodeFormat.QR_CODE, 0, 0, mapOf(EncodeHintType.MARGIN to 0))
    }
    Canvas(Modifier.fillMaxWidth().then(Modifier.size(200.dp))) {
        val cell = size.minDimension / matrix.width
        for (y in 0 until matrix.height) {
            for (x in 0 until matrix.width) {
                if (matrix[x, y]) drawRect(color, topLeft = Offset(x * cell, y * cell), size = Size(cell + 0.5f, cell + 0.5f))
            }
        }
    }
}

private sealed interface ScanResult {
    data class Found(val handle: String) : ScanResult
    data object NotOurs : ScanResult
    data object Cancelled : ScanResult
    data object Failed : ScanResult
}

/**
 * Scans a code with Google's scanner, which draws its own camera screen
 * through Play services: the app itself needs no camera permission.
 */
private suspend fun scanCode(context: Context): ScanResult = suspendCancellableCoroutine { continuation ->
    val options = GmsBarcodeScannerOptions.Builder().setBarcodeFormats(Barcode.FORMAT_QR_CODE).build()
    GmsBarcodeScanning.getClient(context, options).startScan()
        .addOnSuccessListener { barcode ->
            val handle = handleFromCode(barcode.rawValue.orEmpty())
            continuation.resume(if (handle != null) ScanResult.Found(handle) else ScanResult.NotOurs)
        }
        .addOnCanceledListener { continuation.resume(ScanResult.Cancelled) }
        .addOnFailureListener { continuation.resume(ScanResult.Failed) }
}

/** The ID in a scanned code or opened link: timbre://add/<id>. Anything else is not ours. */
fun handleFromCode(raw: String): String? {
    val prefix = "timbre://add/"
    if (!raw.startsWith(prefix, ignoreCase = true)) return null
    return raw.substring(prefix.length).trim('/').lowercase().takeIf { it.isNotEmpty() }
}
