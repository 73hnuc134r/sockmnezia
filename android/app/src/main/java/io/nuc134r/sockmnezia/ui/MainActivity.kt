package io.nuc134r.sockmnezia.ui

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.animation.EnterTransition
import androidx.compose.animation.ExitTransition
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.List
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import io.nuc134r.sockmnezia.R
import io.nuc134r.sockmnezia.core.Core
import io.nuc134r.sockmnezia.ui.screens.AboutScreen
import io.nuc134r.sockmnezia.ui.screens.AmneziaScreen
import io.nuc134r.sockmnezia.ui.screens.ChainEditScreen
import io.nuc134r.sockmnezia.ui.screens.HealthScreen
import io.nuc134r.sockmnezia.ui.screens.HomeScreen
import io.nuc134r.sockmnezia.ui.screens.LogsScreen
import io.nuc134r.sockmnezia.ui.screens.RoutingScreen
import io.nuc134r.sockmnezia.ui.screens.RuleEditorScreen
import io.nuc134r.sockmnezia.ui.screens.ServerEditScreen
import io.nuc134r.sockmnezia.ui.screens.SettingsScreen
import io.nuc134r.sockmnezia.ui.theme.SockmneziaTheme

object Routes {
    const val HOME = "home"
    const val ROUTING = "routing"
    const val SETTINGS = "settings"
    const val AMNEZIA = "amnezia"
    const val HEALTH = "health"
    const val LOGS = "logs"
    const val ABOUT = "about"
    const val RULE = "rule/{index}"
    const val CHAIN = "chain/{id}"
    const val SELECT = "select"
    const val SERVER = "server/{id}"
    fun server(id: String) = "server/$id"
    /** Chain route argument for a new chain. */
    const val NEW = "new"
    fun chain(id: String) = "chain/$id"
    fun rule(index: Int) = "rule/$index"
    val tabs = setOf(HOME, ROUTING, SETTINGS)
}

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        runCatching { Core.init(applicationContext) }
        setContent { SockmneziaTheme { AppRoot() } }
    }

    override fun onResume() {
        super.onResume()
        runCatching { Core.refreshStatus() }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AppRoot() {
    val nav = rememberNavController()
    val snackbar = remember { SnackbarHostState() }
    val entry by nav.currentBackStackEntryAsState()
    val route = entry?.destination?.route ?: Routes.HOME
    val title = when (route) {
        Routes.ROUTING -> R.string.title_routing
        Routes.SETTINGS -> R.string.title_settings
        Routes.AMNEZIA -> R.string.title_amnezia
        Routes.HEALTH -> R.string.title_health
        Routes.LOGS -> R.string.title_logs
        Routes.ABOUT -> R.string.title_about
        Routes.RULE -> R.string.title_rule
        Routes.CHAIN -> R.string.title_chain
        Routes.SELECT -> R.string.selector_title
        Routes.SERVER -> R.string.title_server
        else -> R.string.app_name
    }
    CompositionLocalProvider(LocalSnackbar provides snackbar) {
        Scaffold(
            topBar = {
                TopAppBar(
                    title = { Text(stringResource(title)) },
                    navigationIcon = {
                        if (route !in Routes.tabs) {
                            IconButton(onClick = { nav.popBackStack() }) {
                                Icon(Icons.AutoMirrored.Filled.ArrowBack, stringResource(R.string.back))
                            }
                        }
                    },
                )
            },
            bottomBar = {
                if (route in Routes.tabs) {
                  Column {
                    if (route == Routes.HOME) ConnectionBar(nav)
                    NavigationBar {
                        listOf(
                            Triple(Routes.HOME, Icons.Default.Home, R.string.tab_home),
                            Triple(Routes.ROUTING, Icons.AutoMirrored.Filled.List, R.string.tab_routing),
                            Triple(Routes.SETTINGS, Icons.Default.Settings, R.string.tab_settings),
                        ).forEach { (r, icon, label) ->
                            NavigationBarItem(
                                selected = route == r,
                                onClick = {
                                    nav.navigate(r) {
                                        popUpTo(Routes.HOME) { saveState = true }
                                        launchSingleTop = true
                                        restoreState = true
                                    }
                                },
                                icon = { Icon(icon, null) },
                                label = { Text(stringResource(label)) },
                            )
                        }
                    }
                  }
                }
            },
            snackbarHost = { SnackbarHost(snackbar) },
        ) { pad ->
            NavHost(
                nav, startDestination = Routes.HOME, modifier = Modifier.padding(pad),
                enterTransition = { EnterTransition.None },
                exitTransition = { ExitTransition.None },
                popEnterTransition = { EnterTransition.None },
                popExitTransition = { ExitTransition.None },
            ) {
                composable(Routes.HOME) { HomeScreen(nav) }
                composable(Routes.ROUTING) { RoutingScreen(nav) }
                composable(Routes.SETTINGS) { SettingsScreen(nav) }
                composable(Routes.AMNEZIA) { AmneziaScreen() }
                composable(Routes.HEALTH) { HealthScreen() }
                composable(Routes.LOGS) { LogsScreen() }
                composable(Routes.ABOUT) { AboutScreen() }
                composable(Routes.SELECT) { SelectScreen(nav) }
                composable(Routes.SERVER, arguments = listOf(navArgument("id") { type = NavType.StringType })) {
                    ServerEditScreen(nav, it.arguments?.getString("id") ?: Routes.NEW)
                }
                composable(Routes.CHAIN, arguments = listOf(navArgument("id") { type = NavType.StringType })) {
                    ChainEditScreen(nav, it.arguments?.getString("id") ?: Routes.NEW)
                }
                composable(Routes.RULE, arguments = listOf(navArgument("index") { type = NavType.IntType })) {
                    RuleEditorScreen(nav, it.arguments?.getInt("index") ?: -1)
                }
            }
        }
    }
}
