#!/bin/sh
# Meshtastic on a Haven 2 (Raspberry Pi 5, openwrt-morse-rpi5 image) with a
# USB LoRa radio that has no firmware of its own, such as the Null Hop MeshToad.
#
# Run on the Haven with the radio plugged in (antenna first). The Haven needs
# internet through the gate for the downloads:
#   sh setup-meshtoad-haven2.sh
#
# The Haven runs Meshtastic itself (meshtasticd) and drives the radio over USB.
# Phones connect to it over the network: Meshtastic app -> Connections ->
# Network -> + -> the Haven's mesh IP (port 4403).
#
# Why the workarounds: the Pi 5 image is OpenWrt 23.05, which has no
# aarch64_cortex-a76 package feeds, so this adds the aarch64_generic feeds
# (OpenWrt 23.05.5 and Meshtastic). meshtasticd-full lists kmod-bluetooth and
# kmod-spi-dev, which a USB radio doesn't need and which aren't built for this
# kernel, so they are registered as empty placeholder packages.
set -e

RADIO_PROFILE=lora-usb-meshtoad-e22.yaml   # see /etc/meshtasticd/available.d
DATA_DIR=/etc/meshtasticd/data             # /var is wiped on reboot
OWRT=https://downloads.openwrt.org/releases/23.05.5/packages/aarch64_generic
MESH=https://openwrt.meshtastic.org/openwrt-23.05/aarch64_generic

if ! lsusb 2>/dev/null | grep -q -i "1a86:5512"; then
	echo "No CH341 USB LoRa radio (1a86:5512) found. Plug in the MeshToad (antenna first) and run again."
	exit 1
fi
mkdir -p /etc/haven-backup
cp -n /etc/opkg.conf /etc/haven-backup/opkg.conf.before-meshtastic 2>/dev/null || true
cp -n /etc/opkg/customfeeds.conf /etc/haven-backup/customfeeds.conf.before-meshtastic 2>/dev/null || true

# 1. Architectures: list the image's own first-class arch too, or opkg treats
#    every installed package as invalid.
if ! grep -q "^arch aarch64_generic" /etc/opkg.conf; then
	printf "arch all 1\narch noarch 1\narch aarch64_generic 5\narch aarch64_cortex-a76 10\n" >> /etc/opkg.conf
fi

# 2. Feeds: Meshtastic's signing key, then the generic package feeds.
if [ ! -f /etc/opkg/keys/meshtastic-ipk-added ]; then
	curl -fsS -m 60 -o /tmp/meshtastic-ipk.pub https://openwrt.meshtastic.org/meshtastic-ipk.pub
	opkg-key add /tmp/meshtastic-ipk.pub && rm -f /tmp/meshtastic-ipk.pub
	touch /etc/opkg/keys/meshtastic-ipk-added
fi
sed -i "/meshtastic\|openwrt_generic_/d" /etc/opkg/customfeeds.conf
cat >> /etc/opkg/customfeeds.conf <<EOF
src/gz meshtastic $MESH
src/gz openwrt_generic_base $OWRT/base
src/gz openwrt_generic_packages $OWRT/packages
EOF
opkg update >/tmp/opkg-update.log 2>&1 || true
grep -q "Updated list.*meshtastic" /tmp/opkg-update.log || { echo "Could not reach the Meshtastic feed. Is there internet through the gate?"; exit 1; }

# 3. Placeholders for the two kernel modules a USB radio doesn't need.
stub() {
	name=$1
	opkg list-installed "$name" | grep -q . && return 0
	d=/tmp/stub-$name
	rm -rf "$d"; mkdir -p "$d/control" "$d/data"
	cat > "$d/control/control" <<EOF
Package: $name
Version: 0-haven-stub
Architecture: all
Maintainer: Haven
Description: Placeholder so meshtasticd installs on the Haven 2 image. Not needed for a USB LoRa radio.
EOF
	(cd "$d/control" && tar -czf ../control.tar.gz ./control)
	(cd "$d/data" && tar -czf ../data.tar.gz .)
	echo 2.0 > "$d/debian-binary"
	(cd "$d" && tar -czf "/tmp/$name-stub.ipk" ./debian-binary ./control.tar.gz ./data.tar.gz)
	opkg install "/tmp/$name-stub.ipk" 2>&1 | grep -v "no valid architecture" || true
}
stub kmod-bluetooth
stub kmod-spi-dev

# 4. Meshtastic and its libraries.
opkg install meshtasticd-full 2>&1 | grep -v "no valid architecture" | grep -E "Installing|Configuring|rror|annot" || true
command -v meshtasticd >/dev/null || { echo "meshtasticd did not install; see the output above."; exit 1; }

# 5. Radio profile, settings that survive a reboot, start at boot.
cp "/etc/meshtasticd/available.d/$RADIO_PROFILE" /etc/meshtasticd/config.d/
mkdir -p "$DATA_DIR"
[ -f /etc/config/meshtasticd ] || touch /etc/config/meshtasticd
uci -q get meshtasticd.meshtasticd >/dev/null || uci set meshtasticd.meshtasticd=meshtasticd
uci set meshtasticd.meshtasticd.data_dir="$DATA_DIR"
uci commit meshtasticd
grep -q "^/etc/meshtasticd/" /etc/sysupgrade.conf || echo "/etc/meshtasticd/" >> /etc/sysupgrade.conf
/etc/init.d/meshtasticd enable
/etc/init.d/meshtasticd restart
sleep 10

echo "meshtasticd $(meshtasticd --version 2>/dev/null): $(pidof meshtasticd >/dev/null && echo running || echo NOT running)"
logread | grep meshtasticd | grep -E "sx1262 init|sx126x init|API server" | tail -2
echo "Connect the Meshtastic app over the network to this Haven's mesh IP, port 4403."
echo "Then set region, preset, frequency and channel to match your other nodes."
