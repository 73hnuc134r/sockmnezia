package io.nuc134r.sockmnezia.service

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import io.nuc134r.sockmnezia.AppPrefs

/** Starts the proxy after boot when the user turned that on and it was running before. */
class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED && intent.action != Intent.ACTION_MY_PACKAGE_REPLACED) return
        val prefs = AppPrefs(context)
        prefs.bootedOnce = true
        if (prefs.startOnBoot && prefs.shouldRun) {
            runCatching { ProxyService.start(context) }
        }
    }
}
