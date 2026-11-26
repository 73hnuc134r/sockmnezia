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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.MoreVert
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
@Composable
fun SelectScreen(nav: NavController) {
    val onDone: () -> Unit = { nav.popBackStack() }
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
        val sem = Semaphore(TestConcurrency)
        (chains.map { c -> suspend { test("c:${c.id}") { Core.testSaved(chainId = c.id) } } } +
            servers.map { s -> suspend { test("s:${s.id}") { Core.testSaved(serverId = s.id) } } })
            .map { t -> async { sem.withPermit { t() } } }.awaitAll()
    }
    /** Ping order: fastest first, failed and untested last. */
    fun pingKey(r: TestResult?): Long = if (r?.ok == true) r.ms else Long.MAX_VALUE
    val deleted = stringResource(R.string.hop_missing)
    val labelEdit = stringResource(R.string.chain_edit)
    val labelShare = stringResource(R.string.share)
    val labelDelete = stringResource(R.string.delete)

    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(bottom = 24.dp)) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 2.dp), verticalAlignment = Alignment.CenterVertically) {
            OutlinedButton(enabled = running.isEmpty(), onClick = { testAll() }) { Text(stringResource(R.string.test_all)) }
            Spacer(Modifier.weight(1f))
            listOf("name" to R.string.sort_name, "ping" to R.string.sort_ping).forEach { (k, label) ->
                FilterChip(
                    selected = sort == k, onClick = { sort = k; prefs.listSort = k },
                    label = { Text(stringResource(label)) }, modifier = Modifier.padding(start = 6.dp),
                )
            }
        }

        ListHeader(stringResource(R.string.selector_chains))
        val shownChains = if (sort == "ping") chains.sortedBy { pingKey(it.last) } else chains
        shownChains.forEach { c ->
            SwipeRow(onTest = { testChain(c) }, onDelete = { deleteChain = c }) {
                SelectRow(
                    selected = c.active,
                    title = c.name,
                    subtitle = c.hops.joinToString(" → ") { if (it.missing) deleted else it.name },
                    warning = if (c.error != null) stringResource(R.string.chain_cannot_run) else null,
                    last = c.last, hostCountry = "", testing = running["c:${c.id}"] == true,
                    onSelect = {
                        scope.act(ctx, snackbar) { Core.setActiveChain(c.id) }
                        onDone()
                    },
                    onTest = { testChain(c) },
                    menu = listOf(
                        MenuAction(labelEdit) { nav.navigate(Routes.chain(c.id)) },
                        MenuAction(labelShare) { shareChain = c },
                        MenuAction(labelDelete) { deleteChain = c },
                    ),
                )
            }
        }
        TextButton(onClick = { nav.navigate(Routes.chain(Routes.NEW)) }, modifier = Modifier.padding(horizontal = 8.dp)) {
            Text(stringResource(R.string.chain_new))
        }
        HorizontalDivider(Modifier.padding(vertical = 4.dp))
        ListHeader(stringResource(R.string.selector_servers))
        if (servers.isEmpty()) {
            Hint(stringResource(R.string.selector_no_servers), modifier = Modifier.padding(horizontal = 16.dp))
        }
        // Grouped by host (domain or IP): in the order hosts first appear, or by their fastest server.
        val groups = servers.groupBy { it.address }.entries.toList().let { g ->
            if (sort == "ping") g.sortedBy { e -> e.value.minOf { pingKey(it.last) } } else g
        }
        groups.forEach { (host, group) ->
            Row(Modifier.padding(start = 16.dp, end = 16.dp, top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(host, style = MaterialTheme.typography.labelSmall, fontFamily = FontFamily.Monospace, color = MaterialTheme.colorScheme.onSurfaceVariant)
                group.first().hostCountry.takeIf { it.isNotBlank() }?.let { Tag(it, MaterialTheme.colorScheme.onSurfaceVariant, Modifier.padding(start = 6.dp)) }
            }
            val rows = if (sort == "ping") group.sortedBy { pingKey(it.last) } else group
            rows.forEach { s ->
                SwipeRow(onTest = { testServer(s) }, onDelete = { deleteServer = s }) {
                    SelectRow(
                        selected = s.active,
                        title = s.name,
                        subtitle = "",
                        tags = { ServerTags(s.protocol, s.security, s.network, s.port, Modifier.padding(top = 2.dp)) },
                        warning = null,
                        last = s.last, hostCountry = s.hostCountry, testing = running["s:${s.id}"] == true,
                        onSelect = {
                            scope.act(ctx, snackbar) { Core.setActiveServer(s.id) }
                            onDone()
                        },
                        onTest = { testServer(s) },
                        menu = listOf(
                            MenuAction(labelEdit) { nav.navigate(Routes.server(s.id)) },
                            MenuAction(labelShare) { shareServer = s },
                            MenuAction(labelDelete) { deleteServer = s },
                        ),
                    )
                }
            }
        }
        Row(Modifier.padding(horizontal = 8.dp)) {
            TextButton(onClick = { nav.navigate(Routes.server(Routes.NEW)) }) { Text(stringResource(R.string.add_server)) }
            TextButton(onClick = { adding = true }) { Text(stringResource(R.string.import_links)) }
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
}

/** Swipe right = test, swipe left = delete (asks first); the row always springs back. */
@Composable
private fun SwipeRow(onTest: () -> Unit, onDelete: () -> Unit, content: @Composable () -> Unit) {
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
        Box(Modifier.background(MaterialTheme.colorScheme.surface)) { content() }
    }
}

