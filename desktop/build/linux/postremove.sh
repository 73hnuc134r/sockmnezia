#!/bin/sh
# Removes the polkit rules sockmnezia installed for TUN mode without a password (docs: D37).
rm -f /etc/polkit-1/rules.d/49-sockmnezia-*.rules
exit 0
