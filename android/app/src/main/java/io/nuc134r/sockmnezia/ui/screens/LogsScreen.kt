package io.nuc134r.sockmnezia.ui.screens

import android.content.Intent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.LogLine
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.copy
import kotlinx.coroutines.delay
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

@Composable
fun LogsScreen() {
    val ctx = LocalContext.current
    var lines by remember { mutableStateOf(emptyList<LogLine>()) }
    LaunchedEffect(Unit) {
        while (true) {
            lines = Core.logs()
            delay(2000)
        }
    }
    val fmt = remember { SimpleDateFormat("HH:mm:ss", Locale.ROOT) }
    fun text() = lines.joinToString("\n") { "${fmt.format(Date(it.t))} ${it.level} ${it.msg}" }
    Column(Modifier.fillMaxSize()) {
        Row(Modifier.padding(16.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            OutlinedButton(onClick = { copy(ctx, "log", text()) }) { Text(stringResource(R.string.copy)) }
            OutlinedButton(onClick = {
                val i = Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, text())
                ctx.startActivity(Intent.createChooser(i, null))
            }) { Text(stringResource(R.string.share)) }
            OutlinedButton(onClick = { Core.clearLogs(); lines = emptyList() }) { Text(stringResource(R.string.clear)) }
        }
        Hint(stringResource(R.string.logs_hint), modifier = Modifier.padding(horizontal = 16.dp))
        LazyColumn(Modifier.fillMaxSize().padding(horizontal = 16.dp)) {
            items(lines.asReversed()) { l ->
                Text(
                    "${fmt.format(Date(l.t))} ${l.level.take(4)} ${l.msg}",
                    fontFamily = FontFamily.Monospace,
                    style = MaterialTheme.typography.bodySmall,
                    color = if (l.level == "error" || l.level == "warning") MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurface,
                    modifier = Modifier.padding(vertical = 2.dp),
                )
            }
        }
    }
}
