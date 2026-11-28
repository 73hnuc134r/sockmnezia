package io.nuc134r.sockmnezia.core

import android.content.Context
import io.nuc134r.gocore.mobile.Mobile
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.withContext
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.builtins.serializer
import java.io.File
import java.util.concurrent.atomic.AtomicLong
import io.nuc134r.gocore.mobile.Core as GoCore

/**
 * Process-wide access to the Go core. All calls run on [Dispatchers.IO]; Go errors arrive as
 * [CoreException] with a code the UI translates.
 */
object Core {
    @Volatile private var go: GoCore? = null
    private val _status = MutableStateFlow(Status())
    val status: StateFlow<Status> = _status.asStateFlow()

    /** Bumped whenever settings, servers or the Amnezia config change, so screens reload. */
    private val _revision = MutableStateFlow(0)
    val revision: StateFlow<Int> = _revision.asStateFlow()

    @Synchronized
    fun init(context: Context): GoCore {
        go?.let { return it }
        val dir = File(context.filesDir, "core").apply { mkdirs() }
        val c = try {
            Mobile.open(dir.absolutePath, KeystoreBox())
        } catch (e: Exception) {
            throw asCoreException(e)
        }
        go = c
        refreshStatus()
        return c
    }

    private fun core(): GoCore = go ?: error("Core.init was not called")

    fun asCoreException(e: Throwable): CoreException {
        if (e is CoreException) return e
        val msg = e.message ?: ""
        val err = runCatching { CoreJson.decodeFromString(CoreError.serializer(), msg) }
            .getOrElse { CoreError(code = "internal", detail = msg) }
        return CoreException(err)
    }

    private suspend fun <T> call(block: (GoCore) -> T): T = withContext(Dispatchers.IO) {
        try {
            block(core())
        } catch (e: Exception) {
            throw asCoreException(e)
        }
    }

    private fun changed() {
        clearProbe()
        _revision.value += 1
        refreshStatus()
    }

    fun refreshStatus(): Status {
        val s = go?.let { CoreJson.decodeFromString(Status.serializer(), it.statusJSON()) } ?: Status()
        _status.value = s
        return s
    }

    // --- Amnezia -------------------------------------------------------------------------------------
    suspend fun setKey(key: String) = call { it.setKey(key) }.also { changed() }
    suspend fun importGatewayKeysFromFile(path: String): Long = call { it.importGatewayKeysFromFile(path) }.also { changed() }
    suspend fun importGatewayKeysFromOfficialRelease(): OfficialImport =
        call { CoreJson.decodeFromString(OfficialImport.serializer(), it.importGatewayKeysFromOfficialRelease()) }.also { changed() }

    suspend fun setGatewayKeysPem(text: String): Long = call { it.setGatewayKeysPEM(text) }.also { changed() }
    suspend fun fetch(req: FetchRequest = FetchRequest()) =
        call { it.fetch(CoreJson.encodeToString(FetchRequest.serializer(), req)) }.also { changed() }

    suspend fun countries(): List<Country> =
        call { CoreJson.decodeFromString(ListSerializer(Country.serializer()), it.countriesJSON()) }

    // --- proxy ---------------------------------------------------------------------------------------
    suspend fun start() = call { it.start() }.also { changed() }
    suspend fun stop() = call { it.stop() }.also { changed() }

    /** Starts in VPN mode with the VpnService tun [fd] (kept open by the caller until after [stop]). */
    suspend fun startVpn(fd: Int) = call { it.startVPN(fd.toLong()) }.also { changed() }

    /** What VpnService.Builder needs for the current settings. */
    suspend fun vpnInterface(): VpnInterface =
        call { CoreJson.decodeFromString(VpnInterface.serializer(), it.vpnInterfaceJSON()) }

    suspend fun settings(): Settings = call { CoreJson.decodeFromString(Settings.serializer(), it.settingsJSON()) }
    suspend fun setSettings(s: Settings) =
        call { it.setSettingsJSON(CoreJson.encodeToString(Settings.serializer(), s)) }.also { changed() }

    suspend fun randomPort(avoid: Int = 0): Int = call { it.randomPort(avoid.toLong()).toInt() }
    fun randomUser(): String = Mobile.randomUser()
    fun randomPassword(): String = Mobile.randomPassword()

    suspend fun exitIp(): String = call { it.exitIP() }
    /** Forgets every remembered Test result. */
    suspend fun clearTests() = call { it.clearTests() }

