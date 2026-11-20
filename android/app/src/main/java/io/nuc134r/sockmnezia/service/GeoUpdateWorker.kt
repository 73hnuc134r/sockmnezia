package io.nuc134r.sockmnezia.service

import android.content.Context
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.NetworkType
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import io.nuc134r.sockmnezia.core.Core
import java.util.concurrent.TimeUnit

/** Weekly geo file update over an unmetered network, when the user turned it on. */
class GeoUpdateWorker(ctx: Context, params: WorkerParameters) : CoroutineWorker(ctx, params) {
    override suspend fun doWork(): Result {
        Core.init(applicationContext)
        val s = Core.settings()
        if (!s.options.geoAutoUpdate) return Result.success()
        return try {
            Core.updateGeo()
            Result.success()
        } catch (e: Exception) {
            Core.log("warning", "geo auto-update failed: ${Core.asCoreException(e).error.code}")
            Result.retry()
        }
    }

    companion object {
        private const val NAME = "geo-update"

        fun schedule(context: Context) {
            val req = PeriodicWorkRequestBuilder<GeoUpdateWorker>(7, TimeUnit.DAYS)
                .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.UNMETERED).build())
                .build()
            WorkManager.getInstance(context).enqueueUniquePeriodicWork(NAME, ExistingPeriodicWorkPolicy.KEEP, req)
        }
    }
}
