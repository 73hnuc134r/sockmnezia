package io.nuc134r.sockmnezia.ui.screens

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.CoreError
import io.nuc134r.sockmnezia.core.Endpoint
import io.nuc134r.sockmnezia.ui.Hint
import io.nuc134r.sockmnezia.ui.LocalSnackbar
import io.nuc134r.sockmnezia.ui.Routes
import io.nuc134r.sockmnezia.ui.Section
import io.nuc134r.sockmnezia.ui.SwitchRow
import io.nuc134r.sockmnezia.ui.act
import io.nuc134r.sockmnezia.ui.errorText
import io.nuc134r.sockmnezia.ui.paste

private val Protocols = listOf("vless", "vmess", "trojan", "shadowsocks", "socks", "http")
private val Transports = listOf("tcp", "ws", "grpc", "httpupgrade", "xhttp")
private val Fingerprints = listOf("", "chrome", "firefox", "safari", "ios", "android", "edge", "qq", "random", "randomized")
private val SsMethods = listOf(
    "2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305",
    "aes-128-gcm", "aes-256-gcm", "chacha20-ietf-poly1305", "xchacha20-ietf-poly1305",
)
private val VmessSecurity = listOf("auto", "aes-128-gcm", "chacha20-poly1305", "none", "zero")
private val XhttpModes = listOf("auto", "packet-up", "stream-up", "stream-one")

/**
 * Endpoint editor for a saved server (or a new one, id = Routes.NEW): editable fields, or the link / Xray
 * outbound JSON as text. Both go through the core's parser, so a saved server is valid whichever way it was
 * edited; editing keeps the server's ID, so chains that use it follow.
 */
@Composable
fun ServerEditScreen(nav: NavController, id: String) {
    val ctx = LocalContext.current
    val res = LocalResources.current
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    val isNew = id == Routes.NEW

    var ep by remember { mutableStateOf(Endpoint()) }
    var text by remember { mutableStateOf("") }
    var fieldsMode by remember { mutableStateOf(true) }
    var jsonOnly by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<CoreError?>(null) }
    LaunchedEffect(id) {
        if (!isNew) {
            runCatching { Core.serverEdit(id) }.getOrNull()?.let { e ->
                text = e.link
                if (e.endpoint != null) ep = e.endpoint else { jsonOnly = true; fieldsMode = false }
            }
        }
    }

    /** Switches between fields and text, converting through the core; stays put when that fails. */
    fun switchMode(toFields: Boolean) {
        if (toFields == fieldsMode) return
        error = null
        try {
            if (toFields) {
                ep = Core.linkToEndpoint(text)
                jsonOnly = false
            } else {
                text = Core.endpointToLink(ep)
            }
            fieldsMode = toFields
        } catch (e: Exception) {
            val ce = Core.asCoreException(e).error
            if (ce.code == "json_only") jsonOnly = true
            error = ce
        }
    }

    fun save() {
        error = null
        val link = try {
            if (fieldsMode) Core.endpointToLink(ep) else text.trim()
        } catch (e: Exception) {
            error = Core.asCoreException(e).error
            return
        }
        scope.act(ctx, snackbar, ok = res.getString(R.string.srv_saved), onError = { error = it; true }) {
            if (isNew) Core.addOwnServer(link, "") else Core.updateOwnServer(id, link)
            nav.popBackStack()
        }
    }

    Column(Modifier.verticalScroll(rememberScrollState()).padding(vertical = 8.dp)) {
        SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp)) {
            SegmentedButton(selected = fieldsMode, onClick = { switchMode(true) }, shape = SegmentedButtonDefaults.itemShape(0, 2)) {
                Text(stringResource(R.string.srv_mode_fields))
            }
            SegmentedButton(selected = !fieldsMode, onClick = { switchMode(false) }, shape = SegmentedButtonDefaults.itemShape(1, 2)) {
                Text(stringResource(R.string.srv_mode_link))
            }
        }
        if (jsonOnly) Section { Hint(stringResource(R.string.srv_json_only), warn = true) }

        if (fieldsMode) {
            EndpointFields(ep) { ep = it; error = null }
        } else {
            Section {
                OutlinedTextField(
                    text, { text = it; error = null }, label = { Text(stringResource(R.string.server_link)) },
                    minLines = 6, textStyle = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace),
                    modifier = Modifier.fillMaxWidth(),
                )
                OutlinedButton(onClick = { text = paste(ctx); error = null }) { Text(stringResource(R.string.paste)) }
                Hint(stringResource(R.string.srv_link_hint))
                Hint(stringResource(R.string.server_link_hint))
            }
        }

        error?.let { Section { Hint(errorText(ctx, it), warn = true) } }
        Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            Spacer(Modifier.weight(1f))
            Button(onClick = { save() }) { Text(stringResource(R.string.save)) }
        }
    }
}

