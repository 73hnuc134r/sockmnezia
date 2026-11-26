package io.nuc134r.sockmnezia.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.HopInfo

/** Amnezia hops are marked "A" in a warm colour; own servers by their position in the chain. */
@Composable
fun hopColor(amnezia: Boolean): Color =
    if (amnezia) MaterialTheme.colorScheme.tertiary else MaterialTheme.colorScheme.primary

@Composable
fun HopDot(amnezia: Boolean, label: String, modifier: Modifier = Modifier) {
    val c = hopColor(amnezia)
    Box(
        modifier
            .size(24.dp)
            .clip(CircleShape)
            .background(MaterialTheme.colorScheme.surfaceContainerHighest)
            .border(2.dp, c, CircleShape),
        contentAlignment = Alignment.Center,
    ) {
        Text(label, color = c, fontSize = 11.sp, fontWeight = FontWeight.SemiBold)
    }
}

/** The hop's name, or "deleted server" when it no longer exists. */
@Composable
fun hopName(h: HopInfo): String = if (h.missing) stringResource(R.string.hop_missing) else h.name

/** Phone → hops → internet, as a vertical stepper. The last hop is marked as the exit. */
@Composable
fun ChainStepper(hops: List<HopInfo>, modifier: Modifier = Modifier) {
    Column(modifier.fillMaxWidth()) {
        StepRow(first = true, last = false, dot = { EndDot() }) { Text(stringResource(R.string.chain_phone), style = MaterialTheme.typography.bodyMedium) }
        hops.forEachIndexed { i, h ->
            StepRow(first = false, last = false, dot = { HopDot(h.isAmnezia, if (h.isAmnezia) "A" else "${i + 1}") }) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text(hopName(h), style = MaterialTheme.typography.bodyMedium)
                        if (h.detail.isNotBlank()) {
                            Text(h.detail, style = MaterialTheme.typography.bodySmall, fontFamily = FontFamily.Monospace, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                    if (i == hops.lastIndex) {
                        Text(
                            stringResource(R.string.chain_exit).uppercase(),
                            style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.primary,
                        )
                    }
                }
            }
        }
        StepRow(first = false, last = true, dot = { EndDot() }) { Text(stringResource(R.string.chain_internet), style = MaterialTheme.typography.bodyMedium) }
    }
}

@Composable
private fun EndDot() {
    Box(Modifier.size(24.dp), contentAlignment = Alignment.Center) {
        Box(Modifier.size(10.dp).clip(CircleShape).background(MaterialTheme.colorScheme.outline))
    }
}

/** One stepper row: a dot on a vertical line (the line stops at the first and last row), then [content]. */
@Composable
private fun StepRow(first: Boolean, last: Boolean, dot: @Composable () -> Unit, content: @Composable () -> Unit) {
    val line = MaterialTheme.colorScheme.outlineVariant
    Row(verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.width(24.dp), contentAlignment = Alignment.Center) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Box(Modifier.width(2.dp).height(6.dp).background(if (first) Color.Transparent else line))
                dot()
                Box(Modifier.width(2.dp).height(6.dp).background(if (last) Color.Transparent else line))
            }
        }
        Box(Modifier.weight(1f).padding(start = 12.dp)) { content() }
    }
}

/** A tiny tag: 10sp text in a tinted rounded box, as small as stays readable. */
@Composable
fun Tag(text: String, color: Color, modifier: Modifier = Modifier) {
    Text(
        text,
        modifier = modifier
            .background(color.copy(alpha = 0.16f), androidx.compose.foundation.shape.RoundedCornerShape(4.dp))
            .padding(horizontal = 4.dp),
        color = color, fontSize = 10.sp, lineHeight = 14.sp, fontWeight = FontWeight.Medium, maxLines = 1,
    )
}

/** Tag colours: one hue per protocol and per security, light and dark variants for contrast. */
@Composable
private fun tagColor(light: Long, dark: Long): Color =
    Color(if (androidx.compose.foundation.isSystemInDarkTheme()) dark else light)

@Composable
fun protocolColor(protocol: String): Color = when (protocol) {
    "vless" -> tagColor(0xFF2F6FC8, 0xFF8DB8FF)
    "vmess" -> tagColor(0xFF8048BD, 0xFFC9A6F2)
    "trojan" -> tagColor(0xFFB8452E, 0xFFF2A08E)
    "shadowsocks" -> tagColor(0xFF237A4B, 0xFF8FD8A8)
    "socks" -> tagColor(0xFF7A6A1E, 0xFFE0CF7A)
    else -> MaterialTheme.colorScheme.onSurfaceVariant
}

/** Reality and TLS in their own colours; no encryption in the error colour, so it stands out. */
@Composable
fun securityColor(security: String): Color = when (security) {
    "reality" -> tagColor(0xFF137A6C, 0xFF7FE0D0)
    "tls" -> tagColor(0xFF4A58C2, 0xFFAEB8FF)
    else -> MaterialTheme.colorScheme.error
}

/** Protocol, security, transport and port as tiny tags ("ss" for shadowsocks to keep it short). */
@Composable
fun ServerTags(protocol: String, security: String, network: String, port: Int, modifier: Modifier = Modifier) {
    val neutral = MaterialTheme.colorScheme.onSurfaceVariant
    Row(modifier, horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(4.dp)) {
        if (protocol.isNotBlank()) Tag(if (protocol == "shadowsocks") "ss" else protocol, protocolColor(protocol))
        if (security.isNotBlank()) Tag(security, securityColor(security))
        if (network.isNotBlank() && network != "tcp" && network != "raw") Tag(network, neutral)
        if (port > 0) Tag(":$port", neutral)
    }
}
