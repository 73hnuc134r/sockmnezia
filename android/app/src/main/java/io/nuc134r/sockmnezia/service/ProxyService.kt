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
import androidx.lifecycle.LifecycleService
import androidx.lifecycle.lifecycleScope
import io.nuc134r.sockmnezia.AppPrefs
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.CoreException
import io.nuc134r.sockmnezia.ui.MainActivity
import io.nuc134r.sockmnezia.ui.errorText
import io.nuc134r.sockmnezia.ui.formatBytes
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

/**
 * Foreground service that keeps the process (and the Go proxy in it) alive. Type specialUse: a local proxy
 * for other apps fits none of the predefined types. Not exported: no other app can start or stop it.
 */
class ProxyService : LifecycleService() {
    private var ticker: Job? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        super.onStartCommand(intent, flags, startId)
        val prefs = AppPrefs(this)
        if (intent?.action == ACTION_STOP) {
            prefs.shouldRun = false
            lifecycleScope.launch {
                runCatching { Core.stop() }
                stopSelf()
            }
            return START_NOT_STICKY
        }
        goForeground()
        prefs.shouldRun = true
        prefs.runBootCount = io.nuc134r.sockmnezia.bootCount(this)
        lifecycleScope.launch {
            try {
                Core.init(this@ProxyService)
                Core.start()
                isRunning = true
                requestTileUpdate(this@ProxyService)
                startTicker()
            } catch (e: Exception) {
                val ce = e as? CoreException ?: Core.asCoreException(e)
                Core.log("error", "start failed: ${ce.error.code}")
                prefs.shouldRun = false
                showFailure(errorText(this@ProxyService, ce.error))
                stopSelf()
            }
        }
        return START_STICKY
    }

    private fun startTicker() {
        ticker?.cancel()
        ticker = lifecycleScope.launch {
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

    /** Notification title: the active chain's hops ("Germany→my-vps"): Amnezia shows its location. */
    private fun notifSubject(s: io.nuc134r.sockmnezia.core.Status): String =
        s.hops.joinToString("→") { h ->
            if (h.isAmnezia) s.countryName.ifBlank { s.country.uppercase() }.ifBlank { "Amnezia" } else h.name
        }.ifBlank { s.chainName ?: getString(R.string.app_name) }

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
        runCatching { kotlinx.coroutines.runBlocking { Core.stop() } }
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

        fun stop(context: Context) {
            context.startService(Intent(context, ProxyService::class.java).setAction(ACTION_STOP))
        }

        fun requestTileUpdate(context: Context) {
            TileService.requestListeningState(context, ComponentName(context, ProxyTileService::class.java))
        }
    }
}
