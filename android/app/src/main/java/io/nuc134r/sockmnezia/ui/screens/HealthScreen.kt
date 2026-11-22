package io.nuc134r.sockmnezia.ui.screens

import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.layout.padding
import androidx.lifecycle.compose.LifecycleResumeEffect
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.health.Check
import io.nuc134r.sockmnezia.health.CheckId
import io.nuc134r.sockmnezia.health.DeviceStateReader
import io.nuc134r.sockmnezia.health.Fix
import io.nuc134r.sockmnezia.health.Severity
import io.nuc134r.sockmnezia.health.evaluate
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.Section
import kotlinx.coroutines.launch

@Composable
fun HealthScreen() {
    val ctx = LocalContext.current
    val uri = LocalUriHandler.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    var checks by remember { mutableStateOf(emptyList<Check>()) }
    LifecycleResumeEffect(Unit) {
        DeviceStateReader.unusedRestrictions(ctx) { u -> checks = evaluate(DeviceStateReader.read(ctx, u)) }
        onPauseOrDispose { }
    }
    val noScreen = stringResource(R.string.health_no_screen)
    LazyColumn(Modifier.fillMaxWidth()) {
        item { Section { Hint(stringResource(R.string.health_intro)) } }
        items(checks.sortedByDescending { it.severity }) { c ->
            Section {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(
                        when (c.severity) {
                            Severity.OK -> Icons.Default.CheckCircle
                            Severity.INFO -> Icons.Default.Info
                            else -> Icons.Default.Warning
                        },
                        null,
                        tint = when (c.severity) {
                            Severity.PROBLEM -> MaterialTheme.colorScheme.error
                            Severity.OK -> MaterialTheme.colorScheme.primary
                            else -> MaterialTheme.colorScheme.onSurfaceVariant
                        },
                    )
                    Text(checkTitle(c), style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(start = 12.dp))
                }
                if (!c.ok) Hint(checkText(c))
                if (!c.ok && c.fix != Fix.NONE) {
                    FilledTonalButton(onClick = {
                        if (!DeviceStateReader.open(ctx, DeviceStateReader.intentFor(ctx, c.fix))) {
                            scope.launch { snackbar.showSnackbar(noScreen, withDismissAction = true) }
                        }
                    }) { Text(stringResource(R.string.health_fix)) }
                }
                if (c.id == CheckId.OEM && !c.ok) {
                    androidx.compose.material3.TextButton(onClick = {
                        uri.openUri("https://dontkillmyapp.com/" + android.os.Build.MANUFACTURER.lowercase())
                    }) { Text("dontkillmyapp.com") }
                }
            }
        }
    }
}

@Composable
private fun checkTitle(c: Check): String = stringResource(
    when (c.id) {
        CheckId.BATTERY_OPTIMIZATION -> if (c.ok) R.string.hc_battery_ok else R.string.hc_battery
        CheckId.BACKGROUND_RESTRICTED -> if (c.ok) R.string.hc_restricted_ok else R.string.hc_restricted
        CheckId.NOTIFICATIONS -> if (c.ok) R.string.hc_notifications_ok else R.string.hc_notifications
        CheckId.DATA_SAVER -> if (c.ok) R.string.hc_datasaver_ok else R.string.hc_datasaver
        CheckId.HIBERNATION -> if (c.ok) R.string.hc_hibernation_ok else R.string.hc_hibernation
        CheckId.POWER_SAVE -> R.string.hc_powersave
        CheckId.OEM -> R.string.hc_oem
        CheckId.BOOT -> R.string.hc_boot
        CheckId.KILLS -> R.string.hc_kills
    },
)

@Composable
private fun checkText(c: Check): String = when (c.id) {
    CheckId.KILLS -> pluralStringResource(R.plurals.hc_kills_text, c.count, c.count)
    else -> stringResource(
        when (c.id) {
            CheckId.BATTERY_OPTIMIZATION -> R.string.hc_battery_text
            CheckId.BACKGROUND_RESTRICTED -> R.string.hc_restricted_text
            CheckId.NOTIFICATIONS -> R.string.hc_notifications_text
            CheckId.DATA_SAVER -> R.string.hc_datasaver_text
            CheckId.HIBERNATION -> R.string.hc_hibernation_text
            CheckId.POWER_SAVE -> R.string.hc_powersave_text
            CheckId.OEM -> R.string.hc_oem_text
            else -> R.string.hc_boot_text
        },
    )
}
