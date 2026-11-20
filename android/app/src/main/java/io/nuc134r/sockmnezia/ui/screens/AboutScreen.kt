package io.nuc134r.sockmnezia.ui.screens

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import io.nuc134r.sockmnezia.BuildConfig
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.Section

const val REPO_URL = "https://github.com/nuc134r/sockmnezia"

@Composable
fun AboutScreen() {
    val uri = LocalUriHandler.current
    val xray = remember { runCatching { Core.xrayVersion() }.getOrDefault("?") }
    Column(Modifier.verticalScroll(rememberScrollState()).padding(vertical = 8.dp)) {
        Section(stringResource(R.string.app_name)) {
            Text(stringResource(R.string.version, BuildConfig.VERSION_NAME))
            Text("Xray-core $xray")
            Hint(stringResource(R.string.unofficial))
            TextButton(onClick = { uri.openUri(REPO_URL) }) { Text(REPO_URL.removePrefix("https://")) }
        }
        Section(stringResource(R.string.licenses)) {
            Hint(stringResource(R.string.licenses_text))
        }
    }
}
