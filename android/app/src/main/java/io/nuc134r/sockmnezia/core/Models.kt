package io.nuc134r.sockmnezia.core

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

/** JSON shared with the Go core (core/engine, core/mobile). Field names follow the Go side. */
val CoreJson = Json {
    ignoreUnknownKeys = true
    encodeDefaults = true
    explicitNulls = false
    coerceInputValues = true
}

/** Hop kinds of a chain (core/engine/state.go). */
object HopKind {
    const val AMNEZIA = "amnezia"
    const val SERVER = "server"
}

@Serializable
data class Captcha(
    @SerialName("captcha_id") val id: String = "",
    @SerialName("captcha_image") val imageBase64: String = "",
    val hint: String? = null,
)

@Serializable
data class CoreError(
    val code: String = "internal",
    val field: String? = null,
    val detail: String? = null,
    val captcha: Captcha? = null,
)

class CoreException(val error: CoreError) : Exception(error.detail ?: error.code)

@Serializable
data class Status(
    val running: Boolean = false,
    /** Running in VPN mode (the whole phone or chosen apps through the tun). */
    val vpn: Boolean = false,
    /** The configured mode, [Modes.PROXY] or [Modes.VPN]. */
    val mode: String = Modes.PROXY,
    val listen: String = "127.0.0.1",
    @SerialName("socks_port") val socksPort: Int = 0,
    @SerialName("http_port") val httpPort: Int = 0,
    @SerialName("started_at") val startedAt: String? = null,
    val up: Long = 0,
    val down: Long = 0,
    @SerialName("auth_failures") val authFailures: Long = 0,
    @SerialName("has_key") val hasKey: Boolean = false,
    @SerialName("has_gateway_key") val hasGatewayKey: Boolean = false,
    @SerialName("has_config") val hasConfig: Boolean = false,
    val country: String = "",
    @SerialName("country_name") val countryName: String = "",
    @SerialName("fetched_at") val fetchedAt: String? = null,
    @SerialName("subscription_to") val subscriptionTo: String? = null,
    @SerialName("own_server") val ownServer: String? = null,
    @SerialName("chain_id") val chainId: String? = null,
    /** The single server selected instead of a chain. */
    @SerialName("server_id") val serverId: String? = null,
    @SerialName("chain_name") val chainName: String? = null,
    val hops: List<HopInfo> = emptyList(),
    /** 1-based position of Amnezia in the active chain, 0 when the chain has no Amnezia hop. */
    @SerialName("amnezia_hop") val amneziaHop: Int = 0,
    @SerialName("chain_error") val chainError: CoreError? = null,
    @SerialName("device_id") val deviceId: String = "",
    val chain: List<String> = emptyList(),
    @SerialName("chain_countries") val chainCountries: List<String> = emptyList(),
    @SerialName("last_error") val lastError: CoreError? = null,
)

@Serializable
data class Inbound(
    val listen: String = "127.0.0.1",
    @SerialName("socks_port") val socksPort: Int = 0,
    @SerialName("http_port") val httpPort: Int = 0,
    val user: String = "",
    val pass: String = "",
    val udp: Boolean = false,
)

@Serializable
data class Rule(
    val id: String? = null,
    val remarks: String? = null,
    val domain: List<String>? = null,
    val ip: List<String>? = null,
    val port: String? = null,
    val network: String? = null,
    val protocol: List<String>? = null,
    val outboundTag: String = "proxy",
    val enabled: Boolean? = null,
    val locked: Boolean = false,
) {
    val isEnabled: Boolean get() = enabled ?: true
}

@Serializable
data class Routing(
    val domainStrategy: String = "AsIs",
    val rules: List<Rule> = emptyList(),
)

