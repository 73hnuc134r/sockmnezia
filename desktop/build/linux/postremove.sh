#!/bin/sh
# Removes the TUN service the app installs for TUN mode (D40), with its files and firewall rules, so
# nothing of sockmnezia keeps running or blocking after the package is removed.
if [ -f /etc/systemd/system/sockmnezia-tun.service ]; then
  systemctl disable --now sockmnezia-tun 2>/dev/null || true
  rm -f /etc/systemd/system/sockmnezia-tun.service
  systemctl daemon-reload 2>/dev/null || true
fi
nft delete table inet sockmnezia 2>/dev/null || true
rm -rf /var/lib/sockmnezia /usr/local/libexec/sockmnezia
rm -f /etc/polkit-1/rules.d/49-sockmnezia-*.rules
exit 0
