package io.nuc134r.sockmnezia.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Share
import androidx.compose.material3.ExtendedFloatingActionButton
import androidx.compose.material3.FloatingActionButton
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.RadioButton
import androidx.compose.material3.SwipeToDismissBox
import androidx.compose.material3.SwipeToDismissBoxValue
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberSwipeToDismissBoxState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.navigation.NavController
import io.nuc134r.sockmnezia.AppPrefs
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.ChainInfo
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.CoreError
import io.nuc134r.sockmnezia.core.OwnServer
import io.nuc134r.sockmnezia.core.TestResult
import io.nuc134r.sockmnezia.ui.screens.AddServerDialog
import io.nuc134r.sockmnezia.ui.screens.PickedHop
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withPermit

/** One entry of a row's ⋮ menu. */
private class MenuAction(val label: String, val onClick: () -> Unit)

/** Tests run at once by "Test all" (each starts a temporary Xray instance). */
private const val TestConcurrency = 4

/**
 * Screen: what the proxy connects through. Saved chains and, in a separate list grouped by host, single
 * servers, plus creating a chain, adding a server or importing links. Tapping a row selects it and goes back.
 * The ping pill shows the last Test (kept by the core) and runs one when tapped; swiping a row right tests it,
 * left deletes it (after a confirmation). Tests run in a temporary instance and never touch the running proxy.
 */
/**
 * The same screen as a hop picker for the chain editor ([pickHop] = Amnezia's state there: "ready", "used" or
 * "setup"): no chains, Amnezia as its own row at the top, and tapping a row hands the hop back to the editor
 * (through its saved state) instead of selecting it.
 */
