package io.nuc134r.sockmnezia

import android.content.Context
import androidx.core.content.edit

/** App-side preferences that the Go core does not need (no secrets here). */
class AppPrefs(context: Context) {
    private val p = context.getSharedPreferences("app", Context.MODE_PRIVATE)

    /** The user started the proxy and has not stopped it. */
    var shouldRun: Boolean
        get() = p.getBoolean("should_run", false)
        set(v) = p.edit { putBoolean("should_run", v) }

    var startOnBoot: Boolean
        get() = p.getBoolean("start_on_boot", false)
        set(v) = p.edit { putBoolean("start_on_boot", v) }

    var bootedOnce: Boolean
        get() = p.getBoolean("booted_once", false)
        set(v) = p.edit { putBoolean("booted_once", v) }

    var checkUpdates: Boolean
        get() = p.getBoolean("check_updates", true)
        set(v) = p.edit { putBoolean("check_updates", v) }

    var betaUpdates: Boolean
        get() = p.getBoolean("beta_updates", false)
        set(v) = p.edit { putBoolean("beta_updates", v) }

    /** Timestamps (ms) of times Android stopped the proxy without the user asking. */
    val unexpectedStops: List<Long>
        get() = p.getString("unexpected_stops", "")!!.split(',').mapNotNull { it.toLongOrNull() }

    fun recordUnexpectedStop(now: Long = System.currentTimeMillis()) {
        val keep = unexpectedStops.filter { now - it < WEEK } + now
        p.edit { putString("unexpected_stops", keep.takeLast(50).joinToString(",")) }
    }

    fun stopsLastWeek(now: Long = System.currentTimeMillis()): Int = unexpectedStops.count { now - it < WEEK }

    companion object {
        const val WEEK = 7L * 24 * 3600 * 1000
    }
}
