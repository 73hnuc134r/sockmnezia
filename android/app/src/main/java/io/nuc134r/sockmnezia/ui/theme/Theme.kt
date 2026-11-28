package io.nuc134r.sockmnezia.ui.theme

import android.os.Build
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.ColorScheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import kotlinx.coroutines.flow.MutableStateFlow

/** The tonal scheme generated from the brand teal (#00695C), Material 3 roles (D43). */
private val Light = lightColorScheme(
    primary = Color(0xFF006A60), onPrimary = Color(0xFFFFFFFF),
    primaryContainer = Color(0xFF9EF2E4), onPrimaryContainer = Color(0xFF00201C),
    inversePrimary = Color(0xFF82D5C8),
    secondary = Color(0xFF4A635F), onSecondary = Color(0xFFFFFFFF),
    secondaryContainer = Color(0xFFCCE8E2), onSecondaryContainer = Color(0xFF05201C),
    tertiary = Color(0xFF456179), onTertiary = Color(0xFFFFFFFF),
    tertiaryContainer = Color(0xFFCCE5FF), onTertiaryContainer = Color(0xFF001E31),
    error = Color(0xFFBA1A1A), onError = Color(0xFFFFFFFF),
    errorContainer = Color(0xFFFFDAD6), onErrorContainer = Color(0xFF410002),
    background = Color(0xFFF4FBF8), onBackground = Color(0xFF161D1C),
    surface = Color(0xFFF4FBF8), onSurface = Color(0xFF161D1C),
    surfaceVariant = Color(0xFFDAE5E1), onSurfaceVariant = Color(0xFF3F4947),
    surfaceTint = Color(0xFF006A60),
    inverseSurface = Color(0xFF2B3230), inverseOnSurface = Color(0xFFECF2EF),
    outline = Color(0xFF6F7977), outlineVariant = Color(0xFFBEC9C6),
    surfaceDim = Color(0xFFD5DBD9), surfaceBright = Color(0xFFF4FBF8),
    surfaceContainerLowest = Color(0xFFFFFFFF), surfaceContainerLow = Color(0xFFEEF5F2),
    surfaceContainer = Color(0xFFE9EFED), surfaceContainerHigh = Color(0xFFE3EAE7),
    surfaceContainerHighest = Color(0xFFDDE4E1),
)

private val Dark = darkColorScheme(
    primary = Color(0xFF82D5C8), onPrimary = Color(0xFF003731),
    primaryContainer = Color(0xFF005048), onPrimaryContainer = Color(0xFF9EF2E4),
    inversePrimary = Color(0xFF006A60),
    secondary = Color(0xFFB1CCC6), onSecondary = Color(0xFF1C3531),
    secondaryContainer = Color(0xFF334B47), onSecondaryContainer = Color(0xFFCCE8E2),
    tertiary = Color(0xFFADCAE5), onTertiary = Color(0xFF153349),
    tertiaryContainer = Color(0xFF2D4961), onTertiaryContainer = Color(0xFFCCE5FF),
    error = Color(0xFFFFB4AB), onError = Color(0xFF690005),
    errorContainer = Color(0xFF93000A), onErrorContainer = Color(0xFFFFDAD6),
    background = Color(0xFF0E1513), onBackground = Color(0xFFDDE4E1),
    surface = Color(0xFF0E1513), onSurface = Color(0xFFDDE4E1),
    surfaceVariant = Color(0xFF3F4947), onSurfaceVariant = Color(0xFFBEC9C6),
    surfaceTint = Color(0xFF82D5C8),
    inverseSurface = Color(0xFFDDE4E1), inverseOnSurface = Color(0xFF2B3230),
    outline = Color(0xFF899390), outlineVariant = Color(0xFF3F4947),
    surfaceDim = Color(0xFF0E1513), surfaceBright = Color(0xFF343B39),
    surfaceContainerLowest = Color(0xFF090F0E), surfaceContainerLow = Color(0xFF161D1C),
    surfaceContainer = Color(0xFF1A2120), surfaceContainerHigh = Color(0xFF252B2A),
    surfaceContainerHighest = Color(0xFF303635),
)

/** The three surface levels the layout uses: the screen behind the islands, an island, and a row inside one.
 * In light the levels get lighter towards the front, in dark lighter too (so "front" never turns darker). */
@Immutable
data class Levels(val screen: Color, val island: Color, val row: Color, val nav: Color, val warn: Color, val onWarn: Color)

val LocalLevels = staticCompositionLocalOf { Levels(Color.Unspecified, Color.Unspecified, Color.Unspecified, Color.Unspecified, Color.Unspecified, Color.Unspecified) }

private fun levels(s: ColorScheme, dark: Boolean) = if (dark) {
    Levels(screen = s.surface, island = s.surfaceContainerLow, row = s.surfaceContainerHigh, nav = s.surfaceContainer, warn = Color(0xFF6A3B12), onWarn = Color(0xFFFFDCC2))
} else {
    Levels(screen = s.surfaceContainer, island = s.surface, row = s.surfaceContainerLowest, nav = s.surfaceContainerHigh, warn = Color(0xFFFFDCC2), onWarn = Color(0xFF2E1500))
}

/** UI preferences the theme reacts to; loaded from AppPrefs at startup and changed from Settings. */
object UiPrefs {
    /** Take colours from the wallpaper (Android 12+) instead of the teal scheme. Off by default (D43). */
    val wallpaperColors = MutableStateFlow(false)
}

@Composable
fun SockmneziaTheme(content: @Composable () -> Unit) {
    val dark = isSystemInDarkTheme()
    val ctx = LocalContext.current
    val wallpaper by UiPrefs.wallpaperColors.collectAsState()
    val scheme = when {
        wallpaper && Build.VERSION.SDK_INT >= 31 -> if (dark) dynamicDarkColorScheme(ctx) else dynamicLightColorScheme(ctx)
        dark -> Dark
        else -> Light
    }
    CompositionLocalProvider(LocalLevels provides levels(scheme, dark)) {
        MaterialTheme(colorScheme = scheme, content = content)
    }
}
