package io.nuc134r.sockmnezia.ui.screens

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.FilterChip
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.Rule
import io.nuc134r.sockmnezia.core.Settings
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.Section
import io.nuc134r.sockmnezia.ui.SwitchRow
import io.nuc134r.sockmnezia.ui.act
import io.nuc134r.sockmnezia.ui.errorText
import java.util.UUID

private fun lines(s: String) = s.split('\n', ',').map { it.trim() }.filter { it.isNotEmpty() }

@Composable
fun RuleEditorScreen(nav: NavController, index: Int) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    var settings by remember { mutableStateOf<Settings?>(null) }
    LaunchedEffect(Unit) { settings = Core.settings() }
    val s = settings ?: return
    val orig = s.routing.rules.getOrNull(index) ?: Rule(id = UUID.randomUUID().toString(), enabled = true)

    var remarks by remember(orig) { mutableStateOf(orig.remarks.orEmpty()) }
    var domains by remember(orig) { mutableStateOf(orig.domain.orEmpty().joinToString("\n")) }
    var ips by remember(orig) { mutableStateOf(orig.ip.orEmpty().joinToString("\n")) }
    var port by remember(orig) { mutableStateOf(orig.port.orEmpty()) }
    var network by remember(orig) { mutableStateOf(orig.network.orEmpty()) }
    var protocols by remember(orig) { mutableStateOf(orig.protocol.orEmpty().toSet()) }
    var outbound by remember(orig) { mutableStateOf(orig.outboundTag) }
    var enabled by remember(orig) { mutableStateOf(orig.isEnabled) }
    var picker by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }

    Column(Modifier.verticalScroll(rememberScrollState()).padding(vertical = 8.dp)) {
        Section {
            OutlinedTextField(remarks, { remarks = it }, label = { Text(stringResource(R.string.rule_name)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
            Text(stringResource(R.string.rule_outbound))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                listOf("proxy", "direct", "block").forEach { o ->
                    FilterChip(selected = outbound == o, onClick = { outbound = o }, label = { Text(outboundLabel(o)) })
                }
            }
            if (outbound == "direct") Hint(stringResource(R.string.direct_hint), warn = true)
            SwitchRow(stringResource(R.string.rule_enabled), enabled) { enabled = it }
        }
        Section(stringResource(R.string.rule_domains)) {
            OutlinedTextField(domains, { domains = it }, minLines = 3, modifier = Modifier.fillMaxWidth(), placeholder = { Text("domain:example.com\ngeosite:category-ru") })
            Hint(stringResource(R.string.rule_domains_hint))
            OutlinedButton(onClick = { picker = "site" }) { Text(stringResource(R.string.add_geosite)) }
        }
        Section(stringResource(R.string.rule_ips)) {
            OutlinedTextField(ips, { ips = it }, minLines = 2, modifier = Modifier.fillMaxWidth(), placeholder = { Text("198.51.100.0/24\ngeoip:ru") })
            OutlinedButton(onClick = { picker = "ip" }) { Text(stringResource(R.string.add_geoip)) }
        }
        Section(stringResource(R.string.rule_more)) {
            OutlinedTextField(port, { port = it }, label = { Text(stringResource(R.string.rule_port)) }, placeholder = { Text("80,443,1000-2000") }, singleLine = true, modifier = Modifier.fillMaxWidth())
            Text(stringResource(R.string.rule_network))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                listOf("" to R.string.any, "tcp" to null, "udp" to null).forEach { (n, label) ->
                    FilterChip(selected = network == n, onClick = { network = n }, label = { Text(label?.let { stringResource(it) } ?: n.uppercase()) })
                }
            }
            Text(stringResource(R.string.rule_protocol))
            listOf("http", "tls", "quic", "bittorrent").forEach { p ->
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Checkbox(p in protocols, { on -> protocols = if (on) protocols + p else protocols - p })
                    Text(p)
                }
            }
        }
        error?.let { Section { Hint(it, warn = true) } }
        Row(Modifier.padding(16.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = {
                val rule = orig.copy(
                    remarks = remarks.trim().ifBlank { null },
                    domain = lines(domains).ifEmpty { null },
                    ip = lines(ips).ifEmpty { null },
                    port = port.trim().ifBlank { null },
                    network = network.ifBlank { null },
                    protocol = protocols.toList().ifEmpty { null },
                    outboundTag = outbound,
                    enabled = enabled,
                )
                try {
                    Core.validateRule(rule)
                } catch (e: Exception) {
                    error = errorText(ctx, Core.asCoreException(e).error)
                    return@Button
                }
                val rules = s.routing.rules.toMutableList()
                if (index in rules.indices) rules[index] = rule else rules.add(rule)
                scope.act(ctx, snackbar) {
                    Core.setSettings(s.copy(routing = s.routing.copy(rules = rules)))
                    nav.popBackStack()
                }
            }) { Text(stringResource(R.string.save)) }
            TextButton(onClick = { nav.popBackStack() }) { Text(stringResource(R.string.cancel)) }
        }
    }

    picker?.let { kind ->
        var tags by remember { mutableStateOf<List<String>?>(null) }
        var filter by remember { mutableStateOf("") }
        LaunchedEffect(kind) {
            tags = try { Core.geoTags(kind) } catch (e: Exception) {
                error = errorText(ctx, Core.asCoreException(e).error)
                picker = null
                emptyList()
            }
        }
        AlertDialog(
            onDismissRequest = { picker = null },
            title = { Text(stringResource(if (kind == "site") R.string.add_geosite else R.string.add_geoip)) },
            text = {
                Column {
                    OutlinedTextField(filter, { filter = it.lowercase() }, singleLine = true, label = { Text(stringResource(R.string.search)) })
                    LazyColumn(Modifier.heightIn(max = 360.dp)) {
                        items(tags.orEmpty().filter { filter in it }.take(500)) { t ->
                            Text(t, Modifier.fillMaxWidth().clickable {
                                if (kind == "site") domains = (lines(domains) + "geosite:$t").joinToString("\n")
                                else ips = (lines(ips) + "geoip:$t").joinToString("\n")
                                picker = null
                            }.padding(vertical = 8.dp))
                        }
                    }
                }
            },
            confirmButton = { TextButton(onClick = { picker = null }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}
