package io.nuc134r.sockmnezia.ui.screens

import android.content.pm.ApplicationInfo
import android.content.pm.PackageManager
import androidx.compose.foundation.Image
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.core.graphics.drawable.toBitmap
import androidx.navigation.NavController
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.service.ProxyService
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.SwitchRow
import io.nuc134r.sockmnezia.ui.act
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

private class AppEntry(val pkg: String, val label: String, val system: Boolean, val icon: ImageBitmap?)

/**
 * Chooses the apps for VPN mode: [mode] "only" (these apps use the VPN) or "except" (all apps but these).
 * This app itself never uses its own VPN and is not listed.
 */
@Composable
fun VpnAppsScreen(nav: NavController, mode: String) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val status by Core.status.collectAsState()
    var apps by remember { mutableStateOf<List<AppEntry>?>(null) }
    val chosen = remember { mutableStateListOf<String>() }
    var query by remember { mutableStateOf("") }
    var showSystem by remember { mutableStateOf(false) }
    val iconPx = with(LocalDensity.current) { 32.dp.roundToPx() }

    LaunchedEffect(Unit) {
        val vpn = Core.settings().options.vpn
        chosen.clear()
        chosen.addAll(vpn.appList)
        apps = withContext(Dispatchers.IO) {
            val pm = ctx.packageManager
            pm.getInstalledApplications(PackageManager.GET_META_DATA)
                .filter { it.packageName != ctx.packageName }
                .map { ai ->
                    AppEntry(
                        pkg = ai.packageName,
                        label = pm.getApplicationLabel(ai).toString(),
                        system = ai.flags and ApplicationInfo.FLAG_SYSTEM != 0 && ai.flags and ApplicationInfo.FLAG_UPDATED_SYSTEM_APP == 0,
                        icon = runCatching { pm.getApplicationIcon(ai).toBitmap(iconPx, iconPx).asImageBitmap() }.getOrNull(),
                    )
                }
                .sortedWith(compareBy({ it.label.lowercase() }, { it.pkg }))
        }
    }

    Column(Modifier.fillMaxSize()) {
        Column(Modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
            Hint(stringResource(if (mode == "only") R.string.vpn_apps_only_hint else R.string.vpn_apps_except_hint))
            OutlinedTextField(query, { query = it }, label = { Text(stringResource(R.string.vpn_apps_search)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
            SwitchRow(stringResource(R.string.vpn_apps_system), showSystem) { showSystem = it }
        }
        val list = apps
        if (list == null) {
            Box(Modifier.fillMaxWidth().padding(24.dp), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
        } else {
            // Chosen apps first, so they are easy to review.
            val shown = list.filter { (showSystem || !it.system || it.pkg in chosen) && (query.isBlank() || it.label.contains(query, true) || it.pkg.contains(query, true)) }
                .sortedBy { it.pkg !in chosen }
            LazyColumn(Modifier.weight(1f)) {
                items(shown, key = { it.pkg }) { a ->
                    Row(
                        Modifier.fillMaxWidth().clickable { if (a.pkg in chosen) chosen.remove(a.pkg) else chosen.add(a.pkg) }
                            .padding(horizontal = 16.dp, vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        if (a.icon != null) Image(a.icon, null, Modifier.size(32.dp)) else Spacer(Modifier.size(32.dp))
                        Column(Modifier.weight(1f).padding(horizontal = 12.dp)) {
                            Text(a.label)
                            Text(a.pkg, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        Checkbox(checked = a.pkg in chosen, onCheckedChange = null)
                    }
                }
            }
        }
        Row(Modifier.fillMaxWidth().padding(16.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(stringResource(R.string.vpn_apps_count, chosen.size), Modifier.weight(1f))
            Button(enabled = mode != "only" || chosen.isNotEmpty(), onClick = {
                scope.act(ctx, snackbar) {
                    val s = Core.settings()
                    Core.setSettings(s.copy(options = s.options.copy(vpn = s.options.vpn.copy(apps = mode, appList = chosen.toList()))))
                    if (status.vpn) ProxyService.restart(ctx)
                    nav.popBackStack()
                }
            }) { Text(stringResource(R.string.save)) }
        }
    }
}
