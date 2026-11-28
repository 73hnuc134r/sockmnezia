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
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.FilledTonalIconButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Switch
import androidx.compose.ui.Alignment
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.path
import io.nuc134r.sockmnezia.core.Modes
import io.nuc134r.sockmnezia.ui.GroupGap
import io.nuc134r.sockmnezia.ui.GroupRow
import io.nuc134r.sockmnezia.ui.Island
import io.nuc134r.sockmnezia.ui.RowText
import io.nuc134r.sockmnezia.ui.rememberStarter
import io.nuc134r.sockmnezia.ui.theme.UiPrefs
import androidx.compose.material3.MaterialTheme
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
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import io.nuc134r.sockmnezia.AppPrefs
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.OwnServer
import io.nuc134r.sockmnezia.core.Settings
import io.nuc134r.sockmnezia.service.ProxyService
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.Routes
import io.nuc134r.sockmnezia.ui.Section
import io.nuc134r.sockmnezia.ui.act
import io.nuc134r.sockmnezia.ui.copy
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
    var servers by remember { mutableStateOf(emptyList<OwnServer>()) }
    LaunchedEffect(revision) { runCatching { servers = Core.ownServers() } }
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

    val status by Core.status.collectAsState()
    val start = rememberStarter()
    var wallpaper by remember { mutableStateOf(prefs.wallpaperColors) }

    Column(Modifier.verticalScroll(rememberScrollState()).padding(vertical = 8.dp)) {
        // Proxy (apps set up for SOCKS/HTTP) or VPN (the whole phone or chosen apps; the ports stay open).
        Section(stringResource(R.string.mode_title)) {
            SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                listOf(Modes.PROXY to R.string.mode_proxy_long, Modes.VPN to R.string.mode_vpn_long).forEachIndexed { i, (m, label) ->
                    SegmentedButton(
                        selected = status.mode == m,
                        onClick = {
                            if (status.mode != m) scope.act(ctx, snackbar) {
                                Core.setSettings(Core.settings().let { it.copy(options = it.options.copy(mode = m)) })
                                if (status.running) start() // restart in the new mode (asks for VPN consent first)
                            }
                        },
                        shape = SegmentedButtonDefaults.itemShape(i, 2),
                    ) { Text(stringResource(label), maxLines = 1) }
                }
            }
            Hint(stringResource(if (status.mode == Modes.VPN) R.string.mode_vpn_hint else R.string.mode_proxy_hint))
        }

        Section(stringResource(R.string.local_proxy)) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                OutlinedTextField(
                    port, { port = it.filter(Char::isDigit).take(5) }, label = { Text(stringResource(R.string.socks_port)) },
                    trailingIcon = {
                        IconButton(onClick = { scope.act(ctx, snackbar) { port = Core.randomPort(s.inbound.httpPort).toString() } }) {
                            Icon(DiceIcon, stringResource(R.string.random))
                        }
                    },
                    singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number), modifier = Modifier.weight(1f),
                )
                OutlinedTextField(
                    if (httpOn) httpPort else "", {
                        httpPort = it.filter(Char::isDigit).take(5)
                        httpOn = httpPort.isNotEmpty()
                    },
                    label = { Text(stringResource(R.string.http_port)) }, placeholder = { Text(stringResource(R.string.off)) },
                    singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number), modifier = Modifier.weight(1f),
                )
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                OutlinedTextField(user, { user = it }, label = { Text(stringResource(R.string.username)) }, singleLine = true,
                    isError = !credentialOk.matches(user), modifier = Modifier.weight(1f))
                FilledTonalIconButton(onClick = { user = Core.randomUser() }) { Icon(DiceIcon, stringResource(R.string.random)) }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                OutlinedTextField(
                    pass, { pass = it }, label = { Text(stringResource(R.string.password)) }, singleLine = true,
                    isError = !credentialOk.matches(pass),
                    visualTransformation = if (showPass) VisualTransformation.None else PasswordVisualTransformation(),
                    trailingIcon = {
                        IconButton(onClick = { showPass = !showPass }) {
                            Icon(EyeIcon, stringResource(if (showPass) R.string.hide else R.string.show))
                        }
                    },
                    modifier = Modifier.weight(1f),
                )
                FilledTonalIconButton(onClick = { pass = Core.randomPassword() }) { Icon(DiceIcon, stringResource(R.string.random)) }
            }
            if (!credentialOk.matches(user) || !credentialOk.matches(pass)) Hint(stringResource(R.string.credential_rule), warn = true)
            else if (pass.length < 12) Hint(stringResource(R.string.password_weak), warn = true)
            Hint(stringResource(R.string.credentials_hint))

            Column(verticalArrangement = Arrangement.spacedBy(GroupGap)) {
                val lan = remember { lanAddresses() }
                GroupRow(0, 2) {
                    RowText(stringResource(R.string.udp), Modifier.weight(1f), subtitle = stringResource(R.string.udp_hint))
                    Switch(udp, { udp = it })
                }
                GroupRow(1, 2) {
                    RowText(
                        stringResource(R.string.share_lan), Modifier.weight(1f),
                        subtitle = stringResource(if (lan.isEmpty()) R.string.share_lan_none else R.string.share_lan_hint),
                    )
                    Switch(listen != "127.0.0.1", { on -> listen = if (on) lan.first() else "127.0.0.1" }, enabled = lan.isNotEmpty() || listen != "127.0.0.1")
                }
                if (listen != "127.0.0.1") {
                    Choice(lan.map { it to it }.ifEmpty { listOf(listen to listen) }, listen) { listen = it }
                    Hint(stringResource(R.string.share_lan_warning), warn = true)
                }
            }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp, Alignment.End)) {
                OutlinedButton(onClick = {
                    copy(ctx, "proxy", "socks5://${s.inbound.user}:${s.inbound.pass}@${s.inbound.listen}:${s.inbound.socksPort}", sensitive = true)
                }) { Text(stringResource(R.string.copy_link)) }
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
        }

        VpnSettings(nav, s)

        Section(stringResource(R.string.app_settings)) {
            Column(verticalArrangement = Arrangement.spacedBy(GroupGap)) {
                val wallpaperAvailable = android.os.Build.VERSION.SDK_INT >= 31
                val n = if (wallpaperAvailable) 2 else 1
                GroupRow(0, n) {
                    RowText(stringResource(R.string.start_on_boot), Modifier.weight(1f), subtitle = stringResource(R.string.start_on_boot_hint))
                    Switch(startOnBoot, {
                        startOnBoot = it
                        prefs.startOnBoot = it
                    })
                }
                if (wallpaperAvailable) GroupRow(1, n) {
                    RowText(stringResource(R.string.wallpaper_colors), Modifier.weight(1f), subtitle = stringResource(R.string.wallpaper_colors_hint))
                    Switch(wallpaper, {
                        wallpaper = it
                        prefs.wallpaperColors = it
                        UiPrefs.wallpaperColors.value = it
                    })
                }
            }
            if (servers.isNotEmpty()) {
                Text(stringResource(R.string.fetch_via))
                Choice(
                    listOf("" to stringResource(R.string.fetch_via_direct)) + servers.map { it.id to it.name },
                    s.options.fetchVia,
                ) { id -> scope.act(ctx, snackbar) { Core.setSettings(s.copy(options = s.options.copy(fetchVia = id))) } }
                Hint(stringResource(R.string.fetch_via_hint))
            }
            Text(stringResource(R.string.log_level))
            Choice(
                listOf("error" to "error", "warning" to "warning", "info" to "info", "debug" to "debug"),
                s.options.logLevel,
            ) { lvl -> scope.act(ctx, snackbar) { Core.setSettings(s.copy(options = s.options.copy(logLevel = lvl))) } }
        }

        Island {
            val tiles = listOf(
                Routes.AMNEZIA to R.string.title_amnezia,
                Routes.HEALTH to R.string.title_health,
                Routes.LOGS to R.string.title_logs,
                Routes.ABOUT to R.string.title_about,
            )
            tiles.forEachIndexed { i, (r, label) ->
                GroupRow(i, tiles.size, onClick = { nav.navigate(r) }) {
                    Text(stringResource(label), style = MaterialTheme.typography.bodyLarge, modifier = Modifier.weight(1f))
                    Icon(Icons.AutoMirrored.Filled.KeyboardArrowRight, null, tint = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }
        OutlinedButton(
            onClick = { forget = true },
            border = androidx.compose.foundation.BorderStroke(1.dp, MaterialTheme.colorScheme.error),
            colors = ButtonDefaults.outlinedButtonColors(contentColor = MaterialTheme.colorScheme.error),
            modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
        ) { Text(stringResource(R.string.forget)) }
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

/** Settings → VPN: DNS, local network and which apps use the VPN (D31). Saving restarts a running VPN, whose
 * interface (routes, DNS, apps) is fixed when it is built. */
@Composable
private fun VpnSettings(nav: NavController, s: Settings) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val status by Core.status.collectAsState()
    val v = s.options.vpn
    fun save(nv: io.nuc134r.sockmnezia.core.VpnOptions) = scope.act(ctx, snackbar) {
        Core.setSettings(s.copy(options = s.options.copy(vpn = nv)))
        if (status.vpn) ProxyService.restart(ctx)
    }
    Section(stringResource(R.string.vpn_settings)) {
        VpnProtectionCheck()
        Text(stringResource(R.string.vpn_dns))
        Choice(
            listOf("public" to stringResource(R.string.vpn_dns_public), "doh" to stringResource(R.string.vpn_dns_doh), "custom" to stringResource(R.string.vpn_dns_custom)),
            v.dns,
        ) { if (it != "custom") save(v.copy(dns = it)) else save(v.copy(dns = it, dnsServers = v.dnsServers.ifEmpty { listOf("9.9.9.9") })) }
        Hint(stringResource(when (v.dns) { "doh" -> R.string.vpn_dns_doh_hint; "custom" -> R.string.vpn_dns_custom_hint; else -> R.string.vpn_dns_public_hint }))
        if (v.dns != "doh" && status.hops.any { it.protocol == "http" || it.protocol == "socks" }) {
            Hint(stringResource(R.string.vpn_dns_udp_warning), warn = true)
        }
        if (v.dns == "custom") {
            var servers by remember(v) { mutableStateOf(v.dnsServers.joinToString(", ")) }
            OutlinedTextField(servers, { servers = it }, label = { Text(stringResource(R.string.vpn_dns_servers)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
            TextButton(onClick = { save(v.copy(dnsServers = servers.split(',', ' ', ';').map { it.trim() }.filter { it.isNotEmpty() })) }) {
                Text(stringResource(R.string.save))
            }
        }
        Text(stringResource(R.string.vpn_lan))
        Choice(listOf("bypass" to stringResource(R.string.vpn_lan_bypass), "tunnel" to stringResource(R.string.vpn_lan_tunnel)), v.lan) { save(v.copy(lan = it)) }
        Hint(stringResource(if (v.lan == "tunnel") R.string.vpn_lan_tunnel_hint else R.string.vpn_lan_bypass_hint))
        Text(stringResource(R.string.vpn_apps))
        Choice(
            listOf("all" to stringResource(R.string.vpn_apps_all), "only" to stringResource(R.string.vpn_apps_only), "except" to stringResource(R.string.vpn_apps_except)),
            v.apps,
        ) {
            if (it == "only" && v.appList.isEmpty()) nav.navigate(Routes.vpnApps("only")) else save(v.copy(apps = it))
        }
        if (v.apps != "all") {
            OutlinedButton(onClick = { nav.navigate(Routes.vpnApps(v.apps)) }) {
                Text(pluralStringResource(R.plurals.vpn_apps_chosen, v.appList.size, v.appList.size))
            }
        }
    }
}

/** Android's own VPN protection (Always-on, "Block connections without VPN"): without the second, apps reach
 * the internet directly whenever this VPN isn't up (stopped, killed, after a reboot). Shown, never changed:
 * only the user can set it, in Android's VPN settings. */
@Composable
private fun VpnProtectionCheck() {
    val ctx = LocalContext.current
    val p by ProxyService.vpnProtection.collectAsState()
    Text(stringResource(R.string.vpn_protection))
    when {
        p == null -> Hint(stringResource(R.string.vpn_protection_unknown))
        p!!.lockdown -> Hint(stringResource(R.string.vpn_protection_ok))
        p!!.alwaysOn -> Hint(stringResource(R.string.vpn_protection_no_block), warn = true)
        else -> Hint(stringResource(R.string.vpn_protection_off), warn = true)
    }
    OutlinedButton(onClick = {
        runCatching { ctx.startActivity(android.content.Intent(android.provider.Settings.ACTION_VPN_SETTINGS)) }
    }) { Text(stringResource(R.string.vpn_protection_open)) }
}

/** Material "casino" (a die), for "random". */
val DiceIcon: ImageVector by lazy {
    ImageVector.Builder("Dice", 24.dp, 24.dp, 24f, 24f).apply {
        path(fill = SolidColor(Color.Black)) {
            moveTo(19f, 3f); horizontalLineTo(5f); curveTo(3.9f, 3f, 3f, 3.9f, 3f, 5f); verticalLineTo(19f)
            curveTo(3f, 20.1f, 3.9f, 21f, 5f, 21f); horizontalLineTo(19f); curveTo(20.1f, 21f, 21f, 20.1f, 21f, 19f)
            verticalLineTo(5f); curveTo(21f, 3.9f, 20.1f, 3f, 19f, 3f); close()
            moveTo(7.5f, 18f); curveTo(6.67f, 18f, 6f, 17.33f, 6f, 16.5f); curveTo(6f, 15.67f, 6.67f, 15f, 7.5f, 15f)
            curveTo(8.33f, 15f, 9f, 15.67f, 9f, 16.5f); curveTo(9f, 17.33f, 8.33f, 18f, 7.5f, 18f); close()
            moveTo(7.5f, 9f); curveTo(6.67f, 9f, 6f, 8.33f, 6f, 7.5f); curveTo(6f, 6.67f, 6.67f, 6f, 7.5f, 6f)
            curveTo(8.33f, 6f, 9f, 6.67f, 9f, 7.5f); curveTo(9f, 8.33f, 8.33f, 9f, 7.5f, 9f); close()
            moveTo(12f, 13.5f); curveTo(11.17f, 13.5f, 10.5f, 12.83f, 10.5f, 12f); curveTo(10.5f, 11.17f, 11.17f, 10.5f, 12f, 10.5f)
            curveTo(12.83f, 10.5f, 13.5f, 11.17f, 13.5f, 12f); curveTo(13.5f, 12.83f, 12.83f, 13.5f, 12f, 13.5f); close()
            moveTo(16.5f, 18f); curveTo(15.67f, 18f, 15f, 17.33f, 15f, 16.5f); curveTo(15f, 15.67f, 15.67f, 15f, 16.5f, 15f)
            curveTo(17.33f, 15f, 18f, 15.67f, 18f, 16.5f); curveTo(18f, 17.33f, 17.33f, 18f, 16.5f, 18f); close()
            moveTo(16.5f, 9f); curveTo(15.67f, 9f, 15f, 8.33f, 15f, 7.5f); curveTo(15f, 6.67f, 15.67f, 6f, 16.5f, 6f)
            curveTo(17.33f, 6f, 18f, 6.67f, 18f, 7.5f); curveTo(18f, 8.33f, 17.33f, 9f, 16.5f, 9f); close()
        }
    }.build()
}

/** Material "visibility" (an eye), for show/hide password. */
val EyeIcon: ImageVector by lazy {
    ImageVector.Builder("Eye", 24.dp, 24.dp, 24f, 24f).apply {
        addPath(
            androidx.compose.ui.graphics.vector.PathParser().parsePathString(
                "M12,4.5C7,4.5 2.73,7.61 1,12c1.73,4.39 6,7.5 11,7.5s9.27,-3.11 11,-7.5c-1.73,-4.39 -6,-7.5 -11,-7.5z" +
                    "M12,17c-2.76,0 -5,-2.24 -5,-5s2.24,-5 5,-5 5,2.24 5,5 -2.24,5 -5,5z" +
                    "M12,9c-1.66,0 -3,1.34 -3,3s1.34,3 3,3 3,-1.34 3,-3 -1.34,-3 -3,-3z",
            ).toNodes(),
            fill = SolidColor(Color.Black),
        )
    }.build()
}
