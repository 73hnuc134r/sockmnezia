package io.nuc134r.sockmnezia.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.ChainInfo
import io.nuc134r.sockmnezia.core.ChainResult
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.CoreError
import io.nuc134r.sockmnezia.core.OwnServer
import io.nuc134r.sockmnezia.ui.screens.AddServerDialog
import kotlinx.coroutines.launch

/** Result of a row's Test, kept while the sheet is open. */
private sealed interface RowTest {
    data object Running : RowTest
    data class Done(val r: ChainResult) : RowTest
    data class Failed(val error: CoreError) : RowTest
}

/** One entry of a row's ⋮ menu. */
private class MenuAction(val label: String, val onClick: () -> Unit)

/**
 * Screen: what the proxy connects through. Saved chains and, in a separate list, single servers (no one-hop
 * chain needed), plus creating a chain, adding a server (endpoint editor) or importing links. Tapping a row
 * selects it and goes back. Test measures the exit address and ping through that row in a separate temporary
 * instance, without selecting it or touching the running proxy. The ⋮ menu edits, shares, and deletes after
 * a confirmation.
 */
@Composable
fun SelectScreen(nav: NavController) {
    val onDone: () -> Unit = { nav.popBackStack() }
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val revision by Core.revision.collectAsState()
    var chains by remember { mutableStateOf(emptyList<ChainInfo>()) }
    var servers by remember { mutableStateOf(emptyList<OwnServer>()) }
    var deleteChain by remember { mutableStateOf<ChainInfo?>(null) }
    var deleteServer by remember { mutableStateOf<OwnServer?>(null) }
    var shareChain by remember { mutableStateOf<ChainInfo?>(null) }
    var shareServer by remember { mutableStateOf<OwnServer?>(null) }
    var adding by remember { mutableStateOf(false) }
    val tests = remember { mutableStateMapOf<String, RowTest>() }
    LaunchedEffect(revision) {
        runCatching { chains = Core.chains() }
        runCatching { servers = Core.ownServers() }
    }
    fun runTest(key: String, block: suspend () -> ChainResult) {
        tests[key] = RowTest.Running
        scope.launch {
            tests[key] = try {
                RowTest.Done(block())
            } catch (e: Exception) {
                RowTest.Failed(Core.asCoreException(e).error)
            }
        }
    }
    val deleted = stringResource(R.string.hop_missing)
    val labelEdit = stringResource(R.string.chain_edit)
    val labelShare = stringResource(R.string.share)
    val labelDelete = stringResource(R.string.delete)

    Column(Modifier.verticalScroll(rememberScrollState()).padding(bottom = 24.dp)) {
        ListHeader(stringResource(R.string.selector_chains))
        chains.forEach { c ->
            val key = "c:${c.id}"
            SelectRow(
                selected = c.active,
                title = c.name,
                subtitle = c.hops.joinToString(" → ") { if (it.missing) deleted else it.name },
                warning = if (c.error != null) stringResource(R.string.chain_cannot_run) else null,
                test = tests[key],
                onSelect = {
                    scope.act(ctx, snackbar) { Core.setActiveChain(c.id) }
                    onDone()
                },
                onTest = { runTest(key) { Core.testSaved(chainId = c.id) } },
                menu = listOf(
                    MenuAction(labelEdit) { nav.navigate(Routes.chain(c.id)) },
                    MenuAction(labelShare) { shareChain = c },
                    MenuAction(labelDelete) { deleteChain = c },
                ),
            )
        }
        TextButton(onClick = { nav.navigate(Routes.chain(Routes.NEW)) }, modifier = Modifier.padding(horizontal = 8.dp)) {
            Text(stringResource(R.string.chain_new))
        }
        HorizontalDivider(Modifier.padding(vertical = 4.dp))
        ListHeader(stringResource(R.string.selector_servers))
        if (servers.isEmpty()) {
            Hint(stringResource(R.string.selector_no_servers), modifier = Modifier.padding(horizontal = 16.dp))
        }
        // Grouped by host (domain or IP), in the order the hosts first appear.
        servers.groupBy { it.address }.forEach { (host, group) ->
            Text(
                host, style = MaterialTheme.typography.labelSmall, fontFamily = FontFamily.Monospace,
                color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 8.dp),
            )
            group.forEach { s ->
                val key = "s:${s.id}"
                SelectRow(
                    selected = s.active,
                    title = s.name,
                    subtitle = "",
                    tags = { ServerTags(s.protocol, s.security, s.network, s.port, Modifier.padding(top = 2.dp)) },
                    warning = null,
                    test = tests[key],
                    onSelect = {
                        scope.act(ctx, snackbar) { Core.setActiveServer(s.id) }
                        onDone()
                    },
                    onTest = { runTest(key) { Core.testSaved(serverId = s.id) } },
                    menu = listOf(
                        MenuAction(labelEdit) { nav.navigate(Routes.server(s.id)) },
                        MenuAction(labelShare) { shareServer = s },
                        MenuAction(labelDelete) { deleteServer = s },
                    ),
                )
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

@Composable
private fun ListHeader(text: String) {
    Text(
        text.uppercase(), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary,
        modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 12.dp, bottom = 4.dp),
    )
}

@Composable
private fun SelectRow(
    selected: Boolean,
    title: String,
    subtitle: String,
    warning: String?,
    test: RowTest?,
    tags: (@Composable () -> Unit)? = null,
    onSelect: () -> Unit,
    onTest: () -> Unit,
    menu: List<MenuAction>,
) {
    val ctx = LocalContext.current
    var menuOpen by remember { mutableStateOf(false) }
    Row(
        Modifier.fillMaxWidth().clickable(onClick = onSelect).padding(start = 4.dp, end = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        RadioButton(selected = selected, onClick = onSelect)
        Column(Modifier.weight(1f).padding(vertical = 8.dp)) {
            Text(title, maxLines = 1, overflow = TextOverflow.Ellipsis)
            tags?.invoke()
            if (subtitle.isNotBlank()) {
                Text(subtitle, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            warning?.let { Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error) }
            when (test) {
                null -> {}
                RowTest.Running -> Text(stringResource(R.string.conn_testing), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                is RowTest.Done -> Text(
                    stringResource(R.string.test_result, withCountry(test.r.ip, test.r.country), test.r.ms.toInt()),
                    style = MaterialTheme.typography.bodySmall, fontFamily = FontFamily.Monospace, color = MaterialTheme.colorScheme.primary,
                )
                is RowTest.Failed -> Text(errorText(ctx, test.error), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
            }
        }
        TextButton(enabled = test != RowTest.Running, onClick = onTest) { Text(stringResource(R.string.test)) }
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

private fun withCountry(addr: String, country: String?) = if (country.isNullOrBlank()) addr else "$addr [$country]"

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
