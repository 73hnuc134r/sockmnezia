package io.nuc134r.sockmnezia.ui

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.os.Build
import android.os.PersistableBundle
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.CoreError
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

val LocalSnackbar = compositionLocalOf { SnackbarHostState() }

/** Translates a core error code (core/engine/errors.go) into a sentence. */
fun errorText(ctx: Context, e: CoreError): String {
    val res = when (e.code) {
        "invalid_key" -> R.string.err_invalid_key
        "no_gateway_key" -> R.string.err_no_gateway_key
        "gateway_wrong_key" -> R.string.err_gateway_wrong_key
        "gateway_unreachable" -> R.string.err_gateway_unreachable
        "device_limit" -> R.string.err_device_limit
        "trial_used" -> R.string.err_trial_used
        "subscription_expired" -> R.string.err_subscription_expired
        "subscription_not_active" -> R.string.err_subscription_not_active
        "captcha_required" -> R.string.err_captcha_required
        "captcha_invalid" -> R.string.err_captcha_invalid
        "rate_limit" -> R.string.err_rate_limit
        "update_required" -> R.string.err_update_required
        "gateway_error" -> R.string.err_gateway_error
        "no_config" -> R.string.err_no_config
        "no_own_server" -> R.string.err_no_own_server
        "port_in_use" -> R.string.err_port_in_use
        "invalid_settings" -> R.string.err_invalid_settings
        "geo_missing" -> R.string.err_geo_missing
        "unsupported_link" -> R.string.err_unsupported_link
        "invalid_link" -> R.string.err_invalid_link
        "not_running" -> R.string.err_not_running
        "network" -> R.string.err_network
        "geo_busy" -> R.string.err_geo_busy
        "no_chain" -> R.string.err_no_chain
        "server_in_use" -> R.string.err_server_in_use
        "chain_empty" -> R.string.err_chain_empty
        "chain_too_long" -> R.string.err_chain_too_long
        "chain_amnezia_twice" -> R.string.err_chain_amnezia_twice
        "hop_not_chainable" -> R.string.err_hop_not_chainable
        else -> R.string.err_internal
    }
    val base = ctx.getString(res)
    return if (e.detail.isNullOrBlank() || e.code in setOf(
            "device_limit", "captcha_required", "not_running", "no_chain", "geo_busy",
            "chain_empty", "chain_too_long", "chain_amnezia_twice", "hop_not_chainable",
        )) base
    else "$base\n${e.detail}"
}

fun formatBytes(b: Long): String = when {
    b < 1024 -> "$b B"
    b < 1024 * 1024 -> "%.1f KiB".format(b / 1024.0)
    b < 1024L * 1024 * 1024 -> "%.1f MiB".format(b / 1048576.0)
    else -> "%.2f GiB".format(b / 1073741824.0)
}

/** Copies text; marks it sensitive so Android 13+ does not show it in the clipboard preview. */
fun copy(ctx: Context, label: String, text: String, sensitive: Boolean = false) {
    val cm = ctx.getSystemService(ClipboardManager::class.java)
    val clip = ClipData.newPlainText(label, text)
    if (sensitive) {
        clip.description.extras = PersistableBundle().apply {
            putBoolean(if (Build.VERSION.SDK_INT >= 33) android.content.ClipDescription.EXTRA_IS_SENSITIVE else "android.content.extra.IS_SENSITIVE", true)
        }
    }
    cm.setPrimaryClip(clip)
}

fun paste(ctx: Context): String =
    ctx.getSystemService(ClipboardManager::class.java).primaryClip?.getItemAt(0)?.coerceToText(ctx)?.toString().orEmpty()

/** Runs a core call, showing the error (translated) or [ok] in a snackbar. */
fun CoroutineScope.act(
    ctx: Context,
    snackbar: SnackbarHostState,
    ok: String? = null,
    onError: (CoreError) -> Boolean = { false },
    block: suspend () -> Unit,
) = launch {
    try {
        block()
        if (ok != null) snackbar.showSnackbar(ok, withDismissAction = true)
    } catch (e: Exception) {
        val ce = Core.asCoreException(e).error
        if (!onError(ce)) snackbar.showSnackbar(errorText(ctx, ce), withDismissAction = true)
    }
}

@Composable
fun Section(title: String? = null, modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    Card(modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp)) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            if (title != null) Text(title, style = MaterialTheme.typography.titleMedium)
            content()
        }
    }
}

@Composable
fun SwitchRow(title: String, checked: Boolean, subtitle: String? = null, enabled: Boolean = true, onChange: (Boolean) -> Unit) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.bodyLarge)
            if (subtitle != null) Text(subtitle, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Spacer(Modifier.width(8.dp))
        Switch(checked = checked, onCheckedChange = onChange, enabled = enabled)
    }
}

@Composable
fun Hint(text: String, warn: Boolean = false, modifier: Modifier = Modifier) {
    Text(
        text,
        modifier = modifier,
        style = MaterialTheme.typography.bodySmall,
        color = if (warn) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurfaceVariant,
    )
}
