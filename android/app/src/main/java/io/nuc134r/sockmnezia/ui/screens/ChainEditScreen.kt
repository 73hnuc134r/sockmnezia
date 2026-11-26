package io.nuc134r.sockmnezia.ui.screens

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
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
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Chain
import io.nuc134r.sockmnezia.core.ChainHop
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.CoreError
import io.nuc134r.sockmnezia.core.HopInfo
import io.nuc134r.sockmnezia.core.HopKind
import io.nuc134r.sockmnezia.core.OwnServer
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.HopDot
import io.nuc134r.sockmnezia.ui.HopTags
import io.nuc134r.sockmnezia.ui.ServerTags
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.Routes
import io.nuc134r.sockmnezia.ui.Section
import io.nuc134r.sockmnezia.ui.ChainShareSheet
import io.nuc134r.sockmnezia.ui.act
import io.nuc134r.sockmnezia.ui.errorText
import io.nuc134r.sockmnezia.ui.hopName

/** Builds or edits a chain: ordered hops (phone first, exit last), validated live by the core. */
@Composable
fun ChainEditScreen(nav: NavController, id: String) {
    val ctx = LocalContext.current
    val res = LocalResources.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val status by Core.status.collectAsState()
    val revision by Core.revision.collectAsState()
    val isNew = id == Routes.NEW

    var loaded by remember { mutableStateOf(isNew) }
    var name by remember { mutableStateOf("") }
    var hops by remember { mutableStateOf(emptyList<ChainHop>()) }
    var servers by remember { mutableStateOf(emptyList<OwnServer>()) }
    var error by remember { mutableStateOf<CoreError?>(null) }
    var adding by remember { mutableStateOf(false) }
    var importing by remember { mutableStateOf(false) }
    var deleting by remember { mutableStateOf(false) }
    var sharing by remember { mutableStateOf(false) }

    LaunchedEffect(revision) { runCatching { servers = Core.ownServers() } }
    LaunchedEffect(id) {
        if (!isNew) {
            runCatching { Core.chains().firstOrNull { it.id == id } }.getOrNull()?.let { c ->
                name = c.name
                hops = c.toChain().hops
            }
            loaded = true
        }
    }
    LaunchedEffect(hops, revision) {
        error = runCatching { Core.validateChain(Chain(name = name, hops = hops)) }.getOrNull()
    }

    fun info(h: ChainHop): HopInfo =
        if (h.kind == HopKind.AMNEZIA) {
            HopInfo(kind = HopKind.AMNEZIA, name = status.countryName.ifBlank { "Amnezia" }, country = status.country)
        } else {
            val s = servers.firstOrNull { it.id == h.server }
            HopInfo(server = h.server, name = s?.name.orEmpty(), protocol = s?.protocol.orEmpty(), network = s?.network.orEmpty(), security = s?.security.orEmpty(), port = s?.port ?: 0, missing = s == null)
        }

    fun move(i: Int, d: Int) {
        val l = hops.toMutableList()
        val j = i + d
        if (j !in l.indices) return
        l[i] = l[j].also { l[j] = l[i] }
        hops = l
    }

    Column(Modifier.verticalScroll(rememberScrollState()).padding(vertical = 8.dp)) {
        Section {
            OutlinedTextField(
                name, { name = it }, label = { Text(stringResource(R.string.chain_name)) },
                singleLine = true, modifier = Modifier.fillMaxWidth(),
            )
            Hint(stringResource(R.string.chain_name_hint))
        }
        Section {
            Hint(stringResource(R.string.chain_editor_hint))
            hops.forEachIndexed { i, h ->
                val hi = info(h)
                Row(verticalAlignment = Alignment.CenterVertically) {
                    HopDot(hi.isAmnezia, if (hi.isAmnezia) "A" else "${i + 1}")
                    Column(Modifier.weight(1f).padding(start = 12.dp)) {
                        Text(hopName(hi), style = MaterialTheme.typography.bodyLarge)
                        HopTags(hi, Modifier.padding(top = 2.dp), withPort = true)
                    }
                    IconButton(enabled = i > 0, onClick = { move(i, -1) }) {
                        Icon(Icons.Default.KeyboardArrowUp, stringResource(R.string.chain_move_up))
                    }
                    IconButton(enabled = i < hops.lastIndex, onClick = { move(i, 1) }) {
                        Icon(Icons.Default.KeyboardArrowDown, stringResource(R.string.chain_move_down))
                    }
                    IconButton(onClick = { hops = hops.filterIndexed { k, _ -> k != i } }) {
                        Icon(Icons.Default.Close, stringResource(R.string.chain_remove_hop))
                    }
                }
                error?.takeIf { it.field == "hop${i + 1}" }?.let { Hint(errorText(ctx, it), warn = true, modifier = Modifier.padding(start = 36.dp)) }
            }
            error?.takeIf { hops.isNotEmpty() && it.field?.startsWith("hop") != true }?.let { Hint(errorText(ctx, it), warn = true) }
            OutlinedButton(enabled = hops.size < MaxHops, onClick = { adding = true }) { Text(stringResource(R.string.chain_add_hop)) }
        }
        Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            if (!isNew) {
                TextButton(onClick = { deleting = true }) { Text(stringResource(R.string.delete)) }
                TextButton(onClick = { sharing = true }) { Text(stringResource(R.string.share)) }
            }
            Spacer(Modifier.weight(1f))
            if (!isNew && status.chainId != id) {
                OutlinedButton(enabled = error == null && hops.isNotEmpty(), onClick = {
                    scope.act(ctx, snackbar) {
                        Core.saveChain(Chain(id, name, hops))
                        Core.setActiveChain(id)
                        nav.popBackStack()
                    }
                }) { Text(stringResource(R.string.chain_use)) }
                Spacer(Modifier.width(8.dp))
            }
            Button(enabled = loaded && error == null && hops.isNotEmpty(), onClick = {
                scope.act(ctx, snackbar, ok = res.getString(R.string.chain_saved)) {
                    val saved = Core.saveChain(Chain(if (isNew) null else id, name, hops))
                    if (isNew) Core.setActiveChain(saved) // a chain just built is the one the user wants
                    nav.popBackStack()
                }
            }) { Text(stringResource(R.string.save)) }
        }
    }

    if (adding) {
        AddHopSheet(
            hops = hops, servers = servers, amneziaReady = status.hasKey && status.hasGatewayKey,
            onPick = { h -> adding = false; hops = hops + h },
            onImport = { adding = false; importing = true },
            onDismiss = { adding = false },
        )
    }
    if (importing) {
        AddServerDialog(onDismiss = { importing = false }, onAdded = { sid ->
            importing = false
            if (sid != null) hops = hops + ChainHop(HopKind.SERVER, sid)
        })
    }
    if (sharing) {
        // Shares the saved chain (not unsaved edits).
        ChainShareSheet(id, name, withAmnezia = hops.any { it.kind == HopKind.AMNEZIA }, onDismiss = { sharing = false })
    }
    if (deleting) {
        AlertDialog(
            onDismissRequest = { deleting = false },
            title = { Text(stringResource(R.string.chain_delete_q, name)) },
            confirmButton = {
                TextButton(onClick = {
                    deleting = false
                    scope.act(ctx, snackbar) {
                        Core.deleteChain(id)
                        nav.popBackStack()
                    }
                }) { Text(stringResource(R.string.delete)) }
            },
            dismissButton = { TextButton(onClick = { deleting = false }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}

/** Same limit as the core (xrayconf.MaxHops). */
private const val MaxHops = 5

/** Amnezia (once per chain, after it is set up), every saved server, or importing a new one. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun AddHopSheet(
    hops: List<ChainHop>,
    servers: List<OwnServer>,
    amneziaReady: Boolean,
    onPick: (ChainHop) -> Unit,
    onImport: () -> Unit,
    onDismiss: () -> Unit,
) {
    // Always fully open: these sheets have no useful half-open state.
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)) {
        // Scrolls: with many saved servers the list is taller than the screen.
        Column(
            Modifier.verticalScroll(rememberScrollState()).padding(start = 16.dp, end = 16.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Text(stringResource(R.string.chain_add_hop), style = MaterialTheme.typography.titleMedium, modifier = Modifier.padding(bottom = 8.dp))
            val amzUsed = hops.any { it.kind == HopKind.AMNEZIA }
            SheetOption(
                dot = { HopDot(true, "A") }, title = "Amnezia",
                subtitle = when {
                    amzUsed -> stringResource(R.string.hop_amnezia_used)
                    !amneziaReady -> stringResource(R.string.hop_amnezia_not_set_up)
                    else -> null
                },
                enabled = !amzUsed && amneziaReady,
            ) { onPick(ChainHop(HopKind.AMNEZIA)) }
            servers.forEach { s ->
                SheetOption(
                    dot = { HopDot(false, "•") }, title = s.name,
                    subtitle = null,
                    tags = { ServerTags(s.protocol, s.security, s.network, s.port, Modifier.padding(top = 2.dp)) },
                ) { onPick(ChainHop(HopKind.SERVER, s.id)) }
            }
            SheetOption(dot = { HopDot(false, "+") }, title = stringResource(R.string.hop_import), subtitle = null, onClick = onImport)
        }
    }
}

@Composable
private fun SheetOption(
    dot: @Composable () -> Unit,
    title: String,
    subtitle: String?,
    enabled: Boolean = true,
    tags: (@Composable () -> Unit)? = null,
    onClick: () -> Unit,
) {
    Row(
        Modifier.fillMaxWidth().clickable(enabled = enabled, onClick = onClick).padding(vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        dot()
        Column(Modifier.padding(start = 12.dp)) {
            Text(title, color = if (enabled) MaterialTheme.colorScheme.onSurface else MaterialTheme.colorScheme.onSurface.copy(alpha = 0.4f))
            tags?.invoke()
            subtitle?.let { Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        }
    }
}
