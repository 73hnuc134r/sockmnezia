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

    suspend fun settings(): Settings = call { CoreJson.decodeFromString(Settings.serializer(), it.settingsJSON()) }
    suspend fun setSettings(s: Settings) =
        call { it.setSettingsJSON(CoreJson.encodeToString(Settings.serializer(), s)) }.also { changed() }

    suspend fun randomPort(avoid: Int = 0): Int = call { it.randomPort(avoid.toLong()).toInt() }
    fun randomUser(): String = Mobile.randomUser()
    fun randomPassword(): String = Mobile.randomPassword()

    suspend fun exitIp(): String = call { it.exitIP() }
    suspend fun testChain(): ChainResult = call { CoreJson.decodeFromString(ChainResult.serializer(), it.testChainJSON()) }

    // --- own servers ---------------------------------------------------------------------------------
    suspend fun ownServers(): List<OwnServer> =
        call { CoreJson.decodeFromString(ListSerializer(OwnServer.serializer()), it.ownServersJSON()) }

    suspend fun addOwnServer(link: String, name: String): String = call { it.addOwnServer(link, name) }.also { changed() }
    suspend fun removeOwnServer(id: String) = call { it.removeOwnServer(id) }.also { changed() }
    suspend fun renameOwnServer(id: String, name: String) = call { it.renameOwnServer(id, name) }.also { changed() }
    suspend fun setActiveOwnServer(id: String) = call { it.setActiveOwnServer(id) }.also { changed() }

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

    // --- updates -------------------------------------------------------------------------------------
    private val _update = MutableStateFlow<UpdateInfo?>(null)
    val update: StateFlow<UpdateInfo?> = _update.asStateFlow()

    suspend fun checkUpdate(current: String, beta: Boolean): UpdateInfo =
        call { CoreJson.decodeFromString(UpdateInfo.serializer(), it.checkUpdateJSON(current, beta)) }.also { _update.value = it }

    // --- misc ----------------------------------------------------------------------------------------
    fun logs(): List<LogLine> = CoreJson.decodeFromString(ListSerializer(LogLine.serializer()), Mobile.logsJSON())
    fun clearLogs() = Mobile.clearLogs()
    fun log(level: String, msg: String) = Mobile.log(level, msg)
    fun xrayVersion(): String = Mobile.xrayVersion()
    suspend fun forget() = call { it.forget() }.also { changed() }
}
