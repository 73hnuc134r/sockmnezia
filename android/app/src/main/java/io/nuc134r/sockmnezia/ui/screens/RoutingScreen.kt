package io.nuc134r.sockmnezia.ui.screens

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.AssistChip
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Switch
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.GeoProgress
import io.nuc134r.sockmnezia.core.GeoStatus
import io.nuc134r.sockmnezia.core.Rule
import io.nuc134r.sockmnezia.core.Settings
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.Routes
import io.nuc134r.sockmnezia.ui.Section
import io.nuc134r.sockmnezia.ui.SwitchRow
import io.nuc134r.sockmnezia.ui.act
import io.nuc134r.sockmnezia.ui.copy
import io.nuc134r.sockmnezia.ui.formatBytes
import io.nuc134r.sockmnezia.ui.paste
import kotlinx.coroutines.delay
import java.text.DateFormat
import java.util.Date

@Composable
fun outboundLabel(tag: String) = stringResource(
    when (tag) {
        "direct" -> R.string.out_direct
        "block" -> R.string.out_block
        else -> R.string.out_proxy
    },
)

fun ruleSummary(r: Rule): String = listOfNotNull(
    r.domain?.takeIf { it.isNotEmpty() }?.joinToString(", "),
    r.ip?.takeIf { it.isNotEmpty() }?.joinToString(", "),
    r.port?.takeIf { it.isNotBlank() }?.let { "port $it" },
    r.network?.takeIf { it.isNotBlank() },
    r.protocol?.takeIf { it.isNotEmpty() }?.joinToString(", "),
).joinToString(" · ")

