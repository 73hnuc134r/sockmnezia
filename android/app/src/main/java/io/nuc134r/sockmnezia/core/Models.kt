package io.nuc134r.sockmnezia.core

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

/** JSON shared with the Go core (core/engine, core/mobile). Field names follow the Go side. */
val CoreJson = Json {
    ignoreUnknownKeys = true
    encodeDefaults = true
    explicitNulls = false
}

object Modes {
    const val AMNEZIA = "amnezia"
    const val AMNEZIA_OWN = "amnezia_own"
    const val OWN = "own"
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
    val mode: String = Modes.AMNEZIA,
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
    @SerialName("fetch_via_own") val fetchViaOwn: Boolean = false,
    @SerialName("geo_source") val geoSource: String = "runetfreedom",
    @SerialName("geo_custom_url") val geoCustomUrl: String = "",
    @SerialName("geo_auto_update") val geoAutoUpdate: Boolean = false,
    @SerialName("log_level") val logLevel: String = "warning",
    @SerialName("exit_ip_url") val exitIpUrl: String = "https://api.ipify.org",
    @SerialName("refresh_after_hrs") val refreshAfterHrs: Int = 24,
)

@Serializable
data class Settings(
    val mode: String = Modes.AMNEZIA,
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
    val active: Boolean = false,
)

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
data class ChainResult(val ip: String, val ms: Long)

@Serializable
data class FetchRequest(
    val country: String? = null,
    @SerialName("captcha_id") val captchaId: String? = null,
    @SerialName("captcha_solution") val captchaSolution: String? = null,
)

@Serializable
data class OfficialImport(val count: Int, val release: String = "")
