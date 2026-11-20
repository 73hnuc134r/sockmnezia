package io.nuc134r.sockmnezia.health

import android.annotation.SuppressLint
import android.app.ActivityManager
import android.content.ActivityNotFoundException
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.net.ConnectivityManager
import android.os.Build
import android.os.PowerManager
import android.provider.Settings
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import androidx.core.net.toUri
import androidx.core.content.IntentCompat
import androidx.core.content.PackageManagerCompat
import androidx.core.content.UnusedAppRestrictionsConstants
import io.nuc134r.sockmnezia.AppPrefs

object DeviceStateReader {
    fun read(context: Context, unusedRestrictions: Int?): DeviceState {
        val pm = context.getSystemService(PowerManager::class.java)
        val am = context.getSystemService(ActivityManager::class.java)
        val cm = context.getSystemService(ConnectivityManager::class.java)
        val prefs = AppPrefs(context)
        return DeviceState(
            sdk = Build.VERSION.SDK_INT,
            manufacturer = Build.MANUFACTURER ?: "",
            ignoringBatteryOptimizations = pm.isIgnoringBatteryOptimizations(context.packageName),
            backgroundRestricted = Build.VERSION.SDK_INT >= 28 && am.isBackgroundRestricted,
            notificationsEnabled = NotificationManagerCompat.from(context).areNotificationsEnabled(),
            dataSaverBlocksBackground = cm.restrictBackgroundStatus == ConnectivityManager.RESTRICT_BACKGROUND_STATUS_ENABLED,
            unusedAppRestrictionsOn = when (unusedRestrictions) {
                null, UnusedAppRestrictionsConstants.ERROR, UnusedAppRestrictionsConstants.FEATURE_NOT_AVAILABLE -> null
                UnusedAppRestrictionsConstants.DISABLED -> false
                else -> true
            },
            powerSaveMode = pm.isPowerSaveMode,
            startOnBoot = prefs.startOnBoot,
            bootSeen = prefs.bootedOnce,
            unexpectedStopsLastWeek = prefs.stopsLastWeek(),
        )
    }

    /** Unused-app restriction status is asynchronous. */
    fun unusedRestrictions(context: Context, onResult: (Int?) -> Unit) {
        val f = PackageManagerCompat.getUnusedAppRestrictionsStatus(context)
        f.addListener({ onResult(runCatching { f.get() }.getOrNull()) }, ContextCompat.getMainExecutor(context))
    }

    private fun appDetails(context: Context) =
        Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, "package:${context.packageName}".toUri())

    @SuppressLint("BatteryLife") // the app is distributed outside Google Play; the exemption is its core need
    fun intentFor(context: Context, fix: Fix): List<Intent> = when (fix) {
        Fix.BATTERY_OPTIMIZATION -> listOf(
            Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, "package:${context.packageName}".toUri()),
            Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS),
        )
        Fix.APP_DETAILS -> listOf(appDetails(context))
        Fix.NOTIFICATIONS -> listOf(
            Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(Settings.EXTRA_APP_PACKAGE, context.packageName),
            appDetails(context),
        )
        Fix.DATA_SAVER -> listOf(
            Intent(Settings.ACTION_IGNORE_BACKGROUND_DATA_RESTRICTIONS_SETTINGS, "package:${context.packageName}".toUri()),
            appDetails(context),
        )
        Fix.UNUSED_APP -> listOf(IntentCompat.createManageUnusedAppRestrictionsIntent(context, context.packageName), appDetails(context))
        Fix.BATTERY_SAVER -> listOf(Intent(Settings.ACTION_BATTERY_SAVER_SETTINGS))
        Fix.OEM_SETTINGS -> oemIntents() + appDetails(context)
        Fix.NONE -> emptyList()
    }

    /** Known autostart / background screens of OEM skins; tried in order, the first that resolves wins. */
    private fun oemIntents(): List<Intent> = listOf(
        "com.miui.securitycenter" to "com.miui.permcenter.autostart.AutoStartManagementActivity",
        "com.huawei.systemmanager" to "com.huawei.systemmanager.startupmgr.ui.StartupNormalAppListActivity",
        "com.huawei.systemmanager" to "com.huawei.systemmanager.optimize.process.ProtectActivity",
        "com.hihonor.systemmanager" to "com.hihonor.systemmanager.startupmgr.ui.StartupNormalAppListActivity",
        "com.samsung.android.lool" to "com.samsung.android.sm.battery.ui.BatteryActivity",
        "com.coloros.safecenter" to "com.coloros.safecenter.permission.startup.StartupAppListActivity",
        "com.oplus.safecenter" to "com.oplus.safecenter.permission.startup.StartupAppListActivity",
        "com.vivo.permissionmanager" to "com.vivo.permissionmanager.activity.BgStartUpManagerActivity",
        "com.iqoo.secure" to "com.iqoo.secure.ui.phoneoptimize.BgStartUpManager",
        "com.oneplus.security" to "com.oneplus.security.chainlaunch.view.ChainLaunchAppListActivity",
        "com.asus.mobilemanager" to "com.asus.mobilemanager.autostart.AutoStartActivity",
    ).map { (pkg, cls) -> Intent().setComponent(ComponentName(pkg, cls)) }

    /** Opens the first intent that works; returns false when none did. */
    fun open(context: Context, intents: List<Intent>): Boolean {
        for (i in intents) {
            try {
                context.startActivity(i.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
                return true
            } catch (_: ActivityNotFoundException) {
            } catch (_: SecurityException) {
            }
        }
        return false
    }
}