@Composable
fun SelectScreen(nav: NavController, pickHop: String? = null) {
    val picking = pickHop != null
    val onDone: () -> Unit = { nav.popBackStack() }
    fun pick(v: String) {
        nav.previousBackStackEntry?.savedStateHandle?.set(PickedHop, v)
        nav.popBackStack()
    }
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val prefs = remember { AppPrefs(ctx) }
    val revision by Core.revision.collectAsState()
    var reload by remember { mutableIntStateOf(0) }
    var chains by remember { mutableStateOf(emptyList<ChainInfo>()) }
    var servers by remember { mutableStateOf(emptyList<OwnServer>()) }
    var deleteChain by remember { mutableStateOf<ChainInfo?>(null) }
    var deleteServer by remember { mutableStateOf<OwnServer?>(null) }
    var shareChain by remember { mutableStateOf<ChainInfo?>(null) }
    var shareServer by remember { mutableStateOf<OwnServer?>(null) }
    var shareAll by remember { mutableStateOf(false) }
    var adding by remember { mutableStateOf(false) }
    var sort by remember { mutableStateOf(prefs.listSort) }
    /** Tests in flight; finished results come from the core with the lists. */
    val running = remember { mutableStateMapOf<String, Boolean>() }
    LaunchedEffect(revision, reload) {
        runCatching { chains = Core.chains() }
        runCatching { servers = Core.ownServers() }
    }
    suspend fun test(key: String, block: suspend () -> Unit) {
        running[key] = true
        runCatching { block() }
        running.remove(key)
        reload++
    }
    fun testChain(c: ChainInfo) = scope.launch { test("c:${c.id}") { Core.testSaved(chainId = c.id) } }
    fun testServer(s: OwnServer) = scope.launch { test("s:${s.id}") { Core.testSaved(serverId = s.id) } }
    fun testAll() = scope.launch {
        // Start from a clean list: old pings must not sit next to new ones.
        runCatching { Core.clearTests() }
        runCatching { chains = Core.chains() }
        runCatching { servers = Core.ownServers() }
        val sem = Semaphore(TestConcurrency)
        ((if (picking) emptyList() else chains).map { c -> suspend { test("c:${c.id}") { Core.testSaved(chainId = c.id) } } } +
            servers.map { s -> suspend { test("s:${s.id}") { Core.testSaved(serverId = s.id) } } })
            .map { t -> async { sem.withPermit { t() } } }.awaitAll()
    }
    /** Ping order: fastest first, failed and untested last. */
    fun pingKey(r: TestResult?): Long = if (r?.ok == true) r.ms else Long.MAX_VALUE
    val deleted = stringResource(R.string.hop_missing)
    val labelEdit = stringResource(R.string.chain_edit)
    val labelShare = stringResource(R.string.share)
    val labelDelete = stringResource(R.string.delete)

    val hostCountryOf = servers.associate { it.id to it.hostCountry }
    var addMenu by remember { mutableStateOf(false) }

    Box(Modifier.fillMaxSize()) {
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(bottom = 96.dp)) {
        Row(Modifier.fillMaxWidth().padding(start = 16.dp, end = 8.dp, bottom = 4.dp), verticalAlignment = Alignment.CenterVertically) {
            listOf("name" to R.string.sort_name, "ping" to R.string.sort_ping).forEach { (k, label) ->
                FilterChip(
                    selected = sort == k, onClick = { sort = k; prefs.listSort = k },
                    label = { Text(stringResource(label)) }, modifier = Modifier.padding(end = 8.dp),
                )
            }
            Spacer(Modifier.weight(1f))
            if (!picking && (servers.isNotEmpty() || chains.isNotEmpty())) {
                IconButton(onClick = { shareAll = true }) { Icon(Icons.Default.Share, stringResource(R.string.share_all)) }
            }
        }

        if (pickHop != null) {
            Island(title = "Amnezia") { AmneziaPickRow(pickHop) { pick("amnezia") } }
        } else if (chains.isNotEmpty()) {
            val shownChains = if (sort == "ping") chains.sortedBy { pingKey(it.last) } else chains
            Island(title = stringResource(R.string.selector_chains), trailing = {
                Text("${chains.size}", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }) {
                shownChains.forEachIndexed { i, c ->
                    val shape = groupShape(i, shownChains.size)
                    SwipeRow(shape, onTest = { testChain(c) }, onDelete = { deleteChain = c }) {
                        GroupRow(i, shownChains.size, selected = c.active, onClick = {
                            scope.act(ctx, snackbar) { Core.setActiveChain(c.id) }
                            onDone()
                        }) {
                            Avatar(c.name, c.active)
                            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(3.dp)) {
                                Text(c.name, style = MaterialTheme.typography.bodyLarge, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                ChainPath(c, hostCountryOf, deleted, onPrimary = c.active)
                                if (c.error != null) {
                                    Text(stringResource(R.string.chain_cannot_run), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
                                }
                            }
                            PingPill(c.last, "", running["c:${c.id}"] == true, onPrimary = c.active) { testChain(c) }
                            RowMenu(listOf(
                                MenuAction(labelEdit) { nav.navigate(Routes.chain(c.id)) },
                                MenuAction(labelShare) { shareChain = c },
                                MenuAction(labelDelete) { deleteChain = c },
                            ))
                        }
                    }
                }
            }
        }

        if (servers.isEmpty()) {
            Island { TipRow(stringResource(R.string.selector_no_servers)) }
        }
        // One island per host (domain or IP): in the order hosts first appear, or by their fastest server.
        val groups = servers.groupBy { it.address }.entries.toList().let { g ->
            if (sort == "ping") g.sortedBy { e -> e.value.minOf { pingKey(it.last) } } else g
        }
        groups.forEach { (host, group) ->
            val rows = if (sort == "ping") group.sortedBy { pingKey(it.last) } else group
            val country = group.first().hostCountry
            Island {
                Row(Modifier.fillMaxWidth().padding(start = 8.dp, end = 8.dp, top = 6.dp, bottom = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                    Box(
                        Modifier.size(32.dp).clip(RoundedCornerShape(10.dp)).background(MaterialTheme.colorScheme.surfaceContainerHigh),
                        contentAlignment = Alignment.Center,
                    ) { Icon(DnsIcon, null, Modifier.size(18.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant) }
                    Column(Modifier.weight(1f).padding(horizontal = 10.dp)) {
                        Text(host, style = MaterialTheme.typography.titleSmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        Text(
                            pluralStringResource(R.plurals.servers_count, group.size, group.size),
                            style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    CountryPing(country.takeIf { it.isNotBlank() }, null, text = countryName(country))
                }
                rows.forEachIndexed { i, sv ->
                    val selected = !picking && sv.active
                    SwipeRow(groupShape(i, rows.size), onTest = { testServer(sv) }, onDelete = { deleteServer = sv }) {
                        GroupRow(i, rows.size, selected = selected, onClick = {
                            if (picking) {
                                pick("server:${sv.id}")
                            } else {
                                scope.act(ctx, snackbar) { Core.setActiveServer(sv.id) }
                                onDone()
                            }
                        }) {
                            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                                Text(sv.name, style = MaterialTheme.typography.bodyLarge, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                ServerTags(sv.protocol, sv.security, sv.network, sv.port, onPrimary = selected)
                            }
                            PingPill(sv.last, sv.hostCountry, running["s:${sv.id}"] == true, onPrimary = selected) { testServer(sv) }
                            RowMenu(listOf(
                                MenuAction(labelEdit) { nav.navigate(Routes.server(sv.id)) },
                                MenuAction(labelShare) { shareServer = sv },
                                MenuAction(labelDelete) { deleteServer = sv },
                            ))
                        }
                    }
                }
            }
        }
    }

    // Thumb actions at the bottom right (D43): test everything, add.
    Row(
        Modifier.align(Alignment.BottomEnd).padding(16.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        FloatingActionButton(
            onClick = { if (running.isEmpty()) testAll() },
            containerColor = MaterialTheme.colorScheme.secondaryContainer,
            contentColor = MaterialTheme.colorScheme.onSecondaryContainer,
        ) { Icon(Icons.Default.Refresh, stringResource(R.string.test_all)) }
        Box {
            ExtendedFloatingActionButton(
                onClick = { addMenu = true },
                icon = { Icon(Icons.Default.Add, null) },
                text = { Text(stringResource(R.string.add)) },
            )
            DropdownMenu(addMenu, onDismissRequest = { addMenu = false }) {
                if (!picking) DropdownMenuItem(text = { Text(stringResource(R.string.chain_new)) }, onClick = {
                    addMenu = false; nav.navigate(Routes.chain(Routes.NEW))
                })
                DropdownMenuItem(text = { Text(stringResource(R.string.add_server)) }, onClick = {
                    addMenu = false; nav.navigate(Routes.server(Routes.NEW))
                })
                DropdownMenuItem(text = { Text(stringResource(R.string.import_links)) }, onClick = { addMenu = false; adding = true })
            }
        }
    }
    }

    deleteChain?.let { c ->
        ConfirmDelete(stringResource(R.string.chain_delete_q, c.name), onDismiss = { deleteChain = null }) {
            scope.act(ctx, snackbar) { Core.deleteChain(c.id) }
        }
    }
    deleteServer?.let { s ->
        ConfirmDelete(stringResource(R.string.delete_server_q, s.name), onDismiss = { deleteServer = null }) {
            scope.act(ctx, snackbar) { Core.removeOwnServer(s.id) }
        }
    }
    shareChain?.let { c ->
        ChainShareSheet(c.id, c.name, withAmnezia = c.hops.any { it.isAmnezia }, onDismiss = { shareChain = null })
    }
    shareServer?.let { s -> ServerShareSheet(s.id, s.name, onDismiss = { shareServer = null }) }
    if (adding) AddServerDialog(onDismiss = { adding = false }, onAdded = { adding = false })
    if (shareAll) AllShareSheet(onDismiss = { shareAll = false })
}

/** Amnezia as a hop: disabled with the reason when it is already in the chain or not set up yet. */
@Composable
private fun AmneziaPickRow(state: String, onPick: () -> Unit) {
    val status by Core.status.collectAsState()
    val enabled = state == "ready"
    GroupRow(0, 1, onClick = if (enabled) onPick else null) {
        HopDot(true, "A")
        Column(Modifier.weight(1f)) {
            val ink = if (enabled) MaterialTheme.colorScheme.onSurface else MaterialTheme.colorScheme.onSurface.copy(alpha = 0.4f)
            Text("Amnezia Premium", style = MaterialTheme.typography.bodyLarge, color = ink)
            when (state) {
                "used" -> Hint(stringResource(R.string.hop_amnezia_used))
                "setup" -> Hint(stringResource(R.string.hop_amnezia_not_set_up))
                else -> Pill("amnezia", MaterialTheme.colorScheme.tertiaryContainer, MaterialTheme.colorScheme.onTertiaryContainer, Modifier.padding(top = 4.dp))
            }
        }
        if (enabled) CountryPing(status.country.takeIf { it.isNotBlank() }, null, text = status.countryName.takeIf { it.isNotBlank() })
    }
}

/** A chain's letter (or a check when selected) in a circle. */
@Composable
private fun Avatar(name: String, selected: Boolean) {
    val cs = MaterialTheme.colorScheme
    Box(
        Modifier.size(40.dp).clip(CircleShape).background(if (selected) cs.primary else cs.secondaryContainer),
        contentAlignment = Alignment.Center,
    ) {
        if (selected) Icon(Icons.Default.Check, null, tint = cs.onPrimary, modifier = Modifier.size(22.dp))
        else Text(name.trim().take(1).uppercase(), color = cs.onSecondaryContainer, fontWeight = FontWeight.SemiBold)
    }
}

/** "[DE] → [NL] · VLESS · Reality": the hops' countries (Amnezia: its location) and the protocols used. */
@Composable
private fun ChainPath(c: ChainInfo, hostCountryOf: Map<String, String>, deleted: String, onPrimary: Boolean) {
    val fg = if (onPrimary) MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.8f) else MaterialTheme.colorScheme.onSurfaceVariant
    val countries = c.hops.map { h -> if (h.isAmnezia) h.country else hostCountryOf[h.server].orEmpty() }
    val protocols = c.hops.filter { !it.missing && !it.isAmnezia }.map { protocolLabel(it.protocol) }.distinct() +
        (if (c.hops.any { it.isAmnezia }) listOf("Amnezia") else emptyList())
    androidx.compose.foundation.layout.FlowRow(
        horizontalArrangement = Arrangement.spacedBy(4.dp), verticalArrangement = Arrangement.spacedBy(2.dp),
        itemVerticalAlignment = Alignment.CenterVertically,
    ) {
        if (countries.any { it.isNotBlank() }) {
            countries.forEachIndexed { i, cc ->
                if (i > 0) Text("→", color = fg, fontSize = 12.sp)
                if (cc.isNotBlank()) CountryCode(cc) else Text(if (c.hops[i].missing) deleted else "?", color = fg, fontSize = 12.sp)
            }
            Spacer(Modifier.width(4.dp))
        }
        Text(protocols.joinToString(" · "), color = fg, fontSize = 12.sp, maxLines = 1)
    }
}

@Composable
private fun RowMenu(menu: List<MenuAction>) {
    var open by remember { mutableStateOf(false) }
    Box {
        IconButton(onClick = { open = true }, modifier = Modifier.size(36.dp)) { Icon(Icons.Default.MoreVert, stringResource(R.string.more_actions)) }
        DropdownMenu(open, onDismissRequest = { open = false }) {
            menu.forEach { a -> DropdownMenuItem(text = { Text(a.label) }, onClick = { open = false; a.onClick() }) }
        }
    }
}

/** Swipe right = test, swipe left = delete (asks first); the row always springs back. */
@Composable
private fun SwipeRow(shape: androidx.compose.ui.graphics.Shape, onTest: () -> Unit, onDelete: () -> Unit, content: @Composable () -> Unit) {
    val state = rememberSwipeToDismissBoxState(confirmValueChange = { v ->
        when (v) {
            SwipeToDismissBoxValue.StartToEnd -> onTest()
            SwipeToDismissBoxValue.EndToStart -> onDelete()
            SwipeToDismissBoxValue.Settled -> {}
        }
        false
    })
    SwipeToDismissBox(
        state = state,
        modifier = Modifier.clip(shape),
        backgroundContent = {
            val toEnd = state.dismissDirection == SwipeToDismissBoxValue.StartToEnd
            val (bg, fg) = if (toEnd) {
                MaterialTheme.colorScheme.secondaryContainer to MaterialTheme.colorScheme.onSecondaryContainer
            } else {
                MaterialTheme.colorScheme.errorContainer to MaterialTheme.colorScheme.onErrorContainer
            }
            Box(
                Modifier.fillMaxSize().background(bg).padding(horizontal = 20.dp),
                contentAlignment = if (toEnd) Alignment.CenterStart else Alignment.CenterEnd,
            ) {
                Text(stringResource(if (toEnd) R.string.test else R.string.delete), color = fg, style = MaterialTheme.typography.labelLarge)
            }
        },
    ) {
        content()
    }
}

/** The last Test as a country+ping pill (the exit's country when it differs from the host's); tapping it runs a
 * test. A failure shows its reason in a snackbar when tapped. */
@Composable
private fun PingPill(last: TestResult?, hostCountry: String, testing: Boolean, onPrimary: Boolean = false, onTest: () -> Unit) {
    val ctx = LocalContext.current
    val snackbar = LocalSnackbar.current
    val scope = rememberCoroutineScope()
    val country = last?.country?.takeIf { it.isNotBlank() && it != hostCountry }
    Box(
        Modifier.clip(RoundedCornerShape(8.dp)).clickable(enabled = !testing) {
            val err = last?.error
            if (err != null && !testing) scope.launch { snackbar.showSnackbar(errorText(ctx, CoreError(code = err)), withDismissAction = true) }
            onTest()
        },
    ) {
        when {
            testing -> CountryPing(null, null, text = stringResource(R.string.conn_testing), onPrimary = onPrimary)
            last == null -> CountryPing(null, null, text = stringResource(R.string.ping_untested), onPrimary = onPrimary)
            !last.ok -> CountryPing(country, null, failed = true, text = stringResource(R.string.ping_failed))
            else -> CountryPing(country, last.ms, onPrimary = onPrimary)
        }
    }
}

@Composable
private fun ConfirmDelete(title: String, onDismiss: () -> Unit, onConfirm: () -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        confirmButton = {
            TextButton(onClick = { onDismiss(); onConfirm() }) { Text(stringResource(R.string.delete)) }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) } },
    )
}
