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
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.automirrored.filled.List
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material3.ExtendedFloatingActionButton
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.FloatingActionButton
import androidx.compose.ui.draw.alpha
import io.nuc134r.sockmnezia.ui.GroupRow
import io.nuc134r.sockmnezia.ui.Island
import io.nuc134r.sockmnezia.ui.Pill
import io.nuc134r.sockmnezia.ui.RowText
import io.nuc134r.sockmnezia.ui.TipRow
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

    Box(Modifier.fillMaxSize()) {
    LazyColumn(Modifier.fillMaxWidth(), contentPadding = androidx.compose.foundation.layout.PaddingValues(top = 4.dp, bottom = 96.dp)) {
        item { RouteTester() }
        item {
            Island(title = stringResource(R.string.your_rules), trailing = {
                Text(stringResource(R.string.first_match_wins), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
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
            }) {
                if (rules.isEmpty()) TipRow(stringResource(R.string.routing_hint))
                rules.forEachIndexed { i, r ->
                    GroupRow(i, rules.size, onClick = { nav.navigate(Routes.rule(i)) }, modifier = Modifier.alpha(if (r.isEnabled) 1f else 0.6f)) {
                        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                Text(
                                    r.remarks?.ifBlank { null } ?: stringResource(R.string.rule_n, i + 1),
                                    style = MaterialTheme.typography.bodyLarge, maxLines = 1, overflow = TextOverflow.Ellipsis,
                                    modifier = Modifier.weight(1f, fill = false),
                                )
                                OutboundPill(r.outboundTag)
                            }
                            RuleChips(r)
                        }
                        Switch(checked = r.isEnabled, onCheckedChange = { on ->
                            save(rules.toMutableList().also { it[i] = r.copy(enabled = on) })
                        })
                        RuleMenu(
                            canUp = i > 0, canDown = i < rules.size - 1,
                            onUp = { save(rules.toMutableList().also { it.add(i - 1, it.removeAt(i)) }) },
                            onDown = { save(rules.toMutableList().also { it.add(i + 1, it.removeAt(i)) }) },
                            onDelete = { save(rules.toMutableList().also { it.removeAt(i) }) },
                        )
                    }
                }
            }
        }
        item {
            Island(title = stringResource(R.string.default_route)) {
                val locked = Core.lockedRules()
                val n = 1 + locked.size
                GroupRow(0, n) {
                    RowText(stringResource(R.string.not_matched), Modifier.weight(1f), subtitle = stringResource(R.string.default_route_hint))
                    OutboundPill("proxy")
                }
                locked.forEachIndexed { j, r ->
                    GroupRow(j + 1, n) {
                        Icon(Icons.Default.Lock, null, tint = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.size(20.dp))
                        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            Text(stringResource(if (r.ip != null) R.string.locked_ip else R.string.locked_domain), style = MaterialTheme.typography.bodyLarge)
                            RuleChips(r)
                        }
                        OutboundPill(r.outboundTag)
                    }
                }
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

    Row(
        Modifier.align(Alignment.BottomEnd).padding(16.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        PresetMenu { id -> presetFor = id }
        FloatingActionButton(onClick = { nav.navigate(Routes.rule(-1)) }) { Icon(Icons.Default.Add, stringResource(R.string.add_rule)) }
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
        ExtendedFloatingActionButton(
            onClick = { open = true },
            icon = { Icon(Icons.AutoMirrored.Filled.List, null) },
            text = { Text(stringResource(R.string.presets)) },
            containerColor = MaterialTheme.colorScheme.secondaryContainer,
            contentColor = MaterialTheme.colorScheme.onSecondaryContainer,
        )
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
        IconButton(onClick = { open = true }, modifier = Modifier.size(36.dp)) { Icon(Icons.Default.MoreVert, stringResource(R.string.more_actions)) }
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
    var verdict by remember { mutableStateOf<Pair<String, String>?>(null) }
    Section(stringResource(R.string.route_tester)) {
        OutlinedTextField(
            host, { host = it.trim() }, label = { Text(stringResource(R.string.host_or_ip)) }, singleLine = true, modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(16.dp),
        )
        val okLocked = stringResource(R.string.verdict_locked)
        val noRule = stringResource(R.string.verdict_default)
        val ruleN = stringResource(R.string.rule_n)
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            OutlinedTextField(
                port, { port = it.filter(Char::isDigit).take(5) }, label = { Text(stringResource(R.string.port)) }, singleLine = true,
                modifier = Modifier.width(110.dp), shape = RoundedCornerShape(16.dp),
            )
            Text("UDP")
            Switch(udp, { udp = it })
            Spacer(Modifier.weight(1f))
            FilledTonalButton(enabled = host.isNotBlank(), onClick = {
                scope.act(ctx, snackbar) {
                    val v = Core.testRoute(host, port.toIntOrNull() ?: 443, if (udp) "udp" else "tcp")
                    val by = when {
                        v.locked -> okLocked
                        v.ruleIndex < 0 -> noRule
                        else -> v.remarks?.ifBlank { null } ?: ruleN.format(v.ruleIndex - Core.lockedRules().size + 1)
                    }
                    verdict = v.outbound to by
                }
            }) { Text(stringResource(R.string.test)) }
        }
        verdict?.let { (out, by) ->
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutboundPill(out)
                Text(by, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}

/** Proxy (teal), Direct (blue) or Block (red) as a pill. */
@Composable
fun OutboundPill(tag: String) {
    val cs = MaterialTheme.colorScheme
    val (bg, fg) = when (tag) {
        "direct" -> cs.tertiaryContainer to cs.onTertiaryContainer
        "block" -> cs.errorContainer to cs.onErrorContainer
        else -> cs.primaryContainer to cs.onPrimaryContainer
    }
    Pill(outboundLabel(tag), bg, fg)
}

/** A rule's conditions as small monospace chips: domains, IPs, port, network, protocols. */
@Composable
fun RuleChips(r: Rule) {
    val cs = MaterialTheme.colorScheme
    val chips = r.domain.orEmpty() + r.ip.orEmpty() +
        listOfNotNull(r.port?.takeIf { it.isNotBlank() }?.let { "port $it" }, r.network?.takeIf { it.isNotBlank() }) +
        r.protocol.orEmpty().map { "protocol:$it" }
    androidx.compose.foundation.layout.FlowRow(horizontalArrangement = Arrangement.spacedBy(4.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
        chips.take(6).forEach { Pill(it, cs.surfaceContainerHigh, cs.onSurfaceVariant, mono = true) }
        if (chips.size > 6) Pill("+${chips.size - 6}", cs.surfaceContainerHigh, cs.onSurfaceVariant)
    }
}

@Composable
private fun RuleMenu(canUp: Boolean, canDown: Boolean, onUp: () -> Unit, onDown: () -> Unit, onDelete: () -> Unit) {
    var open by remember { mutableStateOf(false) }
    Box {
        IconButton(onClick = { open = true }, modifier = Modifier.size(36.dp)) { Icon(Icons.Default.MoreVert, stringResource(R.string.more_actions)) }
        DropdownMenu(open, { open = false }) {
            if (canUp) DropdownMenuItem(text = { Text(stringResource(R.string.move_up)) }, onClick = { open = false; onUp() })
            if (canDown) DropdownMenuItem(text = { Text(stringResource(R.string.move_down)) }, onClick = { open = false; onDown() })
            DropdownMenuItem(text = { Text(stringResource(R.string.delete)) }, onClick = { open = false; onDelete() })
        }
    }
}
