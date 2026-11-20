package io.nuc134r.sockmnezia.ui.screens

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
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
import androidx.compose.ui.unit.dp
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.OwnServer
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.Section
import io.nuc134r.sockmnezia.ui.act
import io.nuc134r.sockmnezia.ui.paste

@Composable
fun ServersScreen() {
    val ctx = LocalContext.current
    val res = LocalResources.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val revision by Core.revision.collectAsState()
    var servers by remember { mutableStateOf(emptyList<OwnServer>()) }
    var adding by remember { mutableStateOf(false) }
    var renaming by remember { mutableStateOf<OwnServer?>(null) }
    var deleting by remember { mutableStateOf<OwnServer?>(null) }
    LaunchedEffect(revision) { runCatching { servers = Core.ownServers() } }

    LazyColumn(Modifier.fillMaxWidth()) {
        item {
            Section {
                Hint(stringResource(R.string.servers_hint))
                Button(onClick = { adding = true }) { Text(stringResource(R.string.add_server)) }
            }
        }
        items(servers, key = { it.id }) { s ->
            Section(modifier = Modifier.clickable { scope.act(ctx, snackbar) { Core.setActiveOwnServer(s.id) } }) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    RadioButton(selected = s.active, onClick = { scope.act(ctx, snackbar) { Core.setActiveOwnServer(s.id) } })
                    Column(Modifier.weight(1f)) {
                        Text(s.name, style = MaterialTheme.typography.titleSmall)
                        Text(
                            "${s.protocol} · ${s.network}/${s.security} · ${s.address}:${s.port}",
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                    IconButton(onClick = { renaming = s }) { Icon(Icons.Default.Edit, stringResource(R.string.rename)) }
                    IconButton(onClick = { deleting = s }) { Icon(Icons.Default.Delete, stringResource(R.string.delete)) }
                }
            }
        }
    }

    if (adding) {
        var link by remember { mutableStateOf("") }
        var name by remember { mutableStateOf("") }
        AlertDialog(
            onDismissRequest = { adding = false },
            title = { Text(stringResource(R.string.add_server)) },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedTextField(link, { link = it }, label = { Text(stringResource(R.string.server_link)) }, minLines = 3, modifier = Modifier.fillMaxWidth())
                    OutlinedButton(onClick = { link = paste(ctx) }) { Text(stringResource(R.string.paste)) }
                    OutlinedTextField(name, { name = it }, label = { Text(stringResource(R.string.name_optional)) }, singleLine = true)
                    Hint(stringResource(R.string.server_link_hint))
                }
            },
            confirmButton = {
                TextButton(enabled = link.isNotBlank(), onClick = {
                    scope.act(ctx, snackbar, ok = res.getString(R.string.server_added)) {
                        Core.addOwnServer(link.trim(), name)
                        adding = false
                    }
                }) { Text(stringResource(R.string.add)) }
            },
            dismissButton = { TextButton(onClick = { adding = false }) { Text(stringResource(R.string.cancel)) } },
        )
    }
    renaming?.let { s ->
        var name by remember(s.id) { mutableStateOf(s.name) }
        AlertDialog(
            onDismissRequest = { renaming = null },
            title = { Text(stringResource(R.string.rename)) },
            text = { OutlinedTextField(name, { name = it }, singleLine = true) },
            confirmButton = {
                TextButton(onClick = {
                    renaming = null
                    scope.act(ctx, snackbar) { Core.renameOwnServer(s.id, name) }
                }) { Text(stringResource(R.string.save)) }
            },
            dismissButton = { TextButton(onClick = { renaming = null }) { Text(stringResource(R.string.cancel)) } },
        )
    }
    deleting?.let { s ->
        AlertDialog(
            onDismissRequest = { deleting = null },
            title = { Text(stringResource(R.string.delete_server_q, s.name)) },
            confirmButton = {
                TextButton(onClick = {
                    deleting = null
                    scope.act(ctx, snackbar) { Core.removeOwnServer(s.id) }
                }) { Text(stringResource(R.string.delete)) }
            },
            dismissButton = { TextButton(onClick = { deleting = null }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}
