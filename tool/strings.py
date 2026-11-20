#!/usr/bin/env python3
"""Source of the app's UI strings: writes android/app/src/main/res/values{,-ru}/strings.xml.
Edit here, then run: python3 tool/strings.py   (CI checks the XML files are up to date: --check)."""
import os, sys

S = {  # key: (english, russian)
 "app_name": ("sockmnezia", "sockmnezia"),
 "channel_proxy": ("Proxy", "Прокси"),
 "back": ("Back", "Назад"),
 "tab_home": ("Home", "Главная"), "tab_servers": ("Servers", "Серверы"), "tab_routing": ("Routing", "Маршруты"), "tab_settings": ("Settings", "Настройки"),
 "title_servers": ("Own servers", "Свои серверы"), "title_routing": ("Routing", "Маршрутизация"), "title_settings": ("Settings", "Настройки"),
 "title_amnezia": ("Amnezia subscription", "Подписка Amnezia"), "title_health": ("Background work", "Работа в фоне"),
 "title_logs": ("Log", "Журнал"), "title_about": ("About", "О приложении"), "title_rule": ("Rule", "Правило"),
 "state_running": ("Proxy is running", "Прокси работает"), "state_stopped": ("Proxy is stopped", "Прокси остановлен"),
 "action_start": ("Start", "Запустить"), "action_stop": ("Stop", "Остановить"),
 "traffic": ("Sent %1$s · received %2$s", "Отправлено %1$s · получено %2$s"),
 "mode": ("Mode", "Режим"), "mode_amnezia": ("Amnezia", "Amnezia"), "mode_amnezia_own": ("Amnezia→own", "Amnezia→свой"), "mode_own": ("Own", "Свой"),
 "mode_amnezia_hint": ("Apps use the proxy, traffic leaves through Amnezia Premium.", "Приложения ходят через прокси, трафик выходит через Amnezia Premium."),
 "mode_amnezia_own_hint": ("Traffic leaves through your own server, which is reached through Amnezia. Exit address: your server.", "Трафик выходит через ваш сервер, к которому подключаемся через Amnezia. Внешний адрес — ваш сервер."),
 "mode_own_hint": ("Traffic goes straight to your own server. No Amnezia subscription needed.", "Трафик идёт прямо на ваш сервер. Подписка Amnezia не нужна."),
 "amnezia": ("Amnezia Premium", "Amnezia Premium"),
 "amnezia_not_set_up": ("Add your subscription key and the gateway key first.", "Сначала добавьте ключ подписки и ключ шлюза."),
 "amnezia_set_up": ("Set up", "Настроить"), "amnezia_settings": ("Subscription", "Подписка"),
 "location_default": ("Default location", "Локация по умолчанию"),
 "subscription_until": ("Subscription until %1$s", "Подписка до %1$s"),
 "refresh_config": ("Refresh config", "Обновить конфигурацию"), "fetch_config": ("Get config", "Получить конфигурацию"),
 "config_updated": ("Configuration updated", "Конфигурация обновлена"),
 "own_server": ("Own server", "Свой сервер"), "own_none": ("No server selected", "Сервер не выбран"), "own_choose": ("Choose", "Выбрать"),
 "proxy_for_apps": ("Proxy for other apps", "Прокси для других приложений"),
 "socks5": ("SOCKS5", "SOCKS5"), "http_proxy": ("HTTP proxy", "HTTP-прокси"),
 "username": ("Username", "Логин"), "password": ("Password", "Пароль"),
 "copy": ("Copy", "Копировать"), "copy_link": ("Copy socks5:// link", "Копировать ссылку socks5://"), "edit": ("Change", "Изменить"),
 "udp_on_hint": ("UDP is on.", "UDP включён."),
 "udp_off_hint": ("UDP is off (safer). Turn it on in Settings if an app needs it.", "UDP выключен (безопаснее). Включите в настройках, если приложению он нужен."),
 "check": ("Check", "Проверка"), "check_exit_ip": ("Exit address", "Внешний адрес"), "test_chain": ("Test chain", "Проверить цепочку"),
 "exit_ip": ("Exit address: %1$s", "Внешний адрес: %1$s"), "chain_ok": ("Chain works: %1$s, %2$d ms", "Цепочка работает: %1$s, %2$d мс"),
 "check_hint": ("Checks contact %1$s only when you tap them.".replace("%1$s", "an IP lookup service"), "Проверки обращаются к сервису определения IP только по нажатию."),
 "health_summary_bad": ("Android may stop the proxy in the background. Review the settings.", "Android может остановить прокси в фоне. Проверьте настройки."),
 "health_open": ("Review", "Проверить"),
 "subscription_key": ("Subscription key", "Ключ подписки"),
 "key_hint": ("Your own Premium subscription key (vpn://…) from the order e-mail or the personal dashboard.", "Ваш ключ подписки Premium (vpn://…) из письма о покупке или личного кабинета."),
 "key_saved": ("Key saved.", "Ключ сохранён."), "paste": ("Paste", "Вставить"), "save": ("Save", "Сохранить"), "remove": ("Remove", "Удалить"),
 "gateway_key": ("Gateway key", "Ключ шлюза"),
 "gateway_key_hint": ("Requests to Amnezia are encrypted with a key that is only inside the official app. Pick the official Amnezia APK (or download it) and sockmnezia finds the key in it. Nothing from the APK is run.", "Запросы к Amnezia шифруются ключом, который есть только в официальном приложении. Выберите официальный APK Amnezia (или скачайте) — sockmnezia найдёт в нём ключ. Ничего из APK не запускается."),
 "gateway_key_saved": ("Gateway key saved.", "Ключ шлюза сохранён."),
 "pick_official_apk": ("Choose official APK", "Выбрать официальный APK"),
 "download_official_apk": ("Download from GitHub", "Скачать с GitHub"),
 "paste_pem": ("Paste key text", "Вставить текст ключа"),
 "scanning_apk": ("Looking for the key…", "Ищем ключ…"), "downloading_apk": ("Downloading the official APK…", "Скачиваем официальный APK…"),
 "fetching": ("Asking Amnezia…", "Запрос к Amnezia…"), "config": ("Configuration", "Конфигурация"),
 "device_slot_warning": ("This phone takes one device slot of your subscription (it keeps the same one on every refresh). Free slots in the Amnezia personal dashboard.", "Телефон занимает одно устройство подписки (одно и то же при каждом обновлении). Освободить можно в личном кабинете Amnezia."),
 "config_status": ("Location %1$s, updated %2$s", "Локация %1$s, обновлено %2$s"),
 "captcha_title": ("Amnezia asks for a CAPTCHA", "Amnezia просит капчу"), "captcha_answer": ("Characters in the picture", "Символы с картинки"),
 "ok": ("OK", "OK"), "cancel": ("Cancel", "Отмена"),
 "servers_hint": ("Your own Xray-compatible server: vless, vmess, trojan, shadowsocks, socks or http link, or an Xray outbound JSON. To chain through Amnezia it must use a TCP transport (tcp, ws, grpc, httpupgrade, xhttp).", "Ваш Xray-совместимый сервер: ссылка vless, vmess, trojan, shadowsocks, socks или http, либо Xray outbound JSON. Для цепочки через Amnezia нужен TCP-транспорт (tcp, ws, grpc, httpupgrade, xhttp)."),
 "add_server": ("Add server", "Добавить сервер"), "server_link": ("Link or JSON", "Ссылка или JSON"),
 "server_link_hint": ("The link contains your server's password; it is stored encrypted on this phone only.", "В ссылке есть пароль вашего сервера; она хранится только на этом телефоне в зашифрованном виде."),
 "name_optional": ("Name (optional)", "Название (необязательно)"), "add": ("Add", "Добавить"), "server_added": ("Server added", "Сервер добавлен"),
 "rename": ("Rename", "Переименовать"), "delete": ("Delete", "Удалить"), "delete_server_q": ("Delete %1$s?", "Удалить %1$s?"),
 "routing_hint": ("Rules are checked from top to bottom; the first match decides. Proxy = through the chain, Direct = from this phone's own network, Block = drop.", "Правила проверяются сверху вниз, срабатывает первое подходящее. Прокси — через цепочку, Напрямую — из сети телефона, Блок — отбросить."),
 "add_rule": ("Add rule", "Добавить"), "presets": ("Presets", "Шаблоны"),
 "import_clipboard": ("Import from clipboard", "Импорт из буфера"), "export_clipboard": ("Copy rules", "Копировать правила"),
 "rules_imported": ("Rules imported", "Правила импортированы"), "rules_copied": ("Rules copied", "Правила скопированы"),
 "locked_rules": ("Always first (cannot be changed)", "Всегда первые (не меняются)"),
 "locked_ip": ("Private and loopback addresses", "Частные и локальные адреса"), "locked_domain": ("Local names", "Локальные имена"),
 "rule_n": ("Rule %1$d", "Правило %1$d"),
 "out_proxy": ("Proxy", "Прокси"), "out_direct": ("Direct", "Напрямую"), "out_block": ("Block", "Блок"),
 "move_up": ("Up", "Выше"), "move_down": ("Down", "Ниже"),
 "default_route": ("Everything else", "Всё остальное"), "default_route_hint": ("Goes through the proxy chain. There is never a fallback to a direct connection.", "Идёт через цепочку прокси. Прямого подключения «на всякий случай» нет."),
 "domain_strategy": ("Domain strategy", "Стратегия доменов"),
 "ds_asis": ("As is (recommended)", "Как есть (рекомендуется)"), "ds_ipifnonmatch": ("Resolve if no rule matched", "Разрешать, если правило не подошло"), "ds_ipondemand": ("Resolve for IP rules", "Разрешать для IP-правил"),
 "domain_strategy_hint": ("Resolving makes this phone look up domains with its normal DNS, which the local network can see.", "При разрешении телефон запрашивает домены через обычный DNS — это видно в локальной сети."),
 "route_tester": ("Where does it go?", "Куда пойдёт?"), "host_or_ip": ("Domain or IP", "Домен или IP"), "port": ("Port", "Порт"), "test": ("Check", "Проверить"),
 "verdict_locked": ("locked rule", "закреплённое правило"), "verdict_default": ("no rule matched", "ни одно правило не подошло"),
 "geo_files": ("Geo files", "Геофайлы"), "geo_custom": ("Custom URL", "Свой адрес"),
 "geo_status_ok": ("Downloaded %1$s", "Загружены %1$s"), "geo_status_missing": ("Not downloaded. Rules with geosite:/geoip: need them.", "Не загружены. Они нужны правилам geosite:/geoip:."),
 "geo_update_now": ("Update now", "Обновить сейчас"), "downloading": ("Downloading…", "Загрузка…"), "geo_updated": ("Geo files updated", "Геофайлы обновлены"),
 "geo_auto": ("Update weekly", "Обновлять раз в неделю"), "geo_auto_hint": ("Over Wi-Fi; through the proxy while it runs.", "По Wi-Fi; через прокси, если он запущен."),
 "preset_apply_q": ("Replace your rules with this preset or add it to them?", "Заменить ваши правила этим шаблоном или добавить к ним?"),
 "preset_replace": ("Replace", "Заменить"), "preset_append": ("Add", "Добавить"),
 "preset_global": ("Everything through the proxy", "Всё через прокси"), "preset_bypass_ru": ("Russian sites directly", "Российские сайты напрямую"),
 "preset_block_ads": ("Block ads", "Блокировать рекламу"), "preset_bypass_bt": ("BitTorrent directly", "BitTorrent напрямую"),
 "preset_block_quic": ("Block QUIC", "Блокировать QUIC"), "preset_unknown": ("Preset", "Шаблон"),
 "rule_name": ("Name", "Название"), "rule_outbound": ("Send to", "Отправлять"),
 "direct_hint": ("Direct connections leave from this phone's own network and show its real address.", "Прямые подключения идут из сети телефона и показывают его настоящий адрес."),
 "rule_enabled": ("Enabled", "Включено"),
 "rule_domains": ("Domains", "Домены"),
 "rule_domains_hint": ("One per line: example.com (part of the name), domain:example.com (with subdomains), full:example.com, regexp:…, geosite:tag.", "По одному в строке: example.com (часть имени), domain:example.com (с поддоменами), full:example.com, regexp:…, geosite:тег."),
 "add_geosite": ("Add geosite tag", "Добавить тег geosite"), "add_geoip": ("Add geoip tag", "Добавить тег geoip"),
 "rule_ips": ("IP addresses", "IP-адреса"), "rule_more": ("More conditions", "Другие условия"),
 "rule_port": ("Ports", "Порты"), "rule_network": ("Network", "Сеть"), "any": ("Any", "Любая"), "rule_protocol": ("Detected protocol", "Распознанный протокол"),
 "search": ("Search", "Поиск"),
 "local_proxy": ("Local proxy", "Локальный прокси"), "socks_port": ("SOCKS5 port", "Порт SOCKS5"), "random": ("Random", "Случайный"),
 "show": ("Show password", "Показать пароль"), "hide": ("Hide password", "Скрыть пароль"),
 "credential_rule": ("Use 1–255 printable latin characters without spaces.", "Используйте 1–255 печатных латинских символов без пробелов."),
 "password_weak": ("Short password: other apps on this phone could guess it.", "Короткий пароль: другие приложения на телефоне могут его подобрать."),
 "credentials_hint": ("Every app that uses the proxy needs the username and password. They change only when you change them here.", "Логин и пароль нужны каждому приложению, которое ходит через прокси. Меняются только когда вы меняете их здесь."),
 "udp": ("UDP", "UDP"),
 "udp_hint": ("Only if an app needs it. The app should tell the proxy its own port when it opens UDP (most do).", "Только если приложению нужно. Приложение должно сообщать прокси свой порт при открытии UDP (большинство так делает)."),
 "http_hint": ("A second port for apps that only speak HTTP proxy; same credentials.", "Второй порт для приложений, умеющих только HTTP-прокси; те же логин и пароль."),
 "http_port": ("HTTP port", "Порт HTTP"),
 "share_lan": ("Share on local network", "Раздавать в локальную сеть"),
 "share_lan_hint": ("Other devices on your Wi-Fi or hotspot can use the proxy (with the password).", "Другие устройства в вашей Wi-Fi-сети или точке доступа смогут пользоваться прокси (с паролем)."),
 "share_lan_none": ("No local network address right now.", "Сейчас нет адреса в локальной сети."),
 "share_lan_warning": ("Anyone on this network can try to connect. Keep a long password.", "Подключиться может попробовать любой в этой сети. Используйте длинный пароль."),
 "saved": ("Saved", "Сохранено"), "app_settings": ("App", "Приложение"),
 "start_on_boot": ("Start after reboot", "Запускать после перезагрузки"), "start_on_boot_hint": ("If the proxy was running before.", "Если прокси был запущен до перезагрузки."),
 "fetch_via_own": ("Reach Amnezia through own server", "Связь с Amnezia через свой сервер"),
 "fetch_via_own_hint": ("Use when the Amnezia gateway is blocked on your network.", "Если шлюз Amnezia заблокирован в вашей сети."),
 "log_level": ("Log detail", "Подробность журнала"),
 "forget": ("Delete all data", "Удалить все данные"), "forget_q": ("Delete all data?", "Удалить все данные?"),
 "forget_text": ("The key, servers, rules and settings are deleted from this phone. The device slot stays used until you free it in the Amnezia dashboard.", "Ключ, серверы, правила и настройки удаляются с телефона. Устройство в подписке остаётся занятым, пока вы не освободите его в кабинете Amnezia."),
 "forgotten": ("All data deleted", "Все данные удалены"),
 "health_intro": ("A proxy for other apps only helps while it runs. These settings decide whether Android lets it run in the background.", "Прокси полезен, только пока работает. Эти настройки решают, даст ли Android ему работать в фоне."),
 "health_fix": ("Open settings", "Открыть настройки"), "health_no_screen": ("This phone has no such settings screen.", "На этом телефоне нет такого экрана настроек."),
 "hc_battery": ("Battery optimization is on", "Включена оптимизация батареи"), "hc_battery_ok": ("Battery optimization is off", "Оптимизация батареи выключена"),
 "hc_battery_text": ("Android may pause the proxy when the screen is off. Allow sockmnezia to run unrestricted.", "Android может приостанавливать прокси при выключенном экране. Разрешите sockmnezia работать без ограничений."),
 "hc_restricted": ("Background use is restricted", "Работа в фоне ограничена"), "hc_restricted_ok": ("Background use is allowed", "Работа в фоне разрешена"),
 "hc_restricted_text": ("The app's battery setting is \"Restricted\"; Android stops it in the background. Choose \"Unrestricted\".", "В настройках батареи для приложения выбрано «Ограничено» — Android останавливает его в фоне. Выберите «Без ограничений»."),
 "hc_notifications": ("Notifications are off", "Уведомления выключены"), "hc_notifications_ok": ("Notifications are on", "Уведомления включены"),
 "hc_notifications_text": ("The proxy still runs, but you will not see that it is on or be able to stop it from the notification.", "Прокси работает, но вы не увидите, что он включён, и не сможете остановить его из уведомления."),
 "hc_datasaver": ("Data Saver blocks background data", "Экономия трафика блокирует фон"), "hc_datasaver_ok": ("Data Saver does not block the app", "Экономия трафика не мешает"),
 "hc_datasaver_text": ("With Data Saver on, the proxy has no network in the background. Allow unrestricted data for sockmnezia.", "При экономии трафика у прокси нет сети в фоне. Разрешите sockmnezia неограниченный трафик."),
 "hc_hibernation": ("App may be paused when unused", "Приложение могут приостановить"), "hc_hibernation_ok": ("App is not paused when unused", "Приложение не приостанавливается"),
 "hc_hibernation_text": ("Turn off \"Pause app activity if unused\" so Android does not reset the app.", "Выключите «Приостанавливать работу, если не используется», чтобы Android не сбрасывал приложение."),
 "hc_powersave": ("Battery Saver is on", "Включён режим энергосбережения"),
 "hc_powersave_text": ("Battery Saver may slow down or stop background work.", "Энергосбережение может замедлять или останавливать работу в фоне."),
 "hc_oem": ("This phone's maker stops background apps", "Производитель телефона останавливает фоновые приложения"),
 "hc_oem_text": ("Besides the Android settings, allow autostart / background activity for sockmnezia in the maker's own settings.", "Кроме настроек Android, разрешите автозапуск / работу в фоне для sockmnezia в настройках производителя."),
 "hc_boot": ("Start after reboot not seen yet", "Запуск после перезагрузки ещё не проверен"),
 "hc_boot_text": ("After the next reboot this turns into a check that it worked.", "После следующей перезагрузки здесь будет видно, сработал ли запуск."),
 "hc_kills": ("Android stopped the proxy", "Android останавливал прокси"),
 "logs_hint": ("Addresses, IDs and passwords are masked.", "Адреса, идентификаторы и пароли скрыты."),
 "share": ("Share", "Поделиться"), "clear": ("Clear", "Очистить"),
 "version": ("Version %1$s", "Версия %1$s"),
 "unofficial": ("Unofficial app, not affiliated with Amnezia. Use only your own subscription key; using it outside the official app may be against Amnezia's terms.", "Неофициальное приложение, не связано с Amnezia. Используйте только свой ключ подписки; использование вне официального приложения может противоречить условиям Amnezia."),
 "licenses": ("Licenses", "Лицензии"),
 "licenses_text": ("sockmnezia: MIT License.\nXray-core (github.com/XTLS/Xray-core): Mozilla Public License 2.0.\nGo, AndroidX, Jetpack Compose, Kotlin and kotlinx libraries: BSD-3-Clause / Apache License 2.0.", "sockmnezia: лицензия MIT.\nXray-core (github.com/XTLS/Xray-core): Mozilla Public License 2.0.\nGo, AndroidX, Jetpack Compose, Kotlin и kotlinx: BSD-3-Clause / Apache License 2.0."),
 "notif_starting": ("Starting…", "Запуск…"),
 "notif_running": ("%1$s:%2$d · ↑%3$s ↓%4$s", "%1$s:%2$d · ↑%3$s ↓%4$s"),
 "notif_failed": ("Could not start: %1$s", "Не удалось запустить: %1$s"),
 "err_invalid_key": ("This is not a Premium subscription key.", "Это не ключ подписки Premium."),
 "err_no_gateway_key": ("No gateway key yet: pick the official Amnezia APK in Subscription.", "Нет ключа шлюза: выберите официальный APK Amnezia в разделе «Подписка»."),
 "err_gateway_wrong_key": ("The gateway did not accept any key (old APK?), or something on the network answered instead of it.", "Шлюз не принял ни один ключ (старый APK?) или вместо него ответил кто-то в сети."),
 "err_gateway_unreachable": ("The Amnezia gateway is unreachable. If it is blocked, turn on \"Reach Amnezia through own server\".", "Шлюз Amnezia недоступен. Если он заблокирован, включите «Связь с Amnezia через свой сервер»."),
 "err_device_limit": ("All device slots of the subscription are used. Free one in the Amnezia personal dashboard.", "Все устройства подписки заняты. Освободите одно в личном кабинете Amnezia."),
 "err_trial_used": ("The trial was already used.", "Пробный период уже использован."),
 "err_subscription_expired": ("The subscription is not active or has expired.", "Подписка не активна или закончилась."),
 "err_subscription_not_active": ("The subscription is not active (payment required).", "Подписка не активна (нужна оплата)."),
 "err_captcha_required": ("Amnezia asks for a CAPTCHA.", "Amnezia просит капчу."),
 "err_captcha_invalid": ("Wrong CAPTCHA answer.", "Неверный ответ на капчу."),
 "err_rate_limit": ("Too many requests. Try again later.", "Слишком много запросов. Попробуйте позже."),
 "err_update_required": ("Amnezia asks for a newer app version. Update sockmnezia.", "Amnezia требует новую версию приложения. Обновите sockmnezia."),
 "err_gateway_error": ("Amnezia gateway error.", "Ошибка шлюза Amnezia."),
 "err_no_config": ("No Amnezia configuration yet. Get it in Subscription.", "Ещё нет конфигурации Amnezia. Получите её в разделе «Подписка»."),
 "err_no_own_server": ("Choose an own server first.", "Сначала выберите свой сервер."),
 "err_port_in_use": ("The port is in use by another app. Choose another port.", "Порт занят другим приложением. Выберите другой."),
 "err_invalid_settings": ("Invalid setting.", "Неверная настройка."),
 "err_geo_missing": ("Rules use geosite:/geoip: but geo files are not downloaded (Routing → Geo files).", "Правила используют geosite:/geoip:, но геофайлы не загружены (Маршруты → Геофайлы)."),
 "err_unsupported_link": ("This server type cannot be used.", "Такой сервер использовать нельзя."),
 "err_invalid_link": ("Not a valid server link.", "Неверная ссылка на сервер."),
 "err_not_running": ("Start the proxy first.", "Сначала запустите прокси."),
 "err_network": ("Network error.", "Ошибка сети."),
 "err_internal": ("Something went wrong.", "Что-то пошло не так."),
}

