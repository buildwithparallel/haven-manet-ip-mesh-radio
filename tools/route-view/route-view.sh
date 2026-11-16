#!/bin/sh
# Live route view for a screen or a video: how traffic from this node (usually
# the gate) reaches another node, redrawn every second.
#
#   sh route-view.sh <node>        for example: sh route-view.sh blue
#
# Ctrl-C stops it and puts the terminal back.
DIR=$(cd "$(dirname "$0")" && pwd)
restore() { printf '\033[?7h\033[?25h\033[0m\033[?1049l'; }
trap 'restore; exit 0' INT TERM
ucode "$DIR/route-view.uc" "$@"
restore
