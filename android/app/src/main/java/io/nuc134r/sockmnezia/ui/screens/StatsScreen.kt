package io.nuc134r.sockmnezia.ui.screens

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.ui.input.pointer.pointerInput
import java.time.LocalDate
import java.time.format.DateTimeFormatter
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.StatsDay
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.Section
import io.nuc134r.sockmnezia.ui.GroupRow
import io.nuc134r.sockmnezia.ui.Island
import io.nuc134r.sockmnezia.ui.TipRow
import androidx.compose.material3.LinearProgressIndicator
import io.nuc134r.sockmnezia.ui.act
import io.nuc134r.sockmnezia.ui.formatBytes
import kotlinx.coroutines.delay

/** Traffic per day: through each chain or single server, and directly (rules around the proxy). Recorded by
 * the core while the proxy runs; kept on the phone only. */
@Composable
fun StatsScreen() {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    var period by rememberSaveable { mutableIntStateOf(7) }
    var days by remember { mutableStateOf(emptyList<StatsDay>()) }
    var reset by remember { mutableStateOf(false) }
    var reload by remember { mutableIntStateOf(0) }
    var selected by remember(period) { mutableStateOf<Int?>(null) }
    LaunchedEffect(period, reload) {
        while (true) {
            runCatching { days = Core.stats(period) }
            delay(5000)
        }
    }
    val chainColor = MaterialTheme.colorScheme.primary
    val directColor = MaterialTheme.colorScheme.tertiary
    val all = days.flatMap { it.entries }
    val sent = all.sumOf { it.up }
    val received = all.sumOf { it.down }
    val direct = all.filter { it.kind == "direct" }.sumOf { it.up + it.down }
    // Per chain/server over the period, biggest first.
    val byKey = all.groupBy { it.key }.map { (_, l) -> Triple(l.last(), l.sumOf { it.up }, l.sumOf { it.down }) }
        .sortedByDescending { it.second + it.third }

    Column(Modifier.verticalScroll(rememberScrollState()).padding(vertical = 8.dp)) {
        SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp)) {
            listOf(1 to R.string.stats_today, 7 to R.string.stats_7d, 30 to R.string.stats_30d).forEachIndexed { i, (d, label) ->
                SegmentedButton(selected = period == d, onClick = { period = d }, shape = SegmentedButtonDefaults.itemShape(i, 3)) {
                    Text(stringResource(label))
                }
            }
        }
        Section {
            Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                Total(stringResource(R.string.stats_received), received, Modifier.weight(1f))
                Total(stringResource(R.string.stats_sent), sent, Modifier.weight(1f))
            }
            // Wraps whole items, never in the middle of one.
            FlowRow(horizontalArrangement = Arrangement.spacedBy(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Legend(chainColor, stringResource(R.string.stats_through_chain, formatBytes(sent + received - direct)))
                Legend(directColor, stringResource(R.string.stats_direct, formatBytes(direct)))
            }
            if (period > 1) {
                val max = days.maxOfOrNull { d -> d.entries.sumOf { it.up + it.down } }?.coerceAtLeast(1) ?: 1
                // The day picked by tapping or dragging on the chart (the phone's hover), with its exact numbers.
                val sel = selected?.takeIf { it in days.indices }
                DayDetail(sel?.let { days[it] }, chainColor, directColor)
                fun dayAt(x: Float, width: Int) = (x / width * days.size).toInt().coerceIn(0, days.lastIndex)
                Canvas(
                    Modifier.fillMaxWidth().height(120.dp)
                        .pointerInput(days.size) { detectTapGestures { o -> if (days.isNotEmpty()) selected = dayAt(o.x, size.width) } }
                        .pointerInput(days.size) {
                            detectHorizontalDragGestures { change, _ -> if (days.isNotEmpty()) selected = dayAt(change.position.x, size.width) }
                        },
                ) {
                    if (days.isEmpty()) return@Canvas
                    val slot = size.width / days.size
                    val bw = (slot * 0.7f).coerceAtLeast(2f)
                    days.forEachIndexed { i, d ->
                        val dir = d.entries.filter { it.kind == "direct" }.sumOf { it.up + it.down }
                        val total = d.entries.sumOf { it.up + it.down }
                        val x = i * slot + (slot - bw) / 2
                        val hTotal = size.height * total / max
                        val hDir = size.height * dir / max
                        val alpha = if (sel == null || sel == i) 1f else 0.35f
                        if (total > 0) {
                            drawRoundRect(chainColor.copy(alpha = alpha), Offset(x, size.height - hTotal), Size(bw, hTotal - hDir), CornerRadius(3f, 3f))
                            if (dir > 0) drawRoundRect(directColor.copy(alpha = alpha), Offset(x, size.height - hDir), Size(bw, hDir), CornerRadius(3f, 3f))
                        } else {
                            drawRect(Color.Gray.copy(alpha = if (sel == i) 0.8f else 0.25f), Offset(x, size.height - 2f), Size(bw, 2f))
                        }
                    }
                }
                Row(Modifier.fillMaxWidth()) {
                    Text(days.firstOrNull()?.date?.drop(5).orEmpty(), style = MaterialTheme.typography.labelSmall, modifier = Modifier.weight(1f))
                    Text(days.lastOrNull()?.date?.drop(5).orEmpty(), style = MaterialTheme.typography.labelSmall)
                }
            }
        }
        Island(title = stringResource(R.string.stats_by_endpoint)) {
            if (byKey.isEmpty()) TipRow(stringResource(R.string.stats_empty))
            val top = byKey.maxOfOrNull { it.second + it.third }?.coerceAtLeast(1) ?: 1
            byKey.forEachIndexed { i, (e, up, down) ->
                val name = when {
                    e.kind == "direct" -> stringResource(R.string.stats_direct_name)
                    e.deleted -> stringResource(R.string.stats_deleted, e.name)
                    else -> e.name
                }
                GroupRow(i, byKey.size) {
                    // Name on its own line, the numbers under it: long names are not cut.
                    Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(name, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f), style = MaterialTheme.typography.bodyLarge)
                            Text(formatBytes(up + down), style = MaterialTheme.typography.labelLarge)
                        }
                        LinearProgressIndicator(
                            progress = { (up + down).toFloat() / top },
                            color = if (e.kind == "direct") directColor else chainColor,
                            trackColor = MaterialTheme.colorScheme.surfaceContainerHigh,
                            drawStopIndicator = {},
                            modifier = Modifier.fillMaxWidth().height(6.dp),
                        )
                        Text(
                            "↓ ${formatBytes(down)}   ↑ ${formatBytes(up)}",
                            style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            }
        }
        Hint(stringResource(R.string.stats_hint), modifier = Modifier.padding(horizontal = 24.dp, vertical = 4.dp))
        TextButton(onClick = { reset = true }, modifier = Modifier.padding(horizontal = 12.dp)) { Text(stringResource(R.string.stats_reset)) }
    }

    if (reset) {
        AlertDialog(
            onDismissRequest = { reset = false },
            title = { Text(stringResource(R.string.stats_reset_q)) },
            confirmButton = {
                TextButton(onClick = {
                    reset = false
                    scope.act(ctx, snackbar) { Core.resetStats(); reload++ }
                }) { Text(stringResource(R.string.delete)) }
            },
            dismissButton = { TextButton(onClick = { reset = false }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}

@Composable
private fun Total(label: String, bytes: Long, modifier: Modifier = Modifier) {
    Column(modifier) {
        Text(label, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(formatBytes(bytes), style = MaterialTheme.typography.headlineSmall)
    }
}

@Composable
private fun Legend(color: Color, text: String) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        Box(Modifier.size(10.dp).background(color, CircleShape))
        Text(text, style = MaterialTheme.typography.bodySmall, maxLines = 1, softWrap = false)
    }
}

/** The picked day's numbers above the chart; a hint until one is picked. Always two lines tall, so the chart
 * doesn't jump. */
@Composable
private fun DayDetail(day: StatsDay?, chainColor: Color, directColor: Color) {
    Column(
        Modifier.fillMaxWidth().height(56.dp)
            .background(MaterialTheme.colorScheme.surfaceContainerHigh, RoundedCornerShape(12.dp))
            .padding(horizontal = 10.dp, vertical = 6.dp),
        verticalArrangement = Arrangement.Center,
    ) {
        if (day == null) {
            Text(stringResource(R.string.stats_tap_hint), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            return@Column
        }
        val up = day.entries.sumOf { it.up }
        val down = day.entries.sumOf { it.down }
        val direct = day.entries.filter { it.kind == "direct" }.sumOf { it.up + it.down }
        val locale = LocalConfiguration.current.locales[0]
        val date = runCatching {
            LocalDate.parse(day.date).format(DateTimeFormatter.ofPattern("EEE, d MMM", locale))
        }.getOrDefault(day.date)
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Text(date, style = MaterialTheme.typography.labelLarge)
            Text("↓ ${formatBytes(down)}   ↑ ${formatBytes(up)}", style = MaterialTheme.typography.labelLarge)
        }
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
            Legend(chainColor, stringResource(R.string.stats_through_chain, formatBytes(up + down - direct)))
            Legend(directColor, stringResource(R.string.stats_direct, formatBytes(direct)))
        }
    }
}
