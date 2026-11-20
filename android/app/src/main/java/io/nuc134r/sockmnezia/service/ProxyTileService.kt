package io.nuc134r.sockmnezia.service

import android.service.quicksettings.Tile
import android.service.quicksettings.TileService
import io.nuc134r.sockmnezia.core.Core

/** Quick Settings tile: start/stop the proxy. Bound only by the system (BIND_QUICK_SETTINGS_TILE). */
class ProxyTileService : TileService() {
    override fun onStartListening() {
        super.onStartListening()
        update()
    }

    override fun onClick() {
        super.onClick()
        if (ProxyService.isRunning || Core.status.value.running) ProxyService.stop(this) else ProxyService.start(this)
        qsTile?.state = Tile.STATE_UNAVAILABLE
        qsTile?.updateTile()
    }

    private fun update() {
        val t = qsTile ?: return
        t.state = if (ProxyService.isRunning) Tile.STATE_ACTIVE else Tile.STATE_INACTIVE
        t.updateTile()
    }
}
