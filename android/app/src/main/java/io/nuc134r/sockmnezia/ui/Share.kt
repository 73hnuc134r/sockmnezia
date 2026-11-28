package io.nuc134r.sockmnezia.ui

import android.content.Intent
import android.graphics.Bitmap
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.FilterQuality
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import com.google.zxing.BarcodeFormat
import com.google.zxing.EncodeHintType
import com.google.zxing.WriterException
import com.google.zxing.qrcode.QRCodeWriter
import com.google.zxing.qrcode.decoder.ErrorCorrectionLevel
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/** A share format the sheet can switch between; [load] returns the text (it may throw a core error). */
class ShareFormat(val label: String, val enabled: Boolean = true, val load: suspend () -> String)

/**
 * Shows shared text (a server link, a chain) with Copy, Send… and QR code. The text holds credentials:
 * the clipboard entry is marked sensitive and the text only leaves through Android's share sheet.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ShareSheet(title: String, formats: List<ShareFormat>, notes: List<String> = emptyList(), onDismiss: () -> Unit) {
    val ctx = LocalContext.current
    val res = LocalResources.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    var selected by remember { mutableStateOf(formats.indexOfFirst { it.enabled }.coerceAtLeast(0)) }
    var text by remember { mutableStateOf<String?>(null) }
    var showQr by remember { mutableStateOf(false) }
    LaunchedEffect(selected) {
        text = null
        scope.act(ctx, snackbar) { text = formats[selected].load() }
    }
    // Always fully open: these sheets have no useful half-open state.
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)) {
        Column(Modifier.padding(start = 16.dp, end = 16.dp, bottom = 24.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Text(title, style = MaterialTheme.typography.titleMedium)
            if (formats.size > 1) {
                SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                    formats.forEachIndexed { i, f ->
                        SegmentedButton(
                            selected = i == selected, enabled = f.enabled, onClick = { selected = i },
                            shape = SegmentedButtonDefaults.itemShape(i, formats.size),
                        ) { Text(f.label) }
                    }
                }
            }
            Box(
                Modifier.fillMaxWidth().heightIn(max = 160.dp)
                    .background(MaterialTheme.colorScheme.surfaceContainerHighest, RoundedCornerShape(10.dp))
                    .padding(10.dp)
                    .verticalScroll(rememberScrollState()),
            ) {
                Text(
                    text ?: "…", fontFamily = FontFamily.Monospace, style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.horizontalScroll(rememberScrollState()),
                )
            }
            Hint(stringResource(R.string.share_warning), warn = true)
            notes.forEach { Hint(it) }
            val t = text
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                FilledTonalButton(enabled = t != null, onClick = {
                    copy(ctx, title, t.orEmpty(), sensitive = true)
                    scope.launch { snackbar.showSnackbar(res.getString(R.string.share_copied), withDismissAction = true) }
                }) { Text(stringResource(R.string.share_copy)) }
                OutlinedButton(enabled = t != null, onClick = {
                    val send = Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, t)
                    ctx.startActivity(Intent.createChooser(send, title))
                }) { Text(stringResource(R.string.share_send)) }
                OutlinedButton(enabled = t != null, onClick = { showQr = true }) { Text(stringResource(R.string.share_qr)) }
            }
        }
    }
    if (showQr) text?.let { QrDialog(title, it) { showQr = false } }
}

/** Share sheet for a saved chain: links (always) or Xray JSON (only without Amnezia). */
@Composable
fun ChainShareSheet(id: String, name: String, withAmnezia: Boolean, onDismiss: () -> Unit) {
    ShareSheet(
        title = stringResource(R.string.share_title, name),
        formats = listOf(
            ShareFormat(stringResource(R.string.share_format_links)) { Core.shareChain(id, "links") },
            ShareFormat(stringResource(R.string.share_format_xray), enabled = !withAmnezia) { Core.shareChain(id, "xray") },
        ),
        notes = listOfNotNull(
            stringResource(R.string.share_links_hint),
            if (withAmnezia) stringResource(R.string.share_amnezia_note) else null,
            if (withAmnezia) stringResource(R.string.share_xray_amnezia) else null,
        ),
        onDismiss = onDismiss,
    )
}

/** Share sheet for everything saved: every server and chain in one text (another sockmnezia imports it all). */
@Composable
fun AllShareSheet(onDismiss: () -> Unit) {
    ShareSheet(
        title = stringResource(R.string.share_all_title),
        formats = listOf(ShareFormat(stringResource(R.string.share_format_links)) { Core.shareAll() }),
        notes = listOf(stringResource(R.string.share_all_hint), stringResource(R.string.share_amnezia_note)),
        onDismiss = onDismiss,
    )
}

/** Share sheet for a saved server: its link. */
@Composable
fun ServerShareSheet(id: String, name: String, onDismiss: () -> Unit) {
    ShareSheet(
        title = stringResource(R.string.share_title, name),
        formats = listOf(ShareFormat(stringResource(R.string.share_format_links)) { Core.shareServer(id) }),
        onDismiss = onDismiss,
    )
}

@Composable
private fun QrDialog(title: String, text: String, onDismiss: () -> Unit) {
    var bitmap by remember(text) { mutableStateOf<Bitmap?>(null) }
    var tooLong by remember(text) { mutableStateOf(false) }
    LaunchedEffect(text) {
        bitmap = withContext(Dispatchers.Default) {
            try {
                qr(text)
            } catch (_: WriterException) {
                null
            } catch (_: IllegalArgumentException) {
                null
            }
        }
        tooLong = bitmap == null
    }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            when {
                tooLong -> Text(stringResource(R.string.share_qr_too_long))
                bitmap != null -> Image(
                    bitmap!!.asImageBitmap(), contentDescription = stringResource(R.string.share_qr),
                    filterQuality = FilterQuality.None,
                    modifier = Modifier.fillMaxWidth().aspectRatio(1f).background(Color.White).padding(8.dp),
                )
            }
        },
        confirmButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.back)) } },
    )
}

/** A black-on-white QR code, one pixel per module (scaled up without smoothing when drawn). */
private fun qr(text: String): Bitmap {
    val hints = mapOf(EncodeHintType.CHARACTER_SET to "UTF-8", EncodeHintType.MARGIN to 1, EncodeHintType.ERROR_CORRECTION to ErrorCorrectionLevel.L)
    val m = QRCodeWriter().encode(text, BarcodeFormat.QR_CODE, 0, 0, hints)
    val px = IntArray(m.width * m.height) { i -> if (m.get(i % m.width, i / m.width)) android.graphics.Color.BLACK else android.graphics.Color.WHITE }
    return Bitmap.createBitmap(px, m.width, m.height, Bitmap.Config.ARGB_8888)
}
