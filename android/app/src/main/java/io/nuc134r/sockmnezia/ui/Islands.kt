package io.nuc134r.sockmnezia.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.ui.theme.LocalLevels

/** Gap between the rows of a group inside an island. */
val GroupGap = 2.dp

/** An island: a rounded block on the screen background holding a titled group (D43). */
@Composable
fun Island(
    modifier: Modifier = Modifier,
    title: String? = null,
    trailing: (@Composable RowScope.() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(
        modifier
            .fillMaxWidth()
            .padding(horizontal = 12.dp, vertical = 6.dp)
            .clip(RoundedCornerShape(28.dp))
            .background(LocalLevels.current.island)
            .padding(start = 8.dp, end = 8.dp, top = if (title != null || trailing != null) 14.dp else 8.dp, bottom = 8.dp),
        verticalArrangement = Arrangement.spacedBy(GroupGap),
    ) {
        if (title != null || trailing != null) {
            Row(Modifier.fillMaxWidth().padding(start = 8.dp, end = 8.dp, bottom = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                if (title != null) IslandTitle(title, Modifier.weight(1f))
                trailing?.invoke(this)
            }
        }
        content()
    }
}

@Composable
fun IslandTitle(text: String, modifier: Modifier = Modifier) {
    Text(text, modifier, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
}

/** The shape of row [index] of [count] in a group: round outer corners, tight inner ones. */
fun groupShape(index: Int, count: Int): Shape {
    val big = 20.dp
    val small = 6.dp
    return when {
        count <= 1 -> RoundedCornerShape(big)
        index == 0 -> RoundedCornerShape(big, big, small, small)
        index == count - 1 -> RoundedCornerShape(small, small, big, big)
        else -> RoundedCornerShape(small)
    }
}

/** One row of a group in an island. [selected] fills it with the primary container. */
@Composable
fun GroupRow(
    index: Int,
    count: Int,
    modifier: Modifier = Modifier,
    selected: Boolean = false,
    onClick: (() -> Unit)? = null,
    content: @Composable RowScope.() -> Unit,
) {
    val bg = if (selected) MaterialTheme.colorScheme.primaryContainer else LocalLevels.current.row
    Row(
        modifier
            .fillMaxWidth()
            .clip(groupShape(index, count))
            .background(bg)
            .then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier)
            .heightIn(min = 56.dp)
            .padding(horizontal = 14.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        content = content,
    )
}

/** A two-line text block for a row: headline and optional supporting text. */
@Composable
fun RowText(title: String, modifier: Modifier = Modifier, subtitle: String? = null, below: (@Composable () -> Unit)? = null) {
    Column(modifier, verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(title, style = MaterialTheme.typography.bodyLarge)
        if (subtitle != null) {
            Text(subtitle, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        below?.invoke()
    }
}

/** A small rounded label: protocol, security, port, outbound, rule condition. */
@Composable
fun Pill(text: String, container: Color, content: Color, modifier: Modifier = Modifier, mono: Boolean = false) {
    Text(
        text,
        modifier = modifier.background(container, RoundedCornerShape(6.dp)).padding(horizontal = 6.dp, vertical = 1.dp),
        color = content, fontSize = 11.sp, lineHeight = 16.sp, maxLines = 1,
        fontWeight = if (mono) FontWeight.Normal else FontWeight.SemiBold,
        fontFamily = if (mono) FontFamily.Monospace else null,
    )
}

/** Two-letter country code in a tiny box. */
@Composable
fun CountryCode(code: String, modifier: Modifier = Modifier, container: Color = MaterialTheme.colorScheme.tertiaryContainer, content: Color = MaterialTheme.colorScheme.onTertiaryContainer) {
    Text(
        code.uppercase(),
        modifier = modifier.background(container, RoundedCornerShape(4.dp)).padding(horizontal = 4.dp),
        color = content, fontSize = 10.sp, lineHeight = 14.sp, fontWeight = FontWeight.Bold, maxLines = 1,
    )
}

/** Country and ping in one pill: "[NL] 118 ms". [text] replaces the ping (e.g. a city or "Unreachable");
 * [failed] paints it in the error colours. Nothing is drawn when there is neither a country nor a value. */
@Composable
fun CountryPing(country: String?, ms: Long?, modifier: Modifier = Modifier, failed: Boolean = false, text: String? = null, onPrimary: Boolean = false) {
    val label = text ?: when {
        failed -> stringResource(R.string.location_ping_failed)
        ms != null && ms > 0 -> stringResource(R.string.conn_ping, ms.toInt())
        else -> null
    }
    if (country.isNullOrBlank() && label == null) return
    val bg = when {
        failed -> MaterialTheme.colorScheme.errorContainer
        onPrimary -> Color.White.copy(alpha = 0.6f)
        else -> MaterialTheme.colorScheme.surfaceContainerHigh
    }
    val fg = when {
        failed -> MaterialTheme.colorScheme.onErrorContainer
        onPrimary -> MaterialTheme.colorScheme.onPrimaryContainer
        else -> MaterialTheme.colorScheme.onSurfaceVariant
    }
    Row(
        modifier.background(bg, RoundedCornerShape(8.dp)).padding(start = if (country.isNullOrBlank()) 8.dp else 4.dp, end = 8.dp, top = 3.dp, bottom = 3.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        if (!country.isNullOrBlank()) CountryCode(country)
        if (label != null) Text(label, color = fg, fontSize = 12.sp, lineHeight = 16.sp, fontWeight = FontWeight.Medium, maxLines = 1)
    }
}

/** A tip or warning row inside an island (tonal background, icon-free). */
@Composable
fun TipRow(text: String, modifier: Modifier = Modifier, warn: Boolean = false) {
    Box(
        modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(16.dp))
            .background(if (warn) MaterialTheme.colorScheme.errorContainer else MaterialTheme.colorScheme.surfaceContainerHigh)
            .padding(horizontal = 14.dp, vertical = 10.dp),
    ) {
        Text(
            text, style = MaterialTheme.typography.bodyMedium,
            color = if (warn) MaterialTheme.colorScheme.onErrorContainer else MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

/** An icon from Material path data (for the few icons the core set lacks). */
fun pathIcon(name: String, d: String): androidx.compose.ui.graphics.vector.ImageVector =
    androidx.compose.ui.graphics.vector.ImageVector.Builder(name, 24.dp, 24.dp, 24f, 24f).apply {
        addPath(androidx.compose.ui.graphics.vector.PathParser().parsePathString(d).toNodes(), fill = androidx.compose.ui.graphics.SolidColor(Color.Black))
    }.build()

/** Material "dns" (two stacked servers). */
val DnsIcon by lazy {
    pathIcon(
        "Dns",
        "M20,13H4c-0.55,0 -1,0.45 -1,1v6c0,0.55 0.45,1 1,1h16c0.55,0 1,-0.45 1,-1v-6c0,-0.55 -0.45,-1 -1,-1zM7,19c-1.1,0 -2,-0.9 -2,-2s0.9,-2 2,-2 2,0.9 2,2 -0.9,2 -2,2z" +
            "M20,3H4c-0.55,0 -1,0.45 -1,1v6c0,0.55 0.45,1 1,1h16c0.55,0 1,-0.45 1,-1V4c0,-0.55 -0.45,-1 -1,-1zM7,9c-1.1,0 -2,-0.9 -2,-2s0.9,-2 2,-2 2,0.9 2,2 -0.9,2 -2,2z",
    )
}
