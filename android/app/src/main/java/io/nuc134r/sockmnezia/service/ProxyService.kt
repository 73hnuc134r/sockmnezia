package io.nuc134r.sockmnezia.service

import android.app.Notification
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.SystemClock
import android.service.quicksettings.TileService
import android.content.ComponentName
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import androidx.core.content.ContextCompat
import android.net.VpnService
import android.os.ParcelFileDescriptor
import io.nuc134r.sockmnezia.AppPrefs
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.CoreError
import io.nuc134r.sockmnezia.core.CoreException
import io.nuc134r.sockmnezia.core.Modes
import io.nuc134r.sockmnezia.ui.MainActivity
import io.nuc134r.sockmnezia.ui.errorText
import io.nuc134r.sockmnezia.ui.formatBytes
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

/**
 * Foreground service that keeps the process (and the Go proxy in it) alive. Type specialUse: a local proxy
 * for other apps fits none of the predefined types. Not exported: no other app can start or stop it.
 *
 * It is also the app's [VpnService] (D31): in VPN mode it builds the VPN interface from the core's parameters
 * (routes, DNS, apps; the app itself is always outside its VPN, so Xray's own connections can't loop) and
 * hands the descriptor to the core, which runs it in the same Xray instance as the SOCKS/HTTP inbounds.
 * The system starts it with [VpnService.SERVICE_INTERFACE] for Always-on VPN.
 */
