package io.nuc134r.sockmnezia

import io.nuc134r.sockmnezia.health.CheckId
import io.nuc134r.sockmnezia.health.DeviceState
import io.nuc134r.sockmnezia.health.Severity
import io.nuc134r.sockmnezia.health.evaluate
import io.nuc134r.sockmnezia.health.needsAction
import io.nuc134r.sockmnezia.health.worstActionable
import io.nuc134r.sockmnezia.health.worst
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class HealthTest {
    private val good = DeviceState(
        sdk = 34, manufacturer = "Google", ignoringBatteryOptimizations = true, backgroundRestricted = false,
        notificationsEnabled = true, dataSaverBlocksBackground = false, unusedAppRestrictionsOn = false,
        powerSaveMode = false, startOnBoot = false, bootSeen = false, unexpectedStopsLastWeek = 0,
    )

    @Test fun allGood() {
        val c = evaluate(good)
        assertEquals(Severity.OK, c.worst())
        assertTrue(c.none { it.id == CheckId.OEM || it.id == CheckId.KILLS || it.id == CheckId.POWER_SAVE })
    }

    @Test fun problems() {
        val c = evaluate(
            good.copy(
                ignoringBatteryOptimizations = false, backgroundRestricted = true, dataSaverBlocksBackground = true,
                manufacturer = "Xiaomi", unexpectedStopsLastWeek = 3, powerSaveMode = true, notificationsEnabled = false,
            ),
        )
        assertEquals(Severity.PROBLEM, c.worst())
        val bad = c.filter { !it.ok }.map { it.id }.toSet()
        assertEquals(
            setOf(CheckId.BATTERY_OPTIMIZATION, CheckId.BACKGROUND_RESTRICTED, CheckId.DATA_SAVER, CheckId.OEM,
                CheckId.KILLS, CheckId.POWER_SAVE, CheckId.NOTIFICATIONS),
            bad,
        )
        assertEquals(3, c.first { it.id == CheckId.KILLS }.count)
    }

    @Test fun oldAndroidHasNoBackgroundRestriction() {
        assertTrue(evaluate(good.copy(sdk = 26)).none { it.id == CheckId.BACKGROUND_RESTRICTED })
    }

    @Test fun unknownHibernationIsSkipped() {
        assertTrue(evaluate(good.copy(unusedAppRestrictionsOn = null)).none { it.id == CheckId.HIBERNATION })
        assertEquals(Severity.WARNING, evaluate(good.copy(unusedAppRestrictionsOn = true)).worst())
    }

    @Test fun bootNotSeenYet() {
        assertTrue(evaluate(good.copy(startOnBoot = true)).any { it.id == CheckId.BOOT })
        assertTrue(evaluate(good.copy(startOnBoot = true, bootSeen = true)).none { it.id == CheckId.BOOT })
    }

    @Test fun homeCardOnlyForSettingsToChange() {
        // Xiaomi + stops in the log + power save: advice only, nothing to switch -> no Home card.
        val advice = evaluate(good.copy(manufacturer = "Xiaomi", unexpectedStopsLastWeek = 2, powerSaveMode = true))
        assertEquals(Severity.PROBLEM, advice.worst())
        assertFalse(advice.needsAction())
        assertEquals(Severity.OK, advice.worstActionable())
        // A setting to change brings it back.
        val battery = evaluate(good.copy(manufacturer = "Xiaomi", ignoringBatteryOptimizations = false))
        assertTrue(battery.needsAction())
        assertEquals(Severity.PROBLEM, battery.worstActionable())
        assertTrue(evaluate(good.copy(notificationsEnabled = false)).needsAction())
    }
}
