package io.nuc134r.sockmnezia.ui.screens

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.RadioButton
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.ui.Alignment
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.LifecycleResumeEffect
import androidx.navigation.NavController
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.Country
import io.nuc134r.sockmnezia.core.FetchRequest
import io.nuc134r.sockmnezia.health.DeviceStateReader
import io.nuc134r.sockmnezia.health.Severity
import io.nuc134r.sockmnezia.health.evaluate
import io.nuc134r.sockmnezia.health.worst
import io.nuc134r.sockmnezia.ui.ChainStepper
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.Routes
import io.nuc134r.sockmnezia.ui.Section
import io.nuc134r.sockmnezia.ui.act
import io.nuc134r.sockmnezia.ui.errorText
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.delay
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withPermit

@Composable
fun HomeScreen(nav: NavController) {
    val ctx = LocalContext.current
    val res = LocalResources.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val status by Core.status.collectAsState()
    val revision by Core.revision.collectAsState()
    var countries by remember { mutableStateOf(emptyList<Country>()) }
    var health by remember { mutableStateOf(Severity.OK) }
    var busy by remember { mutableStateOf(false) }

    LaunchedEffect(revision) {
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
    Column(Modifier.verticalScroll(rememberScrollState()).padding(vertical = 8.dp)) {
        status.lastError?.takeIf { !status.running }?.let {
            Section { Hint(errorText(ctx, it), warn = true) }
        }
        if (status.authFailures > 0) {
            Section {
                Hint(pluralStringResource(R.plurals.auth_failures, status.authFailures.toInt(), status.authFailures.toInt()), warn = true)
            }
        }

        Section {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    status.chainName ?: stringResource(R.string.chain),
                    style = MaterialTheme.typography.titleMedium, modifier = Modifier.weight(1f),
                )
                OutlinedButton(onClick = { nav.navigate(Routes.SELECT) }) { Text(stringResource(R.string.chain_switch)) }
                status.chainId?.let { id ->
                    TextButton(onClick = { nav.navigate(Routes.chain(id)) }) { Text(stringResource(R.string.chain_edit)) }
                }
            }
            if (status.chainId == null && status.serverId == null) {
                Hint(stringResource(R.string.chain_none))
                FilledTonalButton(onClick = { nav.navigate(Routes.chain(Routes.NEW)) }) { Text(stringResource(R.string.chain_new)) }
            } else {
                ChainStepper(status.hops)
                status.chainError?.let { Hint(errorText(ctx, it), warn = true) }
            }
        }

        if (status.amneziaHop > 0) {
            Section(stringResource(R.string.amnezia)) {
                if (status.hops.size > 1) Hint(stringResource(R.string.hop_of, status.amneziaHop, status.hops.size))
                if (!status.hasKey || !status.hasGatewayKey) {
                    Hint(stringResource(R.string.amnezia_not_set_up))
                    FilledTonalButton(onClick = { nav.navigate(Routes.AMNEZIA) }) { Text(stringResource(R.string.amnezia_set_up)) }
                } else {
                    var showPingDialog by remember { mutableStateOf(false) }
                    val onPick: (String) -> Unit = { code ->
                        busy = true
                        scope.act(ctx, snackbar, ok = res.getString(R.string.config_updated)) {
                            try { Core.fetch(FetchRequest(country = code)) } finally { busy = false }
                        }
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        CountryPicker(status.country, status.countryName, countries, enabled = !busy, onPick = onPick)
                        OutlinedButton(enabled = !busy && countries.isNotEmpty(), onClick = { showPingDialog = true }) {
                            Text(stringResource(R.string.location_find_fastest))
                        }
                    }
                    if (showPingDialog) {
                        LocationPingDialog(
                            countries = countries,
                            onPick = { code -> showPingDialog = false; onPick(code) },
                            onDismiss = { showPingDialog = false },
                        )
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
        if (Core.update.collectAsState().value?.newer == true) {
            Section(stringResource(R.string.updates)) { UpdateCard() }
        }
        if (health != Severity.OK) {
            Section(stringResource(R.string.title_health)) {
                Hint(stringResource(R.string.health_summary_bad), warn = health == Severity.PROBLEM)
                FilledTonalButton(onClick = { nav.navigate(Routes.HEALTH) }) { Text(stringResource(R.string.health_open)) }
            }
        }
        // Room for the switch and start/stop buttons that stick out above the connection bar.
        Spacer(Modifier.height(32.dp))
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
            countries.sortedBy { it.name }.forEach { c ->
                DropdownMenuItem(text = { Text("${c.name} (${c.code.uppercase()})") }, onClick = {
                    open = false
                    onPick(c.code)
                })
            }
        }
    }
}

/** One location's ping measurement: up to 3 attempts, the lowest successful ms kept so far. */
private data class LocationPing(val bestMs: Long? = null, val attempts: Int = 0) {
    val failed get() = attempts >= PingAttempts && bestMs == null
}

private const val PingAttempts = 3
private const val PingConcurrency = 4

/** Pings every available location (up to [PingAttempts] times each, [PingConcurrency] at once) and lists them
 * sorted by the lowest ping found so far, updating live as attempts complete. Tapping a location with a
 * successful ping applies it and closes the dialog. */
@Composable
private fun LocationPingDialog(countries: List<Country>, onPick: (String) -> Unit, onDismiss: () -> Unit) {
    val results = remember { mutableStateMapOf<String, LocationPing>() }
    LaunchedEffect(countries) {
        val semaphore = Semaphore(PingConcurrency)
        countries.map { c ->
            async {
                semaphore.withPermit {
                    var best: Long? = null
                    repeat(PingAttempts) { attempt ->
                        val ms = runCatching { Core.pingCountry(c.code) }.getOrNull()
                        if (ms != null) best = best?.let { minOf(it, ms) } ?: ms
                        results[c.code] = LocationPing(best, attempt + 1)
                    }
                }
            }
        }.awaitAll()
    }
    AlertDialog(
        onDismissRequest = onDismiss,
        confirmButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.back)) } },
        title = { Text(stringResource(R.string.location_fastest_title)) },
        text = {
            val sorted = countries.sortedWith(
                compareBy({ results[it.code]?.bestMs == null }, { results[it.code]?.bestMs ?: Long.MAX_VALUE }),
            )
            LazyColumn {
                items(sorted, key = { it.code }) { c ->
                    val r = results[c.code] ?: LocationPing()
                    Row(
                        Modifier.fillMaxWidth().clickable(enabled = r.bestMs != null) { onPick(c.code) }.padding(vertical = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text("${c.name} (${c.code.uppercase()})", Modifier.weight(1f))
                        Text(
                            when {
                                r.bestMs != null -> stringResource(R.string.conn_ping, r.bestMs.toInt())
                                r.failed -> stringResource(R.string.location_ping_failed)
                                else -> stringResource(R.string.location_ping_testing)
                            },
                            style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            }
        },
    )
}
