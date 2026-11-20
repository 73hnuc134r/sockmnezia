package io.nuc134r.sockmnezia.ui.screens

import android.graphics.BitmapFactory
import android.net.Uri
import android.util.Base64
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Captcha
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.FetchRequest
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.Section
import io.nuc134r.sockmnezia.ui.act
import io.nuc134r.sockmnezia.ui.paste
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File

@Composable
fun AmneziaScreen() {
    val ctx = LocalContext.current
    val res = LocalResources.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val status by Core.status.collectAsState()
    var key by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf<String?>(null) }
    var captcha by remember { mutableStateOf<Captcha?>(null) }
    var pemDialog by remember { mutableStateOf(false) }

    fun fetch(req: FetchRequest = FetchRequest()) {
        busy = res.getString(R.string.fetching)
        scope.act(ctx, snackbar, ok = res.getString(R.string.config_updated), onError = {
            if (it.code == "captcha_required" || it.code == "captcha_invalid") {
                it.captcha?.let { c -> captcha = c; true } ?: false
            } else false
        }) {
            try { Core.fetch(req) } finally { busy = null }
        }
    }

    val pickApk = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri: Uri? ->
        if (uri == null) return@rememberLauncherForActivityResult
        busy = res.getString(R.string.scanning_apk)
        scope.act(ctx, snackbar) {
            try {
                // Copy into the app's cache: the core reads a file path; the copy is deleted right after.
                val tmp = File(ctx.cacheDir, "official.apk")
                withContext(Dispatchers.IO) {
                    ctx.contentResolver.openInputStream(uri)!!.use { i -> tmp.outputStream().use { o -> i.copyTo(o) } }
                }
                val n = try { Core.importGatewayKeysFromFile(tmp.absolutePath) } finally { tmp.delete() }
                snackbar.showSnackbar(res.getQuantityString(R.plurals.gateway_keys_found, n.toInt(), n.toInt()))
            } finally {
                busy = null
            }
        }
    }

    Column(Modifier.verticalScroll(rememberScrollState()).padding(vertical = 8.dp)) {
        Section(stringResource(R.string.subscription_key)) {
            Hint(stringResource(if (status.hasKey) R.string.key_saved else R.string.key_hint))
            OutlinedTextField(
                value = key, onValueChange = { key = it.trim() },
                label = { Text(stringResource(R.string.subscription_key)) },
                placeholder = { Text("vpn://…") },
                singleLine = true,
                visualTransformation = PasswordVisualTransformation(),
                modifier = Modifier.fillMaxWidth(),
            )
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedButton(onClick = { key = paste(ctx).trim() }) { Text(stringResource(R.string.paste)) }
                Button(enabled = key.isNotBlank(), onClick = {
                    scope.act(ctx, snackbar, ok = res.getString(R.string.key_saved)) {
                        Core.setKey(key)
                        key = ""
                    }
                }) { Text(stringResource(R.string.save)) }
                if (status.hasKey) {
                    TextButton(onClick = { scope.act(ctx, snackbar) { Core.setKey("") } }) { Text(stringResource(R.string.remove)) }
                }
            }
        }

        Section(stringResource(R.string.gateway_key)) {
            Hint(stringResource(if (status.hasGatewayKey) R.string.gateway_key_saved else R.string.gateway_key_hint))
            FilledTonalButton(enabled = busy == null, onClick = {
                pickApk.launch(arrayOf("application/vnd.android.package-archive", "application/octet-stream", "*/*"))
            }) { Text(stringResource(R.string.pick_official_apk)) }
            OutlinedButton(enabled = busy == null, onClick = {
                busy = res.getString(R.string.downloading_apk)
                scope.act(ctx, snackbar) {
                    try {
                        val r = Core.importGatewayKeysFromOfficialRelease()
                        snackbar.showSnackbar(res.getQuantityString(R.plurals.gateway_keys_found_release, r.count, r.count, r.release))
                    } finally { busy = null }
                }
            }) { Text(stringResource(R.string.download_official_apk)) }
            TextButton(onClick = { pemDialog = true }) { Text(stringResource(R.string.paste_pem)) }
        }

        Section(stringResource(R.string.config)) {
            Hint(stringResource(R.string.device_slot_warning))
            if (status.hasConfig) {
                Hint(stringResource(R.string.config_status, status.countryName.ifBlank { status.country }, status.fetchedAt?.take(16)?.replace('T', ' ') ?: "—"))
            }
            Button(enabled = status.hasKey && status.hasGatewayKey && busy == null, onClick = { fetch() }) {
                Text(stringResource(if (status.hasConfig) R.string.refresh_config else R.string.fetch_config))
            }
        }
        busy?.let {
            Row(Modifier.padding(16.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                CircularProgressIndicator(Modifier.height(24.dp))
                Text(it)
            }
        }
    }

    captcha?.let { c ->
        var answer by remember(c.id) { mutableStateOf("") }
        val bmp = remember(c.imageBase64) {
            runCatching {
                val raw = Base64.decode(c.imageBase64.substringAfter("base64,"), Base64.DEFAULT)
                BitmapFactory.decodeByteArray(raw, 0, raw.size)?.asImageBitmap()
            }.getOrNull()
        }
        AlertDialog(
            onDismissRequest = { captcha = null },
            title = { Text(stringResource(R.string.captcha_title)) },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    bmp?.let { Image(it, null, Modifier.fillMaxWidth().height(100.dp)) }
                    c.hint?.takeIf { it.isNotBlank() }?.let { Hint(it) }
                    OutlinedTextField(answer, { answer = it }, singleLine = true, label = { Text(stringResource(R.string.captcha_answer)) })
                }
            },
            confirmButton = {
                TextButton(enabled = answer.isNotBlank(), onClick = {
                    captcha = null
                    fetch(FetchRequest(captchaId = c.id, captchaSolution = answer.trim()))
                }) { Text(stringResource(R.string.ok)) }
            },
            dismissButton = { TextButton(onClick = { captcha = null }) { Text(stringResource(R.string.cancel)) } },
        )
    }

    if (pemDialog) {
        var text by remember { mutableStateOf("") }
        AlertDialog(
            onDismissRequest = { pemDialog = false },
            title = { Text(stringResource(R.string.paste_pem)) },
            text = { OutlinedTextField(text, { text = it }, minLines = 4, modifier = Modifier.fillMaxWidth()) },
            confirmButton = {
                TextButton(onClick = {
                    pemDialog = false
                    scope.act(ctx, snackbar) {
                        val n = Core.setGatewayKeysPem(text)
                        snackbar.showSnackbar(res.getQuantityString(R.plurals.gateway_keys_found, n.toInt(), n.toInt()))
                    }
                }) { Text(stringResource(R.string.save)) }
            },
            dismissButton = { TextButton(onClick = { pemDialog = false }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}
