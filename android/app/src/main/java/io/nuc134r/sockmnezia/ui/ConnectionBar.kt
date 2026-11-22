package io.nuc134r.sockmnezia.ui

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.service.ProxyService

/** Height of the start/stop button: it overlaps the connection-check block below by exactly half of this,
 * so it does not sit fully inside the block (covering its text) nor take extra space above it. */
private val StartStopButtonHeight = 56.dp

/** Bottom bar of the home screen: start/stop button, connection state, route, exit address and ping. */
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
        Box(Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, bottom = 12.dp)) {
            Column(
                Modifier
                    .fillMaxWidth()
                    .padding(top = StartStopButtonHeight / 2)
                    .clip(RoundedCornerShape(16.dp))
                    .background(MaterialTheme.colorScheme.surfaceContainerHigh)
                    .clickable(enabled = !testing) {
                        testing = true
                        scope.act(ctx, snackbar) { try { Core.runProbe() } finally { testing = false } }
                    }
                    .padding(start = 16.dp, end = 16.dp, top = 12.dp, bottom = 12.dp),
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                Text(
                    stringResource(if (status.running) R.string.conn_connected else R.string.conn_stopped),
                    style = MaterialTheme.typography.titleMedium,
                )
                if (status.chain.isNotEmpty()) {
                    val route = status.chain.mapIndexed { i, addr ->
                        withCountry(addr, status.chainCountries.getOrNull(i))
                    }.joinToString(" → ")
                    Text(
                        stringResource(R.string.conn_route, route),
                        style = MaterialTheme.typography.bodySmall, fontFamily = FontFamily.Monospace,
                        maxLines = 3, overflow = TextOverflow.Ellipsis,
                    )
                }
                val p = probe
                Text(
                    when {
                        testing -> stringResource(R.string.conn_testing)
                        p != null -> stringResource(R.string.conn_exit, withCountry(p.ip, p.country)) + " · " + stringResource(R.string.conn_ping, p.ms.toInt())
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
                modifier = Modifier.align(Alignment.TopEnd).offset(y = -(StartStopButtonHeight / 2)),
            )
        }
    }
}

/** "addr [XX]" when the geo database resolved a country for addr, otherwise just addr. */
private fun withCountry(addr: String, country: String?) =
    if (country.isNullOrBlank()) addr else "$addr [$country]"