@Composable
fun RoutingScreen(nav: NavController) {
    val ctx = LocalContext.current
    val res = LocalResources.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val revision by Core.revision.collectAsState()
    var settings by remember { mutableStateOf<Settings?>(null) }
    var geo by remember { mutableStateOf(GeoStatus()) }
    var presetFor by remember { mutableStateOf<String?>(null) }
    var geoBusy by remember { mutableStateOf(false) }
    var geoProgress by remember { mutableStateOf<GeoProgress?>(null) }
    LaunchedEffect(Unit) { // fast while an update runs (from here or the weekly job), slow otherwise
        while (true) {
            geoProgress = runCatching { Core.geoProgress() }.getOrNull()
            delay(if (geoBusy || geoProgress != null) 250 else 2000)
        }
    }
    LaunchedEffect(revision) {
        runCatching { settings = Core.settings() }
        runCatching { geo = Core.geoStatus() }
    }
    val s = settings ?: return
    val rules = s.routing.rules

    fun save(newRules: List<Rule> = rules, strategy: String = s.routing.domainStrategy, opts: io.nuc134r.sockmnezia.core.Options = s.options) {
        val ns = s.copy(routing = s.routing.copy(rules = newRules, domainStrategy = strategy), options = opts)
        settings = ns
        scope.act(ctx, snackbar) { Core.setSettings(ns) }
    }

    LazyColumn(Modifier.fillMaxWidth()) {
        item {
            Section {
                Hint(stringResource(R.string.routing_hint))
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = { nav.navigate(Routes.rule(-1)) }) { Text(stringResource(R.string.add_rule)) }
                    PresetMenu { id -> presetFor = id }
                    MoreMenu(
                        onImport = {
                            scope.act(ctx, snackbar, ok = res.getString(R.string.rules_imported)) {
                                val r = Core.parseRouting(paste(ctx))
                                save(r.rules, r.domainStrategy)
                            }
                        },
                        onExport = {
                            copy(ctx, "rules", Core.exportRules(rules))
                            scope.act(ctx, snackbar, ok = res.getString(R.string.rules_copied)) {}
                        },
                    )
                }
            }
        }
        item {
            Section(stringResource(R.string.locked_rules)) {
                Core.lockedRules().forEach { r ->
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Icon(Icons.Default.Lock, null)
                        Column(Modifier.padding(start = 8.dp).weight(1f)) {
                            Text(stringResource(if (r.ip != null) R.string.locked_ip else R.string.locked_domain))
                            Text(ruleSummary(r), style = MaterialTheme.typography.bodySmall, maxLines = 2, overflow = TextOverflow.Ellipsis)
                        }
                        Text(outboundLabel(r.outboundTag))
                    }
                }
            }
        }
        itemsIndexed(rules) { i, r ->
            Section(modifier = Modifier.clickable { nav.navigate(Routes.rule(i)) }) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text(r.remarks?.ifBlank { null } ?: stringResource(R.string.rule_n, i + 1), style = MaterialTheme.typography.titleSmall)
                        Text(ruleSummary(r), style = MaterialTheme.typography.bodySmall, maxLines = 3, overflow = TextOverflow.Ellipsis)
                        AssistChip(onClick = { nav.navigate(Routes.rule(i)) }, label = { Text(outboundLabel(r.outboundTag)) })
                    }
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Switch(checked = r.isEnabled, onCheckedChange = { on ->
                            save(rules.toMutableList().also { it[i] = r.copy(enabled = on) })
                        })
                        Row {
                            IconButton(enabled = i > 0, onClick = {
                                save(rules.toMutableList().also { it.add(i - 1, it.removeAt(i)) })
                            }) { Icon(Icons.Default.KeyboardArrowUp, stringResource(R.string.move_up)) }
                            IconButton(enabled = i < rules.size - 1, onClick = {
                                save(rules.toMutableList().also { it.add(i + 1, it.removeAt(i)) })
                            }) { Icon(Icons.Default.KeyboardArrowDown, stringResource(R.string.move_down)) }
                            IconButton(onClick = { save(rules.toMutableList().also { it.removeAt(i) }) }) {
                                Icon(Icons.Default.Delete, stringResource(R.string.delete))
                            }
                        }
                    }
                }
            }
        }
        item {
            Section(stringResource(R.string.default_route)) {
                Hint(stringResource(R.string.default_route_hint))
            }
        }
        item {
            Section(stringResource(R.string.domain_strategy)) {
                Choice(
                    listOf(
                        "AsIs" to stringResource(R.string.ds_asis),
                        "IPIfNonMatch" to stringResource(R.string.ds_ipifnonmatch),
                        "IPOnDemand" to stringResource(R.string.ds_ipondemand),
                    ),
                    s.routing.domainStrategy,
                ) { save(strategy = it) }
                Hint(stringResource(R.string.domain_strategy_hint), warn = s.routing.domainStrategy != "AsIs")
            }
        }
        item { RouteTester() }
        item {
            Section(stringResource(R.string.geo_files)) {
                val sources = remember { Core.geoSources() }
                Choice(sources.map { it.id to it.name } + ("custom" to res.getString(R.string.geo_custom)), s.options.geoSource) {
                    save(opts = s.options.copy(geoSource = it))
                }
                if (s.options.geoSource == "custom") {
                    var url by remember { mutableStateOf(s.options.geoCustomUrl) }
                    OutlinedTextField(url, { url = it }, label = { Text("https://…") }, singleLine = true, modifier = Modifier.fillMaxWidth())
                    TextButton(onClick = { save(opts = s.options.copy(geoCustomUrl = url.trim())) }) { Text(stringResource(R.string.save)) }
                }
                Hint(
                    if (geo.hasGeoIp && geo.hasGeoSite) {
                        stringResource(R.string.geo_status_ok, DateFormat.getDateInstance().format(Date((geo.updated ?: 0) * 1000)))
                    } else stringResource(R.string.geo_status_missing),
                )
                geoProgress?.let { GeoProgressBar(it) }
                OutlinedButton(enabled = !geoBusy && geoProgress == null, onClick = {
                    geoBusy = true
                    scope.act(ctx, snackbar, ok = res.getString(R.string.geo_updated)) {
                        try { Core.updateGeo() } finally { geoBusy = false }
                    }
                }) { Text(stringResource(if (geoBusy || geoProgress != null) R.string.downloading else R.string.geo_update_now)) }
                SwitchRow(stringResource(R.string.geo_auto), s.options.geoAutoUpdate, stringResource(R.string.geo_auto_hint)) {
                    save(opts = s.options.copy(geoAutoUpdate = it))
                }
            }
        }
    }

    presetFor?.let { id ->
        AlertDialog(
            onDismissRequest = { presetFor = null },
            title = { Text(presetName(id)) },
            text = { Text(stringResource(R.string.preset_apply_q)) },
            confirmButton = {
                TextButton(onClick = {
                    presetFor = null
                    val p = Core.presets().first { it.id == id }.rules.orEmpty()
                    save(p)
                }) { Text(stringResource(R.string.preset_replace)) }
            },
            dismissButton = {
                TextButton(onClick = {
                    presetFor = null
                    val p = Core.presets().first { it.id == id }.rules.orEmpty()
                    save(rules + p.filter { n -> rules.none { it.id != null && it.id == n.id } })
                }) { Text(stringResource(R.string.preset_append)) }
            },
        )
    }
}

