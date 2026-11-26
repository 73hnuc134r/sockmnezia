package io.nuc134r.sockmnezia.ui

import android.Manifest
import android.app.Activity
import android.content.pm.PackageManager
import android.net.VpnService
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalContext
import androidx.core.content.ContextCompat
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.core.Modes
import io.nuc134r.sockmnezia.service.ProxyService

/**
 * Starts (or restarts) the proxy service in the configured mode, asking first for what it needs: the
 * notification permission (Android 13+) and, in VPN mode, the user's VPN consent ([VpnService.prepare]).
 * Starting while running restarts with the current mode and VPN settings.
 */
@Composable
fun rememberStarter(): () -> Unit {
    val ctx = LocalContext.current
    val vpnConsent = rememberLauncherForActivityResult(ActivityResultContracts.StartActivityForResult()) { r ->
        if (r.resultCode == Activity.RESULT_OK) ProxyService.start(ctx)
    }
    fun afterNotifications() {
        if (Core.status.value.mode == Modes.VPN) {
            VpnService.prepare(ctx)?.let { vpnConsent.launch(it); return }
        }
        ProxyService.start(ctx)
    }
    val notifPermission = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { afterNotifications() }
    return remember {
        {
            if (Build.VERSION.SDK_INT >= 33 &&
                ContextCompat.checkSelfPermission(ctx, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
            ) {
                notifPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
            } else {
                afterNotifications()
            }
        }
    }
}
