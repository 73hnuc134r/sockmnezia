package io.nuc134r.sockmnezia.ui

import androidx.compose.foundation.background
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

/** A hop's number in a filled circle: own servers in the secondary container, Amnezia ("A") in the tertiary. */
@Composable
fun HopDot(amnezia: Boolean, label: String, modifier: Modifier = Modifier) {
    val cs = MaterialTheme.colorScheme
    Box(
        modifier.size(32.dp).clip(CircleShape).background(if (amnezia) cs.tertiaryContainer else cs.secondaryContainer),
        contentAlignment = Alignment.Center,
    ) {
        Text(label, color = if (amnezia) cs.onTertiaryContainer else cs.onSecondaryContainer, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
    }
}

/** The hop's name, or "deleted server" when it no longer exists. */
@Composable
fun hopName(h: HopInfo): String = if (h.missing) stringResource(R.string.hop_missing) else h.name

/** What is known about one hop for display: its country and last ping. */
data class HopState(val country: String? = null, val ms: Long? = null, val failed: Boolean = false)

/** The hops top to bottom as a numbered rail: number, name, country+ping, then the pills (D43). Amnezia is "A". */
@Composable
fun ChainStepper(hops: List<HopInfo>, modifier: Modifier = Modifier, state: (Int) -> HopState = { HopState() }) {
    val cs = MaterialTheme.colorScheme
    Column(modifier.fillMaxWidth()) {
        hops.forEachIndexed { i, h ->
            if (i > 0) Box(Modifier.padding(start = 15.dp, top = 4.dp, bottom = 4.dp).width(2.dp).height(14.dp).background(cs.outlineVariant))
            val st = state(i)
            Row(verticalAlignment = Alignment.Top) {
                Box(
                    Modifier.padding(top = 2.dp).size(32.dp).clip(CircleShape)
                        .background(if (h.isAmnezia) cs.tertiaryContainer else cs.secondaryContainer),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        if (h.isAmnezia) "A" else "${i + 1}", fontSize = 13.sp, fontWeight = FontWeight.SemiBold,
                        color = if (h.isAmnezia) cs.onTertiaryContainer else cs.onSecondaryContainer,
                    )
                }
                Column(Modifier.weight(1f).padding(start = 14.dp), verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(6.dp)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(hopName(h), style = MaterialTheme.typography.bodyLarge, modifier = Modifier.weight(1f))
                        CountryPing(st.country ?: h.country.takeIf { it.isNotBlank() }, st.ms, failed = st.failed)
                    }
                    HopTags(h)
                }
            }
        }
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

/** Protocol, security, transport and port as pills ("ss" for shadowsocks to keep it short; the port without ":").
 * No encryption is shown in the error colours so it stands out. */
@Composable
fun ServerTags(protocol: String, security: String, network: String, port: Int, modifier: Modifier = Modifier, onPrimary: Boolean = false) {
    val cs = MaterialTheme.colorScheme
    val tonal = if (onPrimary) Color.White.copy(alpha = 0.6f) else cs.secondaryContainer
    val onTonal = if (onPrimary) cs.onPrimaryContainer else cs.onSecondaryContainer
    val neutral = if (onPrimary) Color.White.copy(alpha = 0.6f) else cs.surfaceContainerHigh
    val onNeutral = if (onPrimary) cs.onPrimaryContainer else cs.onSurfaceVariant
    // Wraps to a second line rather than cutting the last pills off when the row is narrow.
    androidx.compose.foundation.layout.FlowRow(
        modifier,
        horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(4.dp),
        verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(4.dp),
    ) {
        if (protocol.isNotBlank()) Pill(protocolLabel(protocol), tonal, onTonal)
        when {
            security == "reality" || security == "tls" -> Pill(securityLabel(security), tonal, onTonal)
            security.isNotBlank() -> Pill(security, cs.errorContainer, cs.onErrorContainer)
        }
        if (network.isNotBlank() && network != "tcp" && network != "raw") Pill(networkLabel(network), tonal, onTonal)
        if (port > 0) Pill("$port", neutral, onNeutral)
    }
}

fun protocolLabel(p: String) = when (p) {
    "vless" -> "VLESS"; "vmess" -> "VMess"; "trojan" -> "Trojan"; "shadowsocks" -> "SS"; "socks" -> "SOCKS"; "http" -> "HTTP"
    else -> p
}

fun securityLabel(s: String) = when (s) { "reality" -> "Reality"; "tls" -> "TLS"; else -> s }

fun networkLabel(n: String) = when (n) {
    "ws" -> "WS"; "grpc" -> "gRPC"; "xhttp" -> "XHTTP"; "httpupgrade" -> "HTTPUpgrade"; "kcp" -> "mKCP"; "h2", "http" -> "H2"
    else -> n
}

/** Pills for a hop: "amnezia" for Amnezia, otherwise the server's protocol, security, transport and port. */
@Composable
fun HopTags(h: HopInfo, modifier: Modifier = Modifier, withPort: Boolean = true, onPrimary: Boolean = false) {
    if (h.isAmnezia) {
        Pill("amnezia", MaterialTheme.colorScheme.tertiaryContainer, MaterialTheme.colorScheme.onTertiaryContainer, modifier)
    } else if (!h.missing) {
        ServerTags(h.protocol, h.security, h.network, if (withPort) h.port else 0, modifier, onPrimary)
    }
}
