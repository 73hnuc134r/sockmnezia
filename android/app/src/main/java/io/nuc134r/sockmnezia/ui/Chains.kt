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
