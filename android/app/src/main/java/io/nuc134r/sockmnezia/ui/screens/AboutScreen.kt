package io.nuc134r.sockmnezia.ui.screens

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import io.nuc134r.sockmnezia.AppPrefs
import io.nuc134r.sockmnezia.BuildConfig
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.UpdateInfo
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.Section
import io.nuc134r.sockmnezia.ui.SwitchRow
import io.nuc134r.sockmnezia.ui.act

const val REPO_URL = "https://github.com/73hnuc134r/sockmnezia"

@Composable
fun AboutScreen() {
    val ctx = LocalContext.current
    val res = LocalResources.current
    val uri = LocalUriHandler.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val prefs = remember { AppPrefs(ctx) }
    val xray = remember { runCatching { Core.xrayVersion() }.getOrDefault("?") }
    var checkUpdates by remember { mutableStateOf(prefs.checkUpdates) }
    var beta by remember { mutableStateOf(prefs.betaUpdates) }
    var busy by remember { mutableStateOf(false) }
    Column(Modifier.verticalScroll(rememberScrollState()).padding(vertical = 8.dp)) {
        Section(stringResource(R.string.app_name)) {
            Text(stringResource(R.string.version, BuildConfig.VERSION_NAME))
            Text("Xray-core $xray")
            Hint(stringResource(R.string.unofficial))
            TextButton(onClick = { uri.openUri(REPO_URL) }) { Text(REPO_URL.removePrefix("https://")) }
        }
        Section(stringResource(R.string.updates)) {
            SwitchRow(stringResource(R.string.check_updates), checkUpdates, stringResource(R.string.check_updates_hint)) {
                checkUpdates = it
                prefs.checkUpdates = it
            }
            SwitchRow(stringResource(R.string.beta_updates), beta) {
                beta = it
                prefs.betaUpdates = it
            }
            OutlinedButton(enabled = !busy, onClick = {
                busy = true
                scope.act(ctx, snackbar) {
                    try {
                        val u = Core.checkUpdate(BuildConfig.VERSION_NAME, beta)
                        if (!u.newer) snackbar.showSnackbar(res.getString(R.string.up_to_date), withDismissAction = true)
                    } finally { busy = false }
                }
            }) { Text(stringResource(R.string.check_now)) }
            UpdateCard()
        }
        Section(stringResource(R.string.licenses)) {
            Hint(stringResource(R.string.licenses_text))
        }
    }
}

/** Shown on Home and About when a newer release exists. */
@Composable
fun UpdateCard() {
    val uri = LocalUriHandler.current
    val u: UpdateInfo? by Core.update.collectAsState()
    val info = u?.takeIf { it.newer } ?: return
    Text(stringResource(R.string.update_available, info.version))
    if (info.notes.isNotBlank()) Hint(info.notes.take(800))
    TextButton(onClick = { uri.openUri(info.url) }) { Text(stringResource(R.string.open_release)) }
}
