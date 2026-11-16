#!/bin/sh
# Install the offline base map into the dump1090 SkyAware page on a Haven.
#
# 1. On a computer with internet, build the map for your area:
#      python3 build-offline-map.py <lat> <lon>          (writes ./offline)
# 2. Copy this folder (with the offline/ folder inside) to the Haven, e.g.
#      scp -O -r skyaware-offline root@<haven-ip>:/tmp/
# 3. On the Haven:
#      sh /tmp/skyaware-offline/install.sh <lat> <lon>
#
# Then open http://<haven-ip>:8080. The map starts on "Offline (Natural Earth)";
# the online maps are still in the layers menu for when there is internet.
# A firmware upgrade replaces /usr/share/dump1090, so run this again after one.
set -e

HERE=$(cd "$(dirname "$0")" && pwd)
LAT=$1
LON=$2
[ -n "$LAT" ] && [ -n "$LON" ] || { echo "Usage: sh install.sh <lat> <lon>"; exit 1; }
[ -d /usr/share/dump1090 ] || { echo "No /usr/share/dump1090: install dump1090 first (setup-haven2.sh)."; exit 1; }
[ -f "$HERE/offline/land.geojson" ] || { echo "No offline/ map data next to this script. Run build-offline-map.py first."; exit 1; }

mkdir -p /usr/share/dump1090/geojson/offline
cp "$HERE"/offline/*.geojson /usr/share/dump1090/geojson/offline/
python3 "$HERE/patch-skyaware.py" "$LAT" "$LON"
echo "Offline map installed: $(du -sh /usr/share/dump1090/geojson/offline | cut -f1) of map data."
