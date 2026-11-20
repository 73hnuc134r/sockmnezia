package io.nuc134r.sockmnezia

import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.service.GeoUpdateWorker
import io.nuc134r.sockmnezia.service.ProxyService

fun bootCount(context: android.content.Context): Int =
    android.provider.Settings.Global.getInt(context.contentResolver, android.provider.Settings.Global.BOOT_COUNT, 0)

class App : Application() {
    override fun onCreate() {
        super.onCreate()
        val nm = getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(
            NotificationChannel(ProxyService.CHANNEL, getString(R.string.channel_proxy), NotificationManager.IMPORTANCE_LOW).apply {
                setShowBadge(false)
            },
        )
        runCatching { Core.init(this) }
        val prefs = AppPrefs(this)
        // The proxy should be running, the process is new and the phone did not reboot: Android killed us.
        if (prefs.shouldRun && prefs.runBootCount == bootCount(this)) {
            prefs.recordUnexpectedStop()
            prefs.runBootCount = -1 // count each kill once; the service sets it again when it starts
        }
        GeoUpdateWorker.schedule(this)
    }
}