@Serializable
data class Options(
    /** Own server ID the gateway request goes through when no hop precedes Amnezia ("" = direct). */
    @SerialName("fetch_via") val fetchVia: String = "",
    @SerialName("geo_source") val geoSource: String = "runetfreedom",
    @SerialName("geo_custom_url") val geoCustomUrl: String = "",
    @SerialName("geo_auto_update") val geoAutoUpdate: Boolean = false,
    @SerialName("log_level") val logLevel: String = "warning",
    @SerialName("exit_ip_url") val exitIpUrl: String = "https://api.ipify.org",
    @SerialName("refresh_after_hrs") val refreshAfterHrs: Int = 24,
    /** [Modes.PROXY] or [Modes.VPN]: what Start starts. */
    val mode: String = Modes.PROXY,
    val vpn: VpnOptions = VpnOptions(),
)

object Modes {
    const val PROXY = "proxy"
    const val VPN = "vpn"
}

/** VPN-mode settings (core engine.VPNOptions). */
@Serializable
data class VpnOptions(
    /** "public" | "doh" | "custom" */
    val dns: String = "public",
    @SerialName("dns_servers") val dnsServers: List<String> = emptyList(),
    /** "bypass" | "tunnel" */
    val lan: String = "bypass",
    /** "all" | "only" | "except" */
    val apps: String = "all",
    @SerialName("app_list") val appList: List<String> = emptyList(),
)

/** Parameters for VpnService.Builder, computed by the core (engine.VPNInterface). */
@Serializable
data class VpnInterface(
    val mtu: Int = 1500,
    val addresses: List<String> = emptyList(),
    val routes: List<String> = emptyList(),
    val dns: List<String> = emptyList(),
    val apps: String = "all",
    @SerialName("app_list") val appList: List<String> = emptyList(),
)

@Serializable
data class Settings(
    val inbound: Inbound = Inbound(),
    val routing: Routing = Routing(),
    val options: Options = Options(),
)

@Serializable
data class Country(
    @SerialName("server_country_code") val code: String = "",
    @SerialName("server_country_name") val name: String = "",
)

@Serializable
data class OwnServer(
    val id: String,
    val name: String = "",
    val protocol: String = "",
    val address: String = "",
    val port: Int = 0,
    val network: String = "",
    val security: String = "",
    /** Names of the chains that use this server (it cannot be deleted while any does). */
    @SerialName("used_in") val usedIn: List<String> = emptyList(),
    /** Selected on its own (not as part of a chain). */
    val active: Boolean = false,
    /** Country of [address] when it is an IP address (geo database; names are never resolved). */
    @SerialName("host_country") val hostCountry: String = "",
    /** The last Test of this server. */
    val last: TestResult? = null,
) {
    /** "vless · tcp/reality" */
    val detail: String get() = listOf(protocol, "$network/$security").filter { it.isNotBlank() && it != "/" }.joinToString(" · ")
}

/** Progress of a running geo update: file [index] of [count], [done] of [total] bytes (-1 = unknown). */
@Serializable
data class GeoProgress(
    val file: String = "",
    val index: Int = 0,
    val count: Int = 2,
    val done: Long = 0,
    val total: Long = -1,
    val verifying: Boolean = false,
) {
    /** 0..1 over both files, or null while the size is unknown. */
    val fraction: Float?
        get() = if (total > 0 && index > 0) ((index - 1) + done.toFloat() / total) / count else null
}

/** An own server as editable fields (core links.Endpoint). Fields that don't apply are ignored by the core. */
@Serializable
data class Endpoint(
    val name: String = "",
    val protocol: String = "vless",
    val address: String = "",
    val port: Int = 443,
    val id: String = "",
    val password: String = "",
    val user: String = "",
    val method: String = "",
    val flow: String = "",
    val encryption: String = "",
    val network: String = "tcp",
    val security: String = "tls",
    val sni: String = "",
    val fp: String = "",
    val alpn: String = "",
    val pbk: String = "",
    val sid: String = "",
    val spx: String = "",
    val path: String = "",
    val host: String = "",
    @SerialName("service_name") val serviceName: String = "",
    val mode: String = "",
    @SerialName("header_type") val headerType: String = "",
    val authority: String = "",
    val extra: String = "",
)

/** A saved server for the editor: its link (or outbound JSON) and fields (null: JSON only). */
@Serializable
data class ServerEdit(val link: String = "", val endpoint: Endpoint? = null)