@Composable
private fun ListHeader(text: String) {
    Text(
        text.uppercase(), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary,
        modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 12.dp, bottom = 2.dp),
    )
}

@Composable
private fun SelectRow(
    selected: Boolean,
    title: String,
    subtitle: String,
    warning: String?,
    last: TestResult?,
    hostCountry: String,
    testing: Boolean,
    tags: (@Composable () -> Unit)? = null,
    onSelect: () -> Unit,
    onTest: () -> Unit,
    menu: List<MenuAction>,
) {
    var menuOpen by remember { mutableStateOf(false) }
    Row(
        Modifier.fillMaxWidth().clickable(onClick = onSelect).padding(start = 4.dp, end = 0.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        RadioButton(selected = selected, onClick = onSelect)
        Column(Modifier.weight(1f).padding(vertical = 6.dp)) {
            Text(title, style = MaterialTheme.typography.bodyLarge, maxLines = 1, overflow = TextOverflow.Ellipsis)
            tags?.invoke()
            if (subtitle.isNotBlank()) {
                Text(
                    subtitle, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
            }
            warning?.let { Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error) }
        }
        PingPill(last, hostCountry, testing, onTest)
        Box {
            IconButton(onClick = { menuOpen = true }) { Icon(Icons.Default.MoreVert, stringResource(R.string.more_actions)) }
            DropdownMenu(menuOpen, onDismissRequest = { menuOpen = false }) {
                menu.forEach { a ->
                    DropdownMenuItem(text = { Text(a.label) }, onClick = { menuOpen = false; a.onClick() })
                }
            }
        }
    }
}

/** The last Test as a small pill ("212 ms", "380 ms · DE" when the exit country differs from the host's),
 * coloured by speed; tapping it runs a test. A failure shows its reason when tapped again in a snackbar. */
@Composable
private fun PingPill(last: TestResult?, hostCountry: String, testing: Boolean, onTest: () -> Unit) {
    val ctx = LocalContext.current
    val snackbar = LocalSnackbar.current
    val scope = rememberCoroutineScope()
    val (text, color) = when {
        testing -> stringResource(R.string.conn_testing) to MaterialTheme.colorScheme.primary
        last == null -> stringResource(R.string.ping_untested) to MaterialTheme.colorScheme.onSurfaceVariant
        !last.ok -> stringResource(R.string.ping_failed) to MaterialTheme.colorScheme.error
        else -> {
            val country = last.country.takeIf { it.isNotBlank() && it != hostCountry }
            (stringResource(R.string.conn_ping, last.ms.toInt()) + (country?.let { " · $it" } ?: "")) to pingColor(last.ms)
        }
    }
    Text(
        text,
        modifier = Modifier
            .padding(start = 4.dp)
            .border(1.dp, color.copy(alpha = 0.45f), RoundedCornerShape(50))
            .clickable(enabled = !testing) {
                val err = last?.error
                if (err != null && !testing) scope.launch { snackbar.showSnackbar(errorText(ctx, CoreError(code = err)), withDismissAction = true) }
                onTest()
            }
            .padding(horizontal = 8.dp, vertical = 3.dp),
        color = color, fontSize = 11.sp, fontFamily = FontFamily.Monospace, fontWeight = FontWeight.Medium, maxLines = 1,
    )
}

/** Green under 300 ms, amber under 800, red above (light and dark shades). */
@Composable
private fun pingColor(ms: Long): Color {
    val dark = androidx.compose.foundation.isSystemInDarkTheme()
    return when {
        ms < 300 -> Color(if (dark) 0xFF8FD8A8 else 0xFF237A4B)
        ms < 800 -> Color(if (dark) 0xFFF2CF7A else 0xFF8A6A00)
        else -> Color(if (dark) 0xFFF2A08E else 0xFFB8452E)
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