    /** Measures a saved chain or a single server without selecting it (the running proxy is untouched). */
    suspend fun testSaved(chainId: String = "", serverId: String = ""): ChainResult =
        call { CoreJson.decodeFromString(ChainResult.serializer(), it.testSavedJSON(chainId, serverId)) }

    suspend fun testChain(): ChainResult = call { CoreJson.decodeFromString(ChainResult.serializer(), it.testChainJSON()) }

    /** Fetches a one-off config for [country] and measures a request through it, without changing the saved config. */
    suspend fun pingCountry(country: String): Long =
        call { CoreJson.decodeFromString(PingResult.serializer(), it.pingCountryJSON(country)).ms }

    private val _probe = MutableStateFlow<Probe>(Probe.Idle)
    private val probeGen = AtomicLong()
    /** Last exit address / ping check (on demand only). Cleared on every change (start, stop, another
     * chain, server, location, settings), so a result never outlives the config it was measured on. */
    val probe: StateFlow<Probe> = _probe.asStateFlow()

    /** Runs the check. A result that arrives after the config changed is dropped; a failure replaces any
     * earlier result (it is never left on screen). */
    suspend fun runProbe() {
        val gen = probeGen.incrementAndGet()
        _probe.value = Probe.Testing
        try {
            val r = testChain()
            if (probeGen.get() == gen) _probe.value = Probe.Done(r)
        } catch (e: Exception) {
            if (probeGen.get() == gen) _probe.value = Probe.Failed
            throw e
        }
    }

    fun clearProbe() {
        probeGen.incrementAndGet()
        _probe.value = Probe.Idle
    }

    // --- own servers ---------------------------------------------------------------------------------
    suspend fun ownServers(): List<OwnServer> =
        call { CoreJson.decodeFromString(ListSerializer(OwnServer.serializer()), it.ownServersJSON()) }

    suspend fun addOwnServer(link: String, name: String): String = call { it.addOwnServer(link, name) }.also { changed() }
    /** A saved server for the endpoint editor. */
    suspend fun serverEdit(id: String): ServerEdit =
        call { CoreJson.decodeFromString(ServerEdit.serializer(), it.serverEditJSON(id)) }

    /** Replaces a saved server's settings, keeping its ID (chains that use it follow). */
    suspend fun updateOwnServer(id: String, link: String) = call { it.updateOwnServer(id, link, "") }.also { changed() }

    /** Editor fields -> link, validated like a pasted link. */
    fun endpointToLink(e: Endpoint): String = try {
        Mobile.endpointToLink(CoreJson.encodeToString(Endpoint.serializer(), e))
    } catch (x: Exception) {
        throw asCoreException(x)
    }

    /** Link or outbound JSON -> editor fields (error code json_only when only JSON can express it). */
    fun linkToEndpoint(link: String): Endpoint = try {
        CoreJson.decodeFromString(Endpoint.serializer(), Mobile.linkToEndpointJSON(link))
    } catch (x: Exception) {
        throw asCoreException(x)
    }

    suspend fun removeOwnServer(id: String) = call { it.removeOwnServer(id) }.also { changed() }
    suspend fun renameOwnServer(id: String, name: String) = call { it.renameOwnServer(id, name) }.also { changed() }

    // --- chains --------------------------------------------------------------------------------------
    suspend fun chains(): List<ChainInfo> =
        call { CoreJson.decodeFromString(ListSerializer(ChainInfo.serializer()), it.chainsJSON()) }

    /** Saves [c] (a new chain when it has no ID) and returns its ID. */
    suspend fun saveChain(c: Chain): String =
        call { it.saveChainJSON(CoreJson.encodeToString(Chain.serializer(), c)) }.also { changed() }

    suspend fun deleteChain(id: String) = call { it.deleteChain(id) }.also { changed() }
    suspend fun setActiveChain(id: String) = call { it.setActiveChain(id) }.also { changed() }

    /** Selects a single own server instead of a chain. */
    suspend fun setActiveServer(id: String) = call { it.setActiveServer(id) }.also { changed() }

    /** A saved server's share link (contains its credentials). */
    suspend fun shareServer(id: String): String = call { it.shareServer(id) }

    /** A chain as text ([format] "links") or Xray outbounds JSON ("xray"). */
    suspend fun shareChain(id: String, format: String): String = call { it.shareChain(id, format) }