/** The editable fields, showing only what applies to the protocol, transport and security chosen. */
@Composable
private fun EndpointFields(ep: Endpoint, onChange: (Endpoint) -> Unit) {
    val none = stringResource(R.string.srv_none)
    val default = stringResource(R.string.srv_default)
    val tls = stringResource(R.string.srv_tls)
    val reality = stringResource(R.string.srv_reality)
    Section(stringResource(R.string.srv_section_server)) {
        Field(stringResource(R.string.srv_name), ep.name) { onChange(ep.copy(name = it)) }
        Labeled(stringResource(R.string.srv_protocol)) {
            Choice(Protocols.map { it to it }, ep.protocol) { p ->
                // Reality is vless-only in practice; socks/http/ss have no transport settings here.
                onChange(ep.copy(protocol = p, security = if (p in setOf("socks", "http", "shadowsocks")) "none" else ep.security))
            }
        }
        Field(stringResource(R.string.srv_address), ep.address) { onChange(ep.copy(address = it.trim())) }
        Field(stringResource(R.string.srv_port), if (ep.port == 0) "" else ep.port.toString(), number = true) {
            onChange(ep.copy(port = it.filter(Char::isDigit).take(5).toIntOrNull() ?: 0))
        }
        when (ep.protocol) {
            "vless" -> {
                Field(stringResource(R.string.srv_uuid), ep.id, mono = true) { onChange(ep.copy(id = it.trim())) }
                Labeled(stringResource(R.string.srv_flow)) {
                    Choice(listOf("" to none, "xtls-rprx-vision" to "xtls-rprx-vision"), ep.flow) { onChange(ep.copy(flow = it)) }
                }
            }
            "vmess" -> {
                Field(stringResource(R.string.srv_uuid), ep.id, mono = true) { onChange(ep.copy(id = it.trim())) }
                Labeled(stringResource(R.string.srv_vmess_security)) {
                    Choice(VmessSecurity.map { it to it }, ep.encryption.ifBlank { "auto" }) { onChange(ep.copy(encryption = it)) }
                }
            }
            "trojan" -> Field(stringResource(R.string.password), ep.password, mono = true) { onChange(ep.copy(password = it)) }
            "shadowsocks" -> {
                Labeled(stringResource(R.string.srv_method)) {
                    Choice((SsMethods + listOfNotNull(ep.method.takeIf { it.isNotBlank() && it !in SsMethods })).map { it to it }, ep.method) {
                        onChange(ep.copy(method = it))
                    }
                }
                Field(stringResource(R.string.password), ep.password, mono = true) { onChange(ep.copy(password = it)) }
            }
            "socks", "http" -> {
                Field(stringResource(R.string.username), ep.user) { onChange(ep.copy(user = it)) }
                Field(stringResource(R.string.password), ep.password, mono = true) { onChange(ep.copy(password = it)) }
                if (ep.protocol == "http") {
                    SwitchRow(stringResource(R.string.srv_tls), ep.security == "tls") { onChange(ep.copy(security = if (it) "tls" else "none")) }
                }
            }
        }
    }
    if (ep.protocol !in setOf("vless", "vmess", "trojan")) return

    Section(stringResource(R.string.srv_section_transport)) {
        Labeled(stringResource(R.string.srv_transport)) {
            Choice(Transports.map { it to it }, ep.network.ifBlank { "tcp" }) { onChange(ep.copy(network = it)) }
        }
        when (ep.network.ifBlank { "tcp" }) {
            "ws", "httpupgrade" -> {
                Field(stringResource(R.string.srv_path), ep.path) { onChange(ep.copy(path = it)) }
                Field(stringResource(R.string.srv_host), ep.host) { onChange(ep.copy(host = it)) }
            }
            "grpc" -> {
                Field(stringResource(R.string.srv_service), ep.serviceName) { onChange(ep.copy(serviceName = it)) }
                Field(stringResource(R.string.srv_authority), ep.authority) { onChange(ep.copy(authority = it)) }
                SwitchRow(stringResource(R.string.srv_grpc_multi), ep.mode == "multi") { onChange(ep.copy(mode = if (it) "multi" else "")) }
            }
            "xhttp" -> {
                Field(stringResource(R.string.srv_path), ep.path) { onChange(ep.copy(path = it)) }
                Field(stringResource(R.string.srv_host), ep.host) { onChange(ep.copy(host = it)) }
                Labeled(stringResource(R.string.srv_xhttp_mode)) {
                    Choice(XhttpModes.map { it to it }, ep.mode.ifBlank { "auto" }) { onChange(ep.copy(mode = it)) }
                }
                Field(stringResource(R.string.srv_extra), ep.extra, mono = true, single = false) { onChange(ep.copy(extra = it)) }
            }
            else -> {
                SwitchRow(stringResource(R.string.srv_http_header), ep.headerType == "http") {
                    onChange(ep.copy(headerType = if (it) "http" else ""))
                }
                if (ep.headerType == "http") {
                    Field(stringResource(R.string.srv_path), ep.path) { onChange(ep.copy(path = it)) }
                    Field(stringResource(R.string.srv_host), ep.host) { onChange(ep.copy(host = it)) }
                }
            }
        }
        val securities = buildList {
            add("none" to none); add("tls" to tls)
            if (ep.protocol != "vmess") add("reality" to reality)
        }
        Labeled(stringResource(R.string.srv_security)) {
            Choice(securities, ep.security.ifBlank { "none" }) { onChange(ep.copy(security = it)) }
        }
        if (ep.security == "tls" || ep.security == "reality") {
            Field(stringResource(R.string.srv_sni), ep.sni) { onChange(ep.copy(sni = it.trim())) }
            Labeled(stringResource(R.string.srv_fp)) {
                Choice(Fingerprints.map { it to it.ifBlank { default } }, ep.fp) { onChange(ep.copy(fp = it)) }
            }
        }
        if (ep.security == "tls") {
            Field(stringResource(R.string.srv_alpn), ep.alpn) { onChange(ep.copy(alpn = it.trim())) }
        }
        if (ep.security == "reality") {
            Field(stringResource(R.string.srv_pbk), ep.pbk, mono = true) { onChange(ep.copy(pbk = it.trim())) }
            Field(stringResource(R.string.srv_sid), ep.sid, mono = true) { onChange(ep.copy(sid = it.trim())) }
            Field(stringResource(R.string.srv_spx), ep.spx) { onChange(ep.copy(spx = it)) }
        }
    }
}

@Composable
private fun Field(label: String, value: String, mono: Boolean = false, number: Boolean = false, single: Boolean = true, onChange: (String) -> Unit) {
    OutlinedTextField(
        value, onChange, label = { Text(label) }, singleLine = single,
        textStyle = if (mono) MaterialTheme.typography.bodyMedium.copy(fontFamily = FontFamily.Monospace) else MaterialTheme.typography.bodyLarge,
        keyboardOptions = if (number) KeyboardOptions(keyboardType = KeyboardType.Number) else KeyboardOptions.Default,
        modifier = Modifier.fillMaxWidth(),
    )
}

@Composable
private fun Labeled(label: String, content: @Composable () -> Unit) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(label, style = MaterialTheme.typography.bodyLarge, modifier = Modifier.weight(1f))
        content()
    }
}
