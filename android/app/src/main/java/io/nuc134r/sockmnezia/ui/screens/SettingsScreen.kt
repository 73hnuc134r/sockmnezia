package io.nuc134r.sockmnezia.ui.screens

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import io.nuc134r.sockmnezia.AppPrefs
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.Settings
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.Routes
import io.nuc134r.sockmnezia.ui.Section
import io.nuc134r.sockmnezia.ui.SwitchRow
import io.nuc134r.sockmnezia.ui.act
import java.net.Inet4Address
import java.net.NetworkInterface

/** Private IPv4 addresses of this device (for LAN sharing). */
fun lanAddresses(): List<String> = runCatching {
    NetworkInterface.getNetworkInterfaces().toList().filter { it.isUp && !it.isLoopback }
        .flatMap { it.inetAddresses.toList() }
        .filterIsInstance<Inet4Address>()
        .filter { it.isSiteLocalAddress }
        .map { it.hostAddress!! }
}.getOrDefault(emptyList())

private val credentialOk = Regex("^[\\x21-\\x7e]{1,255}$")

@Composable
fun SettingsScreen(nav: NavController) {
    val ctx = LocalContext.current
    val res = LocalResources.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val prefs = remember { AppPrefs(ctx) }
    val revision by Core.revision.collectAsState()
    var loaded by remember { mutableStateOf<Settings?>(null) }
    LaunchedEffect(revision) { runCatching { loaded = Core.settings() } }
    val s = loaded ?: return

    var port by remember(s) { mutableStateOf(s.inbound.socksPort.toString()) }
    var user by remember(s) { mutableStateOf(s.inbound.user) }
    var pass by remember(s) { mutableStateOf(s.inbound.pass) }
    var showPass by remember { mutableStateOf(false) }
    var udp by remember(s) { mutableStateOf(s.inbound.udp) }
    var httpOn by remember(s) { mutableStateOf(s.inbound.httpPort != 0) }
    var httpPort by remember(s) { mutableStateOf(if (s.inbound.httpPort != 0) s.inbound.httpPort.toString() else "") }
    var listen by remember(s) { mutableStateOf(s.inbound.listen) }
    var forget by remember { mutableStateOf(false) }
    var startOnBoot by remember { mutableStateOf(prefs.startOnBoot) }

    val dirty = port != s.inbound.socksPort.toString() || user != s.inbound.user || pass != s.inbound.pass ||
        udp != s.inbound.udp || listen != s.inbound.listen ||
        (if (httpOn) httpPort else "") != (if (s.inbound.httpPort != 0) s.inbound.httpPort.toString() else "")

    Column(Modifier.verticalScroll(rememberScrollState()).padding(vertical = 8.dp)) {
        Section(stringResource(R.string.local_proxy)) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = androidx.compose.ui.Alignment.CenterVertically) {
                OutlinedTextField(
                    port, { port = it.filter(Char::isDigit).take(5) }, label = { Text(stringResource(R.string.socks_port)) },
                    singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number), modifier = Modifier.weight(1f),
                )
                OutlinedButton(onClick = { scope.act(ctx, snackbar) { port = Core.randomPort(s.inbound.httpPort).toString() } }) {
                    Text(stringResource(R.string.random))
                }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = androidx.compose.ui.Alignment.CenterVertically) {
                OutlinedTextField(user, { user = it }, label = { Text(stringResource(R.string.username)) }, singleLine = true,
                    isError = !credentialOk.matches(user), modifier = Modifier.weight(1f))
                OutlinedButton(onClick = { user = Core.randomUser() }) { Text(stringResource(R.string.random)) }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = androidx.compose.ui.Alignment.CenterVertically) {
                OutlinedTextField(
                    pass, { pass = it }, label = { Text(stringResource(R.string.password)) }, singleLine = true,
                    isError = !credentialOk.matches(pass),
                    visualTransformation = if (showPass) VisualTransformation.None else PasswordVisualTransformation(),
                    modifier = Modifier.weight(1f),
                )
                OutlinedButton(onClick = { pass = Core.randomPassword() }) { Text(stringResource(R.string.random)) }
            }
            TextButton(onClick = { showPass = !showPass }) { Text(stringResource(if (showPass) R.string.hide else R.string.show)) }
            if (!credentialOk.matches(user) || !credentialOk.matches(pass)) Hint(stringResource(R.string.credential_rule), warn = true)
            else if (pass.length < 12) Hint(stringResource(R.string.password_weak), warn = true)
            Hint(stringResource(R.string.credentials_hint))

            SwitchRow(stringResource(R.string.udp), udp, stringResource(R.string.udp_hint)) { udp = it }
            SwitchRow(stringResource(R.string.http_proxy), httpOn, stringResource(R.string.http_hint)) {
                httpOn = it
                if (it && httpPort.isBlank()) scope.act(ctx, snackbar) { httpPort = Core.randomPort(port.toIntOrNull() ?: 0).toString() }
            }
            if (httpOn) {
                OutlinedTextField(httpPort, { httpPort = it.filter(Char::isDigit).take(5) }, label = { Text(stringResource(R.string.http_port)) },
                    singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number))
            }
            val lan = remember { lanAddresses() }
            SwitchRow(
                stringResource(R.string.share_lan), listen != "127.0.0.1",
                stringResource(if (lan.isEmpty()) R.string.share_lan_none else R.string.share_lan_hint),
                enabled = lan.isNotEmpty() || listen != "127.0.0.1",
            ) { on -> listen = if (on) lan.first() else "127.0.0.1" }
            if (listen != "127.0.0.1") {
                Choice(lan.map { it to it }.ifEmpty { listOf(listen to listen) }, listen) { listen = it }
                Hint(stringResource(R.string.share_lan_warning), warn = true)
            }
            Button(enabled = dirty, onClick = {
                val ns = s.copy(
                    inbound = s.inbound.copy(
                        socksPort = port.toIntOrNull() ?: 0, user = user, pass = pass, udp = udp, listen = listen,
                        httpPort = if (httpOn) httpPort.toIntOrNull() ?: 0 else 0,
                    ),
                )
                scope.act(ctx, snackbar, ok = res.getString(R.string.saved)) { Core.setSettings(ns) }
            }) { Text(stringResource(R.string.save)) }
        }

        Section(stringResource(R.string.app_settings)) {
            SwitchRow(stringResource(R.string.start_on_boot), startOnBoot, stringResource(R.string.start_on_boot_hint)) {
                startOnBoot = it
                prefs.startOnBoot = it
            }
            SwitchRow(stringResource(R.string.fetch_via_own), s.options.fetchViaOwn, stringResource(R.string.fetch_via_own_hint)) {
                scope.act(ctx, snackbar) { Core.setSettings(s.copy(options = s.options.copy(fetchViaOwn = it))) }
            }
            Text(stringResource(R.string.log_level))
            Choice(
                listOf("error" to "error", "warning" to "warning", "info" to "info", "debug" to "debug"),
                s.options.logLevel,
            ) { lvl -> scope.act(ctx, snackbar) { Core.setSettings(s.copy(options = s.options.copy(logLevel = lvl))) } }
        }

        Section {
            listOf(
                Routes.AMNEZIA to R.string.title_amnezia,
                Routes.HEALTH to R.string.title_health,
                Routes.LOGS to R.string.title_logs,
                Routes.ABOUT to R.string.title_about,
            ).forEach { (r, label) ->
                TextButton(onClick = { nav.navigate(r) }) { Text(stringResource(label)) }
            }
            TextButton(onClick = { forget = true }) { Text(stringResource(R.string.forget)) }
        }
    }

    if (forget) {
        AlertDialog(
            onDismissRequest = { forget = false },
            title = { Text(stringResource(R.string.forget_q)) },
            text = { Text(stringResource(R.string.forget_text)) },
            confirmButton = {
                TextButton(onClick = {
                    forget = false
                    prefs.shouldRun = false
                    scope.act(ctx, snackbar, ok = res.getString(R.string.forgotten)) { Core.forget() }
                }) { Text(stringResource(R.string.delete)) }
            },
            dismissButton = { TextButton(onClick = { forget = false }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}