    /** Imports shared text (a chain, links or an outbound JSON); with [dryRun] nothing is saved. */
    suspend fun shareAll(): String = call { it.shareAll() }
    suspend fun import(text: String, dryRun: Boolean): ImportResult =
        call { CoreJson.decodeFromString(ImportResult.serializer(), it.importJSON(text, dryRun)) }
            .also { if (!dryRun) changed() }

    /** Why [c] cannot run, or null when it can (no side effects). */
    suspend fun validateChain(c: Chain): CoreError? = call {
        it.validateChainJSON(CoreJson.encodeToString(Chain.serializer(), c))
    }.takeIf { it.isNotBlank() }?.let { CoreJson.decodeFromString(CoreError.serializer(), it) }

    // --- routing and geo -----------------------------------------------------------------------------
    fun presets(): List<Preset> = CoreJson.decodeFromString(ListSerializer(Preset.serializer()), Mobile.presetsJSON())
    fun lockedRules(): List<Rule> = CoreJson.decodeFromString(ListSerializer(Rule.serializer()), Mobile.lockedRulesJSON())
    fun geoSources(): List<GeoSource> = CoreJson.decodeFromString(ListSerializer(GeoSource.serializer()), Mobile.geoSourcesJSON())

    fun parseRouting(json: String): Routing = try {
        CoreJson.decodeFromString(Routing.serializer(), Mobile.parseRoutingJSON(json))
    } catch (e: Exception) {
        throw asCoreException(e)
    }

    fun validateRule(rule: Rule) {
        try {
            Mobile.validateRuleJSON(CoreJson.encodeToString(Rule.serializer(), rule))
        } catch (e: Exception) {
            throw asCoreException(e)
        }
    }

    fun exportRules(rules: List<Rule>): String =
        CoreJson.encodeToString(ListSerializer(Rule.serializer()), rules)

    suspend fun testRoute(host: String, port: Int, network: String): Verdict =
        call { CoreJson.decodeFromString(Verdict.serializer(), it.testRouteJSON(host, port.toLong(), network)) }

    suspend fun geoStatus(): GeoStatus = call { CoreJson.decodeFromString(GeoStatus.serializer(), it.geoStatusJSON()) }
    suspend fun geoTags(kind: String): List<String> =
        call { CoreJson.decodeFromString(ListSerializer(String.serializer()), it.geoTagsJSON(kind)) }

    suspend fun updateGeo() = call { it.updateGeo() }.also { changed() }

    /** Progress of the running geo update (started here or by the weekly job), null when none runs. */
    suspend fun geoProgress(): GeoProgress? =
        call { it.geoProgressJSON() }.takeIf { it.isNotBlank() }?.let { CoreJson.decodeFromString(GeoProgress.serializer(), it) }

    // --- updates -------------------------------------------------------------------------------------
    private val _update = MutableStateFlow<UpdateInfo?>(null)
    val update: StateFlow<UpdateInfo?> = _update.asStateFlow()

    /** Downloads the newer release's APK for this device (verified against the release's SHA256SUMS). */
    suspend fun downloadUpdate(dir: java.io.File): java.io.File = call {
        java.io.File(it.downloadUpdate(android.os.Build.SUPPORTED_ABIS.joinToString(","), dir.absolutePath))
    }

    /** Progress of the running update download, null when none runs. */
    suspend fun updateProgress(): DownloadProgress? =
        call { it.updateProgressJSON() }.takeIf { it.isNotBlank() }?.let { CoreJson.decodeFromString(DownloadProgress.serializer(), it) }

    suspend fun checkUpdate(current: String, beta: Boolean): UpdateInfo =
        call { CoreJson.decodeFromString(UpdateInfo.serializer(), it.checkUpdateJSON(current, beta)) }.also { _update.value = it }

    // --- misc ----------------------------------------------------------------------------------------
    fun logs(): List<LogLine> = CoreJson.decodeFromString(ListSerializer(LogLine.serializer()), Mobile.logsJSON())
    fun clearLogs() = Mobile.clearLogs()
    fun log(level: String, msg: String) = Mobile.log(level, msg)
    fun xrayVersion(): String = Mobile.xrayVersion()
    suspend fun forget() = call { it.forget() }.also { changed() }
    suspend fun stats(days: Int): List<StatsDay> =
        call { CoreJson.decodeFromString(ListSerializer(StatsDay.serializer()), it.statsJSON(days.toLong())) }
    suspend fun resetStats() = call { it.resetStats() }
}
