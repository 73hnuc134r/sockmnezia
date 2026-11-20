package io.nuc134r.sockmnezia.ui.screens

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LifecycleResumeEffect
import androidx.navigation.NavController
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.Country
import io.nuc134r.sockmnezia.core.FetchRequest
import io.nuc134r.sockmnezia.core.Inbound
import io.nuc134r.sockmnezia.core.Modes
import io.nuc134r.sockmnezia.health.DeviceStateReader
import io.nuc134r.sockmnezia.health.Severity
import io.nuc134r.sockmnezia.health.evaluate
import io.nuc134r.sockmnezia.health.worst
import io.nuc134r.sockmnezia.service.ProxyService
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.Routes
import io.nuc134r.sockmnezia.ui.Section
import io.nuc134r.sockmnezia.ui.act
import io.nuc134r.sockmnezia.ui.copy
import io.nuc134r.sockmnezia.ui.errorText
import io.nuc134r.sockmnezia.ui.formatBytes
import kotlinx.coroutines.delay

@Composable
fun HomeScreen(nav: NavController) {
    val ctx = LocalContext.current
    val res = LocalResources.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val status by Core.status.collectAsState()
    val revision by Core.revision.collectAsState()
    var inbound by remember { mutableStateOf(Inbound()) }
    var countries by remember { mutableStateOf(emptyList<Country>()) }
    var health by remember { mutableStateOf(Severity.OK) }
    var check by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }

    LaunchedEffect(revision) {
        runCatching { inbound = Core.settings().inbound }
        runCatching { countries = Core.countries() }
    }
    LaunchedEffect(Unit) {
        val prefs = io.nuc134r.sockmnezia.AppPrefs(ctx)
        if (prefs.checkUpdates && Core.update.value == null) {
            runCatching { Core.checkUpdate(io.nuc134r.sockmnezia.BuildConfig.VERSION_NAME, prefs.betaUpdates) }
        }
    }
    LaunchedEffect(Unit) {
        while (true) {
            Core.refreshStatus()
            delay(3000)
        }
    }
    LifecycleResumeEffect(Unit) {
        DeviceStateReader.unusedRestrictions(ctx) { u -> health = evaluate(DeviceStateReader.read(ctx, u)).worst() }
        onPauseOrDispose { }
    }
    val notifPermission = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) {
        ProxyService.start(ctx)
    }

    Column(Modifier.verticalScroll(rememberScrollState()).padding(vertical = 8.dp)) {
        Section {
            Text(
                stringResource(if (status.running) R.string.state_running else R.string.state_stopped),
                style = MaterialTheme.typography.headlineSmall,
            )
            status.lastError?.takeIf { !status.running }?.let { Hint(errorText(ctx, it), warn = true) }
            Button(
                onClick = {
                    if (status.running) {
                        ProxyService.stop(ctx)
                    } else if (Build.VERSION.SDK_INT >= 33 &&
                        ContextCompat.checkSelfPermission(ctx, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
                    ) {
                        notifPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
                    } else {
                        ProxyService.start(ctx)
                    }
                },
                modifier = Modifier.fillMaxWidth().height(56.dp),
            ) { Text(stringResource(if (status.running) R.string.action_stop else R.string.action_start)) }
            if (status.running) {
                Hint(stringResource(R.string.traffic, formatBytes(status.up), formatBytes(status.down)))
            }
            if (status.authFailures > 0) {
                Hint(pluralStringResource(R.plurals.auth_failures, status.authFailures.toInt(), status.authFailures.toInt()), warn = true)
            }
        }

        Section(stringResource(R.string.mode)) {
            val modes = listOf(Modes.AMNEZIA to R.string.mode_amnezia, Modes.AMNEZIA_OWN to R.string.mode_amnezia_own, Modes.OWN to R.string.mode_own)
            SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                modes.forEachIndexed { i, (m, label) ->
                    SegmentedButton(
                        selected = status.mode == m,
                        onClick = { scope.act(ctx, snackbar) { Core.setSettings(Core.settings().copy(mode = m)) } },
                        shape = SegmentedButtonDefaults.itemShape(i, modes.size),
                    ) { Text(stringResource(label), maxLines = 1) }
                }
            }
            Hint(
                stringResource(
                    when (status.mode) {
                        Modes.AMNEZIA_OWN -> R.string.mode_amnezia_own_hint
                        Modes.OWN -> R.string.mode_own_hint
                        else -> R.string.mode_amnezia_hint
                    },
                ),
            )
        }

        if (status.mode != Modes.OWN) {
            Section(stringResource(R.string.amnezia)) {
                if (!status.hasKey || !status.hasGatewayKey) {
                    Hint(stringResource(R.string.amnezia_not_set_up))
                    FilledTonalButton(onClick = { nav.navigate(Routes.AMNEZIA) }) { Text(stringResource(R.string.amnezia_set_up)) }
                } else {
                    CountryPicker(status.country, status.countryName, countries, enabled = !busy) { code ->
                        busy = true
                        scope.act(ctx, snackbar, ok = res.getString(R.string.config_updated)) {
                            try { Core.fetch(FetchRequest(country = code)) } finally { busy = false }
                        }
                    }
                    status.subscriptionTo?.takeIf { it.isNotBlank() }?.let { Hint(stringResource(R.string.subscription_until, it.take(10))) }
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedButton(enabled = !busy, onClick = {
                            busy = true
                            scope.act(ctx, snackbar, ok = res.getString(R.string.config_updated)) {
                                try { Core.fetch() } finally { busy = false }
                            }
                        }) { Text(stringResource(R.string.refresh_config)) }
                        TextButton(onClick = { nav.navigate(Routes.AMNEZIA) }) { Text(stringResource(R.string.amnezia_settings)) }
                    }
                }
            }
        }
        if (status.mode != Modes.AMNEZIA) {
            Section(stringResource(R.string.own_server)) {
                Text(status.ownServer ?: stringResource(R.string.own_none))
                FilledTonalButton(onClick = { nav.navigate(Routes.SERVERS) }) { Text(stringResource(R.string.own_choose)) }
            }
        }

        Section(stringResource(R.string.proxy_for_apps)) {
            val addr = "${inbound.listen}:${inbound.socksPort}"
            KeyValue(stringResource(R.string.socks5), addr) { copy(ctx, "address", addr) }
            KeyValue(stringResource(R.string.username), inbound.user) { copy(ctx, "user", inbound.user) }
            KeyValue(stringResource(R.string.password), "••••••••") { copy(ctx, "password", inbound.pass, sensitive = true) }
            if (inbound.httpPort != 0) {
                val h = "${inbound.listen}:${inbound.httpPort}"
                KeyValue(stringResource(R.string.http_proxy), h) { copy(ctx, "address", h) }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedButton(onClick = {
                    copy(ctx, "proxy", "socks5://${inbound.user}:${inbound.pass}@${inbound.listen}:${inbound.socksPort}", sensitive = true)
                }) { Text(stringResource(R.string.copy_link)) }
                TextButton(onClick = { nav.navigate(Routes.SETTINGS) }) { Text(stringResource(R.string.edit)) }
            }
            Hint(stringResource(if (inbound.udp) R.string.udp_on_hint else R.string.udp_off_hint))
        }

        Section(stringResource(R.string.check)) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedButton(enabled = status.running, onClick = {
                    check = null
                    scope.act(ctx, snackbar) { check = res.getString(R.string.exit_ip, Core.exitIp()) }
                }) { Text(stringResource(R.string.check_exit_ip)) }
                OutlinedButton(onClick = {
                    check = null
                    scope.act(ctx, snackbar) {
                        val r = Core.testChain()
                        check = res.getString(R.string.chain_ok, r.ip, r.ms)
                    }
                }) { Text(stringResource(R.string.test_chain)) }
            }
            check?.let { Text(it, fontFamily = FontFamily.Monospace) }
            Hint(stringResource(R.string.check_hint))
        }

        if (Core.update.collectAsState().value?.newer == true) {
            Section(stringResource(R.string.updates)) { UpdateCard() }
        }
        if (health != Severity.OK) {
            Section(stringResource(R.string.title_health)) {
                Hint(stringResource(R.string.health_summary_bad), warn = health == Severity.PROBLEM)
                FilledTonalButton(onClick = { nav.navigate(Routes.HEALTH) }) { Text(stringResource(R.string.health_open)) }
            }
        }
    }
}

@Composable
private fun KeyValue(key: String, value: String, onCopy: () -> Unit) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = androidx.compose.ui.Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) {
            Text(key, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Text(value, fontFamily = FontFamily.Monospace)
        }
        TextButton(onClick = onCopy) { Text(stringResource(R.string.copy)) }
    }
}

@Composable
private fun CountryPicker(code: String, name: String, countries: List<Country>, enabled: Boolean, onPick: (String) -> Unit) {
    var open by remember { mutableStateOf(false) }
    Box {
        OutlinedButton(enabled = enabled && countries.isNotEmpty(), onClick = { open = true }) {
            Text(if (code.isBlank()) stringResource(R.string.location_default) else "${name.ifBlank { code }} (${code.uppercase()})")
        }
        DropdownMenu(open, onDismissRequest = { open = false }) {
            countries.forEach { c ->
                DropdownMenuItem(text = { Text("${c.name} (${c.code.uppercase()})") }, onClick = {
                    open = false
                    onPick(c.code)
                })
            }
        }
    }
}
