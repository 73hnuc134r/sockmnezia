package io.nuc134r.sockmnezia.health

/** What the checks look at; collected by [DeviceStateReader], faked in unit tests. */
data class DeviceState(
    val sdk: Int,
    val manufacturer: String,
    val ignoringBatteryOptimizations: Boolean,
    val backgroundRestricted: Boolean,
    val notificationsEnabled: Boolean,
    val dataSaverBlocksBackground: Boolean,
    /** null when the status is unknown. */
    val unusedAppRestrictionsOn: Boolean?,
    val powerSaveMode: Boolean,
    val startOnBoot: Boolean,
    val bootSeen: Boolean,
    val unexpectedStopsLastWeek: Int,
)

enum class Severity { OK, INFO, WARNING, PROBLEM }

enum class CheckId {
    BATTERY_OPTIMIZATION, BACKGROUND_RESTRICTED, NOTIFICATIONS, DATA_SAVER, HIBERNATION, POWER_SAVE, OEM, BOOT, KILLS
}

enum class Fix { BATTERY_OPTIMIZATION, APP_DETAILS, NOTIFICATIONS, DATA_SAVER, UNUSED_APP, BATTERY_SAVER, OEM_SETTINGS, NONE }

data class Check(val id: CheckId, val severity: Severity, val fix: Fix = Fix.NONE, val count: Int = 0) {
    val ok get() = severity == Severity.OK
}

/** Phone makers known to stop background apps beyond stock Android (see dontkillmyapp.com). */
val aggressiveOems = setOf("xiaomi", "redmi", "poco", "huawei", "honor", "samsung", "oppo", "realme", "vivo", "oneplus", "meizu", "asus", "nokia", "tecno", "infinix")

fun evaluate(s: DeviceState): List<Check> = buildList {
    add(
        Check(
            CheckId.BATTERY_OPTIMIZATION,
            if (s.ignoringBatteryOptimizations) Severity.OK else Severity.PROBLEM,
            Fix.BATTERY_OPTIMIZATION,
        ),
    )
    if (s.sdk >= 28) {
        add(Check(CheckId.BACKGROUND_RESTRICTED, if (s.backgroundRestricted) Severity.PROBLEM else Severity.OK, Fix.APP_DETAILS))
    }
    add(Check(CheckId.NOTIFICATIONS, if (s.notificationsEnabled) Severity.OK else Severity.WARNING, Fix.NOTIFICATIONS))
    add(Check(CheckId.DATA_SAVER, if (s.dataSaverBlocksBackground) Severity.PROBLEM else Severity.OK, Fix.DATA_SAVER))
    s.unusedAppRestrictionsOn?.let {
        add(Check(CheckId.HIBERNATION, if (it) Severity.WARNING else Severity.OK, Fix.UNUSED_APP))
    }
    if (s.powerSaveMode) add(Check(CheckId.POWER_SAVE, Severity.INFO, Fix.BATTERY_SAVER))
    if (s.manufacturer.lowercase() in aggressiveOems) add(Check(CheckId.OEM, Severity.WARNING, Fix.OEM_SETTINGS))
    if (s.startOnBoot && !s.bootSeen) add(Check(CheckId.BOOT, Severity.INFO))
    if (s.unexpectedStopsLastWeek > 0) {
        add(Check(CheckId.KILLS, Severity.PROBLEM, count = s.unexpectedStopsLastWeek))
    }
}

/** The worst severity, for the summary on the main screen. */
fun List<Check>.worst(): Severity = maxOfOrNull { it.severity } ?: Severity.OK
