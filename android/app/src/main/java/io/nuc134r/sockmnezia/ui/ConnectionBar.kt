package io.nuc134r.sockmnezia.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.path
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.Probe
import io.nuc134r.sockmnezia.service.ProxyService
import java.util.Locale

private val DockButton = 64.dp
private val DockShape = RoundedCornerShape(20.dp)

/** The home screen's thumb row (D43), right above the navigation bar: the connection status (tap = check the exit
 * address and country), the chain/server switcher, and start/stop in the bottom-right corner. */
@Composable
fun ThumbDock(nav: androidx.navigation.NavController) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val status by Core.status.collectAsState()
    val probe by Core.probe.collectAsState()
    val start = rememberStarter()
    val testing = probe == Probe.Testing
    val cs = MaterialTheme.colorScheme
    val done = probe as? Probe.Done
    val (bg, fg) = when {
        probe == Probe.Failed -> cs.errorContainer to cs.onErrorContainer
        status.running -> cs.primaryContainer to cs.onPrimaryContainer
        else -> cs.surfaceContainerHighest to cs.onSurface
    }

    Row(
        Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Surface(
            onClick = { scope.act(ctx, snackbar) { Core.runProbe() } },
            enabled = !testing,
            shape = DockShape, color = bg, contentColor = fg,
            modifier = Modifier.weight(1f).height(DockButton),
        ) {
            Row(Modifier.padding(horizontal = 14.dp), verticalAlignment = Alignment.CenterVertically) {
                val dot = when {
                    probe == Probe.Failed -> cs.error
                    status.running -> cs.primary
                    else -> cs.outline
                }
                Box(Modifier.size(10.dp).clip(CircleShape).background(dot))
                Column(Modifier.weight(1f).padding(horizontal = 12.dp)) {
                    val state = stringResource(if (status.running) R.string.conn_connected else R.string.conn_stopped)
                    val detail = when (val p = probe) {
                        Probe.Testing -> stringResource(R.string.conn_testing)
                        Probe.Failed -> stringResource(R.string.conn_failed)
                        is Probe.Done -> stringResource(R.string.conn_ping, p.result.ms.toInt())
                        Probe.Idle -> null
                    }
                    Text(
                        if (detail != null) "$state · $detail" else state,
                        style = MaterialTheme.typography.labelLarge, maxLines = 1, overflow = TextOverflow.Ellipsis,
                    )
                    if (done != null) {
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                            val code = done.result.country
                            if (code.isNotBlank()) {
                                CountryCode(code, container = Color.White.copy(alpha = 0.7f), content = cs.onPrimaryContainer)
                            }
                            Text(
                                listOfNotNull(countryName(code), done.result.ip).joinToString(" · "),
                                style = MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis,
                                color = fg.copy(alpha = 0.8f),
                            )
                        }
                    } else {
                        Text(
                            if (probe == Probe.Idle) stringResource(R.string.conn_tap_check) else routeText(status.chain, status.chainCountries),
                            style = MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis,
                            color = fg.copy(alpha = 0.8f),
                        )
                    }
                }
                Icon(Icons.Default.Refresh, null, Modifier.size(20.dp))
            }
        }
        Surface(
            onClick = { nav.navigate(Routes.SELECT) },
            shape = DockShape, color = cs.secondaryContainer, contentColor = cs.onSecondaryContainer,
            modifier = Modifier.size(DockButton),
        ) {
            Box(contentAlignment = Alignment.Center) { Icon(SwapIcon, stringResource(R.string.chain_switch), Modifier.size(26.dp)) }
        }
        Surface(
            onClick = { if (status.running) ProxyService.stop(ctx) else start() },
            shape = DockShape, color = cs.primary, contentColor = cs.onPrimary, shadowElevation = 3.dp,
            modifier = Modifier.size(DockButton),
        ) {
            Box(contentAlignment = Alignment.Center) {
                Icon(
                    if (status.running) StopIcon else Icons.Default.PlayArrow,
                    stringResource(if (status.running) R.string.action_stop else R.string.action_start),
                    Modifier.size(28.dp),
                )
            }
        }
    }
}

/** The country's name in the UI language, from its two-letter code; null when unknown. */
fun countryName(code: String): String? =
    code.takeIf { it.length == 2 }?.let { Locale("", it.uppercase()).getDisplayCountry(Locale.getDefault()) }?.takeIf { it.isNotBlank() && it.length > 2 }

/** The hops' addresses with their countries: "203.0.113.5 [DE] → …". */
private fun routeText(chain: List<String>, countries: List<String>): String =
    chain.mapIndexed { i, addr -> withCountry(addr, countries.getOrNull(i)) }.joinToString(" → ")

/** Material "swap_horiz" (left/right arrows); the core icon set does not include it. */
val SwapIcon: ImageVector by lazy {
    ImageVector.Builder("SwapHoriz", 24.dp, 24.dp, 24f, 24f).apply {
        path(fill = SolidColor(Color.Black)) {
            moveTo(6.99f, 11f); lineTo(3f, 15f); lineTo(6.99f, 19f); verticalLineTo(16f)
            horizontalLineTo(14f); verticalLineTo(14f); horizontalLineTo(6.99f); close()
            moveTo(21f, 9f); lineTo(17.01f, 5f); verticalLineTo(8f); horizontalLineTo(10f)
            verticalLineTo(10f); horizontalLineTo(17.01f); verticalLineTo(13f); close()
        }
    }.build()
}

/** A rounded square, Material "stop"; not in the core icon set. */
val StopIcon: ImageVector by lazy {
    ImageVector.Builder("Stop", 24.dp, 24.dp, 24f, 24f).apply {
        path(fill = SolidColor(Color.Black)) {
            moveTo(8f, 6f); horizontalLineTo(16f)
            curveTo(17.1f, 6f, 18f, 6.9f, 18f, 8f); verticalLineTo(16f)
            curveTo(18f, 17.1f, 17.1f, 18f, 16f, 18f); horizontalLineTo(8f)
            curveTo(6.9f, 18f, 6f, 17.1f, 6f, 16f); verticalLineTo(8f)
            curveTo(6f, 6.9f, 6.9f, 6f, 8f, 6f); close()
        }
    }.build()
}

/** "addr [XX]" when the geo database resolved a country for addr, otherwise just addr. */
private fun withCountry(addr: String, country: String?) =
    if (country.isNullOrBlank()) addr else "$addr [$country]"