@Composable
fun presetName(id: String) = stringResource(
    when (id) {
        "global" -> R.string.preset_global
        "bypass_ru" -> R.string.preset_bypass_ru
        "block_ads" -> R.string.preset_block_ads
        "bypass_bittorrent" -> R.string.preset_bypass_bt
        "block_quic" -> R.string.preset_block_quic
        else -> R.string.preset_unknown
    },
)

@Composable
private fun PresetMenu(onPick: (String) -> Unit) {
    var open by remember { mutableStateOf(false) }
    Box {
        OutlinedButton(onClick = { open = true }) { Text(stringResource(R.string.presets)) }
        DropdownMenu(open, { open = false }) {
            Core.presets().forEach { p ->
                DropdownMenuItem(text = { Text(presetName(p.id)) }, onClick = { open = false; onPick(p.id) })
            }
        }
    }
}

@Composable
private fun MoreMenu(onImport: () -> Unit, onExport: () -> Unit) {
    var open by remember { mutableStateOf(false) }
    Box {
        TextButton(onClick = { open = true }) { Text("⋮") }
        DropdownMenu(open, { open = false }) {
            DropdownMenuItem(text = { Text(stringResource(R.string.import_clipboard)) }, onClick = { open = false; onImport() })
            DropdownMenuItem(text = { Text(stringResource(R.string.export_clipboard)) }, onClick = { open = false; onExport() })
        }
    }
}

/** A dropdown button choosing one of [options] (key to label). */
/** "geosite.dat (2 of 2): 12.3 MiB of 20.1 MiB" over a progress bar (indeterminate while the size is unknown). */
@Composable
private fun GeoProgressBar(p: GeoProgress) {
    val f = p.fraction
    if (f != null && !p.verifying) {
        LinearProgressIndicator(progress = { f }, modifier = Modifier.fillMaxWidth())
    } else {
        LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
    }
    val bytes = if (p.total > 0) stringResource(R.string.geo_progress_of, formatBytes(p.done), formatBytes(p.total)) else formatBytes(p.done)
    Hint(
        when {
            p.file.isEmpty() -> stringResource(R.string.geo_connecting)
            p.verifying -> stringResource(R.string.geo_progress, p.file, p.index, p.count, stringResource(R.string.geo_verifying))
            else -> stringResource(R.string.geo_progress, p.file, p.index, p.count, bytes)
        },
    )
}

@Composable
fun Choice(options: List<Pair<String, String>>, selected: String, onPick: (String) -> Unit) {
    var open by remember { mutableStateOf(false) }
    Box {
        OutlinedButton(onClick = { open = true }) {
            Text(options.firstOrNull { it.first == selected }?.second ?: selected)
        }
        DropdownMenu(open, { open = false }) {
            options.forEach { (k, v) ->
                DropdownMenuItem(text = { Text(v) }, onClick = { open = false; onPick(k) })
            }
        }
    }
}

@Composable
private fun RouteTester() {
    val ctx = LocalContext.current
    val res = LocalResources.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    var host by remember { mutableStateOf("") }
    var port by remember { mutableStateOf("443") }
    var udp by remember { mutableStateOf(false) }
    var result by remember { mutableStateOf<String?>(null) }
    Section(stringResource(R.string.route_tester)) {
        OutlinedTextField(host, { host = it.trim() }, label = { Text(stringResource(R.string.host_or_ip)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
        Row(verticalAlignment = Alignment.CenterVertically) {
            OutlinedTextField(port, { port = it.filter(Char::isDigit).take(5) }, label = { Text(stringResource(R.string.port)) }, singleLine = true, modifier = Modifier.width(120.dp))
            Row(Modifier.padding(start = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                Text("UDP")
                Switch(udp, { udp = it })
            }
        }
        val okLocked = stringResource(R.string.verdict_locked)
        val noRule = stringResource(R.string.verdict_default)
        val labels = mapOf("proxy" to stringResource(R.string.out_proxy), "direct" to stringResource(R.string.out_direct), "block" to stringResource(R.string.out_block))
        val ruleN = stringResource(R.string.rule_n)
        OutlinedButton(enabled = host.isNotBlank(), onClick = {
            scope.act(ctx, snackbar) {
                val v = Core.testRoute(host, port.toIntOrNull() ?: 443, if (udp) "udp" else "tcp")
                val by = when {
                    v.locked -> okLocked
                    v.ruleIndex < 0 -> noRule
                    else -> v.remarks?.ifBlank { null } ?: ruleN.format(v.ruleIndex - Core.lockedRules().size + 1)
                }
                result = "→ ${labels[v.outbound] ?: v.outbound} ($by)"
            }
        }) { Text(stringResource(R.string.test)) }
        result?.let { Text(it) }
    }
}
