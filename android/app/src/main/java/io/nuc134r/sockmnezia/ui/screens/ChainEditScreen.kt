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
import androidx.compose.runtime.saveable.Saver
import androidx.compose.runtime.saveable.rememberSaveable
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

    // Saveable: the hop picker is another screen, and edits must survive the round trip.
    var loaded by rememberSaveable { mutableStateOf(isNew) }
    var name by rememberSaveable { mutableStateOf("") }
    var hops by rememberSaveable(stateSaver = HopListSaver) { mutableStateOf(emptyList<ChainHop>()) }
    var servers by remember { mutableStateOf(emptyList<OwnServer>()) }
    var error by remember { mutableStateOf<CoreError?>(null) }
    var deleting by remember { mutableStateOf(false) }
    var sharing by remember { mutableStateOf(false) }

    LaunchedEffect(revision) { runCatching { servers = Core.ownServers() } }
    LaunchedEffect(id) {
        if (!loaded) {
            runCatching { Core.chains().firstOrNull { it.id == id } }.getOrNull()?.let { c ->
                name = c.name
                hops = c.toChain().hops
            }
            loaded = true
        }
    }
    // A hop chosen on the picker screen comes back through this entry's saved state.
    val handle = nav.currentBackStackEntry?.savedStateHandle
    val picked by (handle?.getStateFlow<String?>(PickedHop, null) ?: kotlinx.coroutines.flow.MutableStateFlow(null)).collectAsState()
    LaunchedEffect(picked) {
        picked?.let { v ->
            decodeHop(v)?.let { hops = hops + it }
            handle?.set(PickedHop, null)
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
            OutlinedButton(enabled = hops.size < MaxHops, onClick = {
                val amz = when {
                    hops.any { it.kind == HopKind.AMNEZIA } -> "used"
                    status.hasKey && status.hasGatewayKey -> "ready"
                    else -> "setup"
                }
                nav.navigate(Routes.pickHop(amz))
            }) { Text(stringResource(R.string.chain_add_hop)) }
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

/** Key of the hop the picker screen returns: "amnezia" or "server:<id>". */
const val PickedHop = "picked_hop"

fun encodeHop(h: ChainHop): String = if (h.kind == HopKind.AMNEZIA) "amnezia" else "server:${h.server}"

fun decodeHop(v: String): ChainHop? = when {
    v == "amnezia" -> ChainHop(HopKind.AMNEZIA)
    v.startsWith("server:") -> ChainHop(HopKind.SERVER, v.removePrefix("server:"))
    else -> null
}

private val HopListSaver = Saver<List<ChainHop>, String>(
    save = { l -> l.joinToString("\n") { encodeHop(it) } },
    restore = { s -> s.split("\n").mapNotNull { decodeHop(it) } },
)