/** The last Test of a saved chain or server (kept by the core across restarts). */
@Serializable
data class TestResult(
    val ms: Long = 0,
    val ip: String = "",
    val country: String = "",
    /** Error code when the test failed. */
    val error: String? = null,
) {
    val ok: Boolean get() = error.isNullOrBlank() && ms > 0
}

/** Progress of an app update download. */
@Serializable
data class DownloadProgress(val done: Long = 0, val total: Long = -1, val verifying: Boolean = false)

/** What importing shared text adds (or would add, in a dry run). */
@Serializable
data class ImportResult(
    val chain: String = "",
    @SerialName("chain_id") val chainId: String = "",
    val hops: Int = 0,
    @SerialName("new_servers") val newServers: List<String> = emptyList(),
    @SerialName("known_servers") val knownServers: List<String> = emptyList(),
    @SerialName("needs_amnezia") val needsAmnezia: Boolean = false,
)

/** One hop of a saved chain, as stored: Amnezia, or an own server by ID. */
@Serializable
data class ChainHop(val kind: String, val server: String? = null)

/** A chain to save (no [id]: a new one). */
@Serializable
data class Chain(val id: String? = null, val name: String = "", val hops: List<ChainHop> = emptyList())

/** A hop as the UI shows it (no credentials). */
@Serializable
data class HopInfo(
    val kind: String = HopKind.SERVER,
    val server: String? = null,
    val name: String = "",
    val country: String = "",
    val protocol: String = "",
    val network: String = "",
    val security: String = "",
    val port: Int = 0,
    val missing: Boolean = false,
) {
    val isAmnezia: Boolean get() = kind == HopKind.AMNEZIA
    /** "vless · tcp/reality" for servers, the location code for Amnezia. */
    val detail: String
        get() = if (isAmnezia) country.uppercase()
        else listOf(protocol, listOf(network, security).filter { it.isNotBlank() }.joinToString("/")).filter { it.isNotBlank() }.joinToString(" · ")
}

@Serializable
data class ChainInfo(
    val id: String,
    val name: String = "",
    val hops: List<HopInfo> = emptyList(),
    val active: Boolean = false,
    /** Why the chain cannot run; null when it can. */
    val error: CoreError? = null,
    /** The last Test of this chain. */
    val last: TestResult? = null,
) {
    fun toChain() = Chain(id, name, hops.map { ChainHop(it.kind, it.server) })
}

@Serializable
data class Verdict(
    @SerialName("rule_index") val ruleIndex: Int = -1,
    @SerialName("rule_id") val ruleId: String? = null,
    val remarks: String? = null,
    val locked: Boolean = false,
    val outbound: String = "proxy",
)

@Serializable
data class GeoStatus(
    @SerialName("has_geoip") val hasGeoIp: Boolean = false,
    @SerialName("has_geosite") val hasGeoSite: Boolean = false,
    val updated: Long? = null,
)

@Serializable
data class GeoSource(
    val id: String,
    val name: String,
    @SerialName("base_url") val baseUrl: String,
)

@Serializable
data class Preset(val id: String, val rules: List<Rule>? = null)

@Serializable
data class LogLine(val t: Long, val level: String, val msg: String)

@Serializable
data class ChainResult(val ip: String, val ms: Long, val country: String = "")

/** State of the on-demand exit address / ping check shown in the connection bar. */
sealed interface Probe {
    data object Idle : Probe
    data object Testing : Probe
    data class Done(val result: ChainResult) : Probe
    data object Failed : Probe
}

@Serializable
data class PingResult(val ms: Long)

@Serializable
data class FetchRequest(
    val country: String? = null,
    @SerialName("captcha_id") val captchaId: String? = null,
    @SerialName("captcha_solution") val captchaSolution: String? = null,
)

@Serializable
data class OfficialImport(val count: Int, val release: String = "")

@Serializable
data class UpdateInfo(val newer: Boolean = false, val version: String = "", val url: String = "", val notes: String = "")
