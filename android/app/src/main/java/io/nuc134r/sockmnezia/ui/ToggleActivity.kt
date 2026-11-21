package io.nuc134r.sockmnezia.ui

import android.app.Activity
import android.os.Bundle
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.service.ProxyService

/** Launcher shortcut target: toggles the proxy and closes at once. */
class ToggleActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (ProxyService.isRunning || Core.status.value.running) ProxyService.stop(this) else ProxyService.start(this)
        finish()
    }
}
