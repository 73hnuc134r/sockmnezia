package io.nuc134r.sockmnezia.ui

import android.os.Build
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.FloatingActionButton
import androidx.navigation.NavController
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.path
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.ChainResult
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.Probe
import io.nuc134r.sockmnezia.service.ProxyService

/** Height of the start/stop and switch buttons. Its centre sits on the top edge of the bar, so its lower half overlaps
 * the first line of the connection-check block (which leaves room for it) and its upper half the content above. */
private val StartStopButtonHeight = 56.dp

/** Bottom bar of the home screen: start/stop button and a compact connection-check block. Line 1 is the
 * state and the check result (ping); below it the whole route, wrapped rather than cut, with the exit address. */
@Composable
fun ConnectionBar(nav: NavController) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val density = LocalDensity.current
    val status by Core.status.collectAsState()
    val probe by Core.probe.collectAsState()
    var fabWidth by remember { mutableStateOf(0.dp) }
    val start = rememberStarter()
    val testing = probe == Probe.Testing
    // At least four lines tall (status + three route lines) so the block is easy to tap for a check.
    val typo = MaterialTheme.typography
    val minHeight = with(density) { typo.labelLarge.lineHeight.toDp() + typo.bodySmall.lineHeight.toDp() * 3 } + 16.dp

    // The FAB must live outside the Surface: Surface clips its content to its bounds, which cropped the
    // half of the button meant to stick out above it.
    Box(Modifier.fillMaxWidth()) {
        Surface(tonalElevation = 3.dp, modifier = Modifier.fillMaxWidth()) {
            Column(
                Modifier
                    .fillMaxWidth()
                    .padding(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 8.dp)
                    .heightIn(min = minHeight)
                    .clip(RoundedCornerShape(14.dp))
                    .background(MaterialTheme.colorScheme.surfaceContainerHigh)
                    .clickable(enabled = !testing) {
                        scope.act(ctx, snackbar) { Core.runProbe() }
                    }
                    .padding(horizontal = 12.dp, vertical = 8.dp),
            ) {
                // Line 1 ends before the button, which overlaps this corner of the block.
                Row(
                    Modifier.fillMaxWidth().padding(end = (fabWidth - 4.dp).coerceAtLeast(0.dp)),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    val dot = when {
                        probe == Probe.Failed -> MaterialTheme.colorScheme.error
                        status.running -> MaterialTheme.colorScheme.primary
                        else -> MaterialTheme.colorScheme.outline
                    }
                    Box(Modifier.size(8.dp).clip(CircleShape).background(dot))
                    Text(
                        buildAnnotatedString {
                            append(stringResource(if (status.running) R.string.conn_connected else R.string.conn_stopped))
                            withStyle(SpanStyle(color = MaterialTheme.colorScheme.onSurfaceVariant, fontWeight = FontWeight.Normal)) {
                                append(" · ")
                                append(
                                    when (val p = probe) {
                                        Probe.Testing -> stringResource(R.string.conn_testing)
                                        Probe.Failed -> stringResource(R.string.conn_failed)
                                        is Probe.Done -> stringResource(R.string.conn_ping, p.result.ms.toInt())
                                        Probe.Idle -> stringResource(R.string.conn_tap_to_test)
                                    },
                                )
                            }
                        },
                        style = MaterialTheme.typography.labelLarge,
                        modifier = Modifier.padding(start = 8.dp),
                    )
                }
                val route = routeText(status.chain, status.chainCountries, (probe as? Probe.Done)?.result)
                if (route.isNotEmpty()) {
                    Text(
                        route,
                        style = MaterialTheme.typography.bodySmall, fontFamily = FontFamily.Monospace,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }
        // Switch (chain / server selector) and start/stop: two same-size icon buttons side by side, start/stop
        // on the right, straddling the top edge of the bar. Labels are their content descriptions.
        Row(
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            modifier = Modifier
                .align(Alignment.TopEnd)
                .padding(end = 16.dp)
                .offset(y = -(StartStopButtonHeight / 2))
                .onSizeChanged { fabWidth = with(density) { it.width.toDp() } },
        ) {
            FloatingActionButton(
                onClick = { nav.navigate(Routes.SELECT) },
                containerColor = MaterialTheme.colorScheme.secondaryContainer,
                contentColor = MaterialTheme.colorScheme.onSecondaryContainer,
            ) { Icon(SwapIcon, stringResource(R.string.chain_switch)) }
            FloatingActionButton(onClick = {
                if (status.running) ProxyService.stop(ctx) else start()
            }) {
                Icon(
                    if (status.running) Icons.Default.Close else Icons.Default.PlayArrow,
                    stringResource(if (status.running) R.string.action_stop else R.string.action_start),
                )
            }
        }
    }
}

/** The route, phone first, then the exit address once the check has run: "exit ✓" when it is the last hop's
 * address, otherwise "→ exit <ip>". Wraps instead of being cut. */
@Composable
private fun routeText(chain: List<String>, countries: List<String>, exit: ChainResult?): String {
    val hops = chain.mapIndexed { i, addr -> withCountry(addr, countries.getOrNull(i)) }
    return buildString {
        append(hops.joinToString(" → "))
        if (exit != null) {
            if (chain.lastOrNull() == exit.ip) {
                append(" ").append(stringResource(R.string.conn_exit_same))
            } else {
                if (isNotEmpty()) append(" → ")
                append(stringResource(R.string.conn_exit, withCountry(exit.ip, exit.country)))
            }
        }
    }
}

/** Material "swap_horiz" (left/right arrows); the core icon set does not include it. */
private val SwapIcon: ImageVector by lazy {
    ImageVector.Builder("SwapHoriz", 24.dp, 24.dp, 24f, 24f).apply {
        path(fill = SolidColor(Color.Black)) {
            moveTo(6.99f, 11f); lineTo(3f, 15f); lineTo(6.99f, 19f); verticalLineTo(16f)
            horizontalLineTo(14f); verticalLineTo(14f); horizontalLineTo(6.99f); close()
            moveTo(21f, 9f); lineTo(17.01f, 5f); verticalLineTo(8f); horizontalLineTo(10f)
            verticalLineTo(10f); horizontalLineTo(17.01f); verticalLineTo(13f); close()
        }
    }.build()
}

/** "addr [XX]" when the geo database resolved a country for addr, otherwise just addr. */
private fun withCountry(addr: String, country: String?) =
    if (country.isNullOrBlank()) addr else "$addr [$country]"
