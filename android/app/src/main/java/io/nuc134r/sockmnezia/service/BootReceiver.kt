package io.nuc134r.sockmnezia.service

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import io.nuc134r.sockmnezia.AppPrefs

/** Starts the proxy after boot when the user turned that on and it was running before, and after an update of
 * the app (installing it stopped the running proxy/VPN) whenever it was running. */
class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val prefs = AppPrefs(context)
        val start = when (intent.action) {
            Intent.ACTION_BOOT_COMPLETED -> {
                prefs.bootedOnce = true
                prefs.startOnBoot && prefs.shouldRun
            }
            Intent.ACTION_MY_PACKAGE_REPLACED -> prefs.shouldRun
            else -> false
        }
        if (start) runCatching { ProxyService.start(context) }
    }
}