P = {  # plurals: key -> ({quantity: en}, {quantity: ru})
 "gateway_keys_found": ({"one": "Found %d key", "other": "Found %d keys"},
                        {"one": "Найден %d ключ", "few": "Найдено %d ключа", "many": "Найдено %d ключей", "other": "Найдено %d ключа"}),
 "gateway_keys_found_release": ({"one": "Found %1$d key in release %2$s", "other": "Found %1$d keys in release %2$s"},
                        {"one": "Найден %1$d ключ (версия %2$s)", "few": "Найдено %1$d ключа (версия %2$s)", "many": "Найдено %1$d ключей (версия %2$s)", "other": "Найдено %1$d ключа (версия %2$s)"}),
 "auth_failures": ({"one": "%d connection attempt with a wrong or no password", "other": "%d connection attempts with a wrong or no password"},
                   {"one": "%d попытка подключения без пароля или с неверным", "few": "%d попытки подключения без пароля или с неверным",
                    "many": "%d попыток подключения без пароля или с неверным", "other": "%d попытки подключения без пароля или с неверным"}),
 "hc_kills_text": ({"one": "%d time in the last 7 days. Fix the items above.", "other": "%d times in the last 7 days. Fix the items above."},
                   {"one": "%d раз за последние 7 дней. Исправьте пункты выше.", "few": "%d раза за последние 7 дней. Исправьте пункты выше.",
                    "many": "%d раз за последние 7 дней. Исправьте пункты выше.", "other": "%d раза за последние 7 дней. Исправьте пункты выше."}),
}

