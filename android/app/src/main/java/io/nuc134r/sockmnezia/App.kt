package io.nuc134r.sockmnezia

import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.service.GeoUpdateWorker
import io.nuc134r.sockmnezia.service.ProxyService

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
        // The proxy should be running but the process was started without it: Android killed us.
        if (prefs.shouldRun && !ProxyService.isRunning) prefs.recordUnexpectedStop()
        GeoUpdateWorker.schedule(this)
    }
}