class ProxyService : VpnService() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private var ticker: Job? = null
    /** The VPN interface while VPN mode runs; closed after the core has stopped. */
    private var tun: ParcelFileDescriptor? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val prefs = AppPrefs(this)
        if (intent?.action == ACTION_STOP) {
            prefs.shouldRun = false
            scope.launch {
                stopEngine()
                stopSelf()
            }
            return START_NOT_STICKY
        }
        goForeground()
        prefs.shouldRun = true
        prefs.runBootCount = io.nuc134r.sockmnezia.bootCount(this)
        val alwaysOn = intent?.action == SERVICE_INTERFACE
        scope.launch {
            try {
                Core.init(this@ProxyService)
                ticker?.cancel() // or it would see the engine stopped below and end the service
                stopEngine() // a restart (mode or VPN settings changed) or a repeated start
                val vpn = alwaysOn || Core.settings().options.mode == Modes.VPN
                if (vpn) startVpn() else Core.start()
                isRunning = true
                requestTileUpdate(this@ProxyService)
                startTicker()
            } catch (e: Exception) {
                val ce = e as? CoreException ?: Core.asCoreException(e)
                Core.log("error", "start failed: ${ce.error.code}")
                prefs.shouldRun = false
                stopEngine()
                showFailure(errorText(this@ProxyService, ce.error))
                stopSelf()
            }
        }
        return START_STICKY
    }

    /** Builds the VPN interface and starts the core with it. Needs the user's consent ([VpnService.prepare]). */
    private suspend fun startVpn() {
        if (prepare(this) != null) throw CoreException(CoreError(code = "vpn_permission"))
        val vi = Core.vpnInterface()
        val b = Builder()
            .setSession(getString(R.string.app_name))
            .setMtu(vi.mtu)
            .setConfigureIntent(PendingIntent.getActivity(this, 0, Intent(this, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE))
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) b.setMetered(false)
        fun cidr(s: String): Pair<String, Int> = s.substringBefore('/') to s.substringAfter('/').toInt()
        vi.addresses.forEach { val (a, p) = cidr(it); b.addAddress(a, p) }
        vi.routes.forEach { val (a, p) = cidr(it); b.addRoute(a, p) }
        vi.dns.forEach { b.addDnsServer(it) }
        when (vi.apps) {
            "only" -> {
                // An allow-list implicitly leaves this app out. If none of the chosen apps is installed any
                // more, an empty allow-list would mean "every app", this one included: refuse instead.
                val added = vi.appList.filter { it != packageName }.count { runCatching { b.addAllowedApplication(it) }.isSuccess }
                if (added == 0) throw CoreException(CoreError(code = "vpn_no_apps"))
            }
            else -> {
                if (vi.apps == "except") vi.appList.forEach { runCatching { b.addDisallowedApplication(it) } }
                b.addDisallowedApplication(packageName) // Xray's own connections never enter the VPN
            }
        }
        val pfd = b.establish() ?: throw CoreException(CoreError(code = "vpn_permission"))
        tun = pfd
        Core.startVpn(pfd.fd)
    }

    /** Stops the core, then closes the VPN interface (in that order: Xray reads a duplicate of it). */
    private suspend fun stopEngine() {
        runCatching { Core.stop() }
        tun?.let { runCatching { it.close() } }
        tun = null
    }

    /** Another VPN took over, or the user turned ours off in the system settings. */
    override fun onRevoke() {
        AppPrefs(this).shouldRun = false
        scope.launch {
            stopEngine()
            stopSelf()
        }
    }

    private fun startTicker() {
        ticker?.cancel()
        ticker = scope.launch {
            var lastUp = -1L
            var lastDown = 0L
            var lastAt = 0L
            while (isActive) {
                val s = Core.refreshStatus()
                if (!s.running) {
                    stopSelf()
                    return@launch
                }
                val now = SystemClock.elapsedRealtime()
                val secs = (now - lastAt).coerceAtLeast(1) / 1000.0
                val upSpeed = if (lastUp < 0) 0L else ((s.up - lastUp).coerceAtLeast(0) / secs).toLong()
                val downSpeed = if (lastUp < 0) 0L else ((s.down - lastDown).coerceAtLeast(0) / secs).toLong()
                lastUp = s.up; lastDown = s.down; lastAt = now
                val text = getString(R.string.notif_title, formatBytes(downSpeed), formatBytes(upSpeed), formatBytes(s.up + s.down))
                ServiceCompat.startForeground(this@ProxyService, NOTIF_ID, notification(notifSubject(s), text, true), fgsType())
                delay(2000)
            }
        }
    }

    /** Notification title: the active chain's hops ("Germany→my-vps", Amnezia shows its location), with
     * "VPN ·" in front in VPN mode. */
    private fun notifSubject(s: io.nuc134r.sockmnezia.core.Status): String {
        val route = s.hops.joinToString("→") { h ->
            if (h.isAmnezia) s.countryName.ifBlank { s.country.uppercase() }.ifBlank { "Amnezia" } else h.name
        }.ifBlank { s.chainName ?: getString(R.string.app_name) }
        return if (s.vpn) getString(R.string.notif_vpn, route) else route
    }

    private fun fgsType() =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE else 0

    private fun goForeground() {
        ServiceCompat.startForeground(this, NOTIF_ID, notification(getString(R.string.app_name), getString(R.string.notif_starting), true), fgsType())
    }

    private fun notification(title: String, text: String, withStop: Boolean): Notification {
        val open = PendingIntent.getActivity(
            this, 0, Intent(this, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val b = NotificationCompat.Builder(this, CHANNEL)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(title)
            .setContentText(text)
            .setContentIntent(open)
            .setShowWhen(false)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .setForegroundServiceBehavior(NotificationCompat.FOREGROUND_SERVICE_IMMEDIATE)
        if (withStop) {
            val stop = PendingIntent.getService(
                this, 1, Intent(this, ProxyService::class.java).setAction(ACTION_STOP), PendingIntent.FLAG_IMMUTABLE,
            )
            b.addAction(0, getString(R.string.action_stop), stop)
        }
        return b.build()
    }

    private fun showFailure(text: String) {
        val nm = getSystemService(android.app.NotificationManager::class.java)
        nm.notify(NOTIF_FAIL_ID, notification(getString(R.string.app_name), getString(R.string.notif_failed, text), false).also {
            it.flags = it.flags and Notification.FLAG_ONGOING_EVENT.inv()
        })
    }

    override fun onDestroy() {
        ticker?.cancel()
        isRunning = false
        // Stop the proxy whenever the service goes away, so it never runs without the notification.
        runCatching { kotlinx.coroutines.runBlocking { stopEngine() } }
        scope.cancel()
        requestTileUpdate(this)
        super.onDestroy()
    }

    companion object {
        const val CHANNEL = "proxy"
        const val NOTIF_ID = 1
        const val NOTIF_FAIL_ID = 2
        const val ACTION_STOP = "io.nuc134r.sockmnezia.STOP"

        @Volatile var isRunning = false
            private set

        fun start(context: Context) {
            ContextCompat.startForegroundService(context, Intent(context, ProxyService::class.java))
        }

        /** Starts again with the current mode and VPN settings (also when already running). */
        fun restart(context: Context) = start(context)

        fun stop(context: Context) {
            context.startService(Intent(context, ProxyService::class.java).setAction(ACTION_STOP))
        }

        fun requestTileUpdate(context: Context) {
            TileService.requestListeningState(context, ComponentName(context, ProxyTileService::class.java))
        }
    }
}