def esc(s):
    s = s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
    s = s.replace("\\", "\\\\").replace("'", "\\'").replace('"', '\\"').replace("\n", "\\n")
    if s.startswith("@") or s.startswith("?"):
        s = "\\" + s
    return s

def render(lang):
    i = 0 if lang == "en" else 1
    out = ['<?xml version="1.0" encoding="utf-8"?>', "<!-- Generated by tool/strings.py; edit there. -->", "<resources>"]
    for k, v in S.items():
        attr = ' translatable="false"' if k == "app_name" and lang == "en" else ""
        if k == "app_name" and lang != "en":
            continue
        out.append(f'    <string name="{k}"{attr}>{esc(v[i])}</string>')
    for k, v in P.items():
        out.append(f'    <plurals name="{k}">')
        for q, t in v[i].items():
            out.append(f'        <item quantity="{q}">{esc(t)}</item>')
        out.append("    </plurals>")
    out.append("</resources>")
    return "\n".join(out) + "\n"

root = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "android", "app", "src", "main", "res")
check = "--check" in sys.argv
bad = False
for lang, d in (("en", "values"), ("ru", "values-ru")):
    path = os.path.join(root, d, "strings.xml")
    text = render(lang)
    if check:
        if not os.path.exists(path) or open(path, encoding="utf-8").read() != text:
            print("out of date:", path); bad = True
    else:
        os.makedirs(os.path.dirname(path), exist_ok=True)
        open(path, "w", encoding="utf-8").write(text)
sys.exit(1 if bad else 0)
