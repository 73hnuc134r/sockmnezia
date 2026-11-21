package io.nuc134r.sockmnezia.ui

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.ExtendedFloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
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
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.service.ProxyService

/** Bottom bar of the home screen: connection state, route, exit address, ping and the start/stop button. */
@Composable
fun ConnectionBar() {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val status by Core.status.collectAsState()
    val probe by Core.probe.collectAsState()
    var testing by remember { mutableStateOf(false) }
    val notifPermission = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) {
        ProxyService.start(ctx)
    }
    LaunchedEffect(status.running) { Core.clearProbe() }

    Surface(tonalElevation = 3.dp, modifier = Modifier.fillMaxWidth()) {
        Row(
            Modifier.padding(start = 16.dp, end = 16.dp, top = 12.dp, bottom = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Column(
                Modifier.weight(1f).clickable(enabled = !testing) {
                    testing = true
                    scope.act(ctx, snackbar) { try { Core.runProbe() } finally { testing = false } }
                },
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                Text(
                    stringResource(if (status.running) R.string.conn_connected else R.string.conn_stopped),
                    style = MaterialTheme.typography.titleMedium,
                )
                if (status.chain.isNotEmpty()) {
                    Text(
                        stringResource(R.string.conn_route, status.chain.joinToString(" → ")),
                        style = MaterialTheme.typography.bodySmall, fontFamily = FontFamily.Monospace,
                        maxLines = 3, overflow = TextOverflow.Ellipsis,
                    )
                }
                val p = probe
                Text(
                    when {
                        testing -> stringResource(R.string.conn_testing)
                        p != null -> stringResource(R.string.conn_exit, p.ip) + " · " + stringResource(R.string.conn_ping, p.ms.toInt())
                        else -> stringResource(R.string.conn_tap_to_test)
                    },
                    style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    fontFamily = if (p != null && !testing) FontFamily.Monospace else null,
                )
            }
            ExtendedFloatingActionButton(
                onClick = {
                    if (status.running) {
                        ProxyService.stop(ctx)
                    } else if (Build.VERSION.SDK_INT >= 33 &&
                        ContextCompat.checkSelfPermission(ctx, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
                    ) {
                        notifPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
                    } else {
                        ProxyService.start(ctx)
                    }
                },
                icon = { Icon(if (status.running) Icons.Default.Close else Icons.Default.PlayArrow, null) },
                text = { Text(stringResource(if (status.running) R.string.action_stop else R.string.action_start)) },
            )
        }
    }
}
