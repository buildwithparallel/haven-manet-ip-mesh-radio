#!/bin/sh
#
# Haven Point - Mesh Extender Node Setup Script
# Configures basic mesh networking (no internet uplink)
#
# Usage: sh setup-haven-point.sh
#

set -e

#═══════════════════════════════════════════════════════════════════════════════
# CONFIGURATION - Modify these values as needed
#═══════════════════════════════════════════════════════════════════════════════

# Node identity (change for each point node)
HOSTNAME="blue"
ROOT_PASSWORD="havenblue"

# Mesh network settings - MUST MATCH GATE NODE
MESH_ID="haven"
MESH_KEY="havenmesh"
MESH_IP="10.41.0.2"           # Initial IP; openmanetd may reassign after boot
MESH_NETMASK="255.255.0.0"
GATEWAY_IP="10.41.0.1"        # Initial gate IP; openmanetd may reassign after boot
DNS_SERVERS="8.8.8.8 8.8.4.4"

# HaLow radio settings - MUST MATCH GATE NODE
HALOW_CHANNEL="28"
HALOW_HTMODE="HT80"

# Client WiFi AP (2.4GHz only: USB, e.g. Panda RT5370) — use network 'ahwlan' (flat
# 10.41.0.0/16, DHCP on the gate and, with POINT_DHCP, on this point too). This script does not configure onboard 5GHz; HaLow + USB 2.4G
# is the supported combo for point nodes. Set ENABLE_2G4_AP=0 to skip the client AP.
ENABLE_2G4_AP="1"
WIFI_2G4_SSID="blue"
WIFI_2G4_KEY="blueblue"
WIFI_2G4_CHANNEL="1"

# Point DHCP: this point hands out addresses on the mesh too, so a phone on its
# Wi-Fi gets one whether or not the gate is reachable. Each point gets its own
# pool from the last number of MESH_IP: 10.41.0.6 serves 10.41.6.100-199. The
# gate serves 10.41.0.100-115, so the pools never overlap. Phones use this
# point as their gateway and DNS; the point passes internet traffic on to the
# gate when it is up. (Do not hand out the gate as the gateway: with the gate
# gone, Android keeps probing the missing gateway and drops off the Wi-Fi.)
# Set to 0 to leave DHCP to the gate only.
POINT_DHCP="1"

#═══════════════════════════════════════════════════════════════════════════════
# SCRIPT START
#═══════════════════════════════════════════════════════════════════════════════

echo "═══════════════════════════════════════════════════════════════════"
echo "  Haven Point Setup - Mesh Extender Node"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

if [ ! -f /etc/openwrt_release ]; then
    echo "ERROR: This script must be run on OpenWrt/OpenMANET"
    exit 1
fi

echo "[1/6] Setting hostname and password..."
uci set system.@system[0].hostname="$HOSTNAME"
uci commit system
(echo "$ROOT_PASSWORD"; echo "$ROOT_PASSWORD") | passwd root

echo "[2/6] Configuring HaLow mesh radio (802.11ah)..."
HALOW_RADIO=$(uci show wireless | grep "morse" | head -1 | cut -d. -f2)
if [ -z "$HALOW_RADIO" ]; then
    echo "WARNING: No HaLow radio found, skipping"
else
    echo "  Found HaLow radio: $HALOW_RADIO"
    uci set wireless.$HALOW_RADIO.disabled='0'
    uci set wireless.$HALOW_RADIO.channel="$HALOW_CHANNEL"
    uci set wireless.$HALOW_RADIO.htmode="$HALOW_HTMODE"

    HALOW_IFACE=$(uci show wireless | grep "wireless\..*\.device='$HALOW_RADIO'" | head -1 | cut -d. -f2)
    if [ -z "$HALOW_IFACE" ]; then
        HALOW_IFACE="mesh_halow"
        uci set wireless.$HALOW_IFACE=wifi-iface
    fi

    uci set wireless.$HALOW_IFACE.device="$HALOW_RADIO"
    uci set wireless.$HALOW_IFACE.mode='mesh'
    uci set wireless.$HALOW_IFACE.mesh_id="$MESH_ID"
    uci set wireless.$HALOW_IFACE.encryption='sae'
    uci set wireless.$HALOW_IFACE.key="$MESH_KEY"
    uci set wireless.$HALOW_IFACE.network='batmesh'
    uci set wireless.$HALOW_IFACE.mesh_fwding='0'
    uci set wireless.$HALOW_IFACE.beacon_int='1000'

    # batmesh — binds the HaLow wlan to bat0 as a hard interface
    uci set network.batmesh=interface
    uci set network.batmesh.proto='batadv_hardif'
    uci set network.batmesh.master='bat0'
fi

echo "[3/6] Configuring 2.4GHz client access point..."
WIFI2_RADIO=
if [ "$ENABLE_2G4_AP" = 1 ] && [ -n "$WIFI_2G4_SSID" ]; then
    USB_2G_RADIO=""
    ONBOARD_2G_RADIO=""
    for r in $(uci show wireless | grep "=wifi-device" | cut -d. -f2 | cut -d= -f1); do
        band=$(uci get wireless.$r.band 2>/dev/null)
        path=$(uci get wireless.$r.path 2>/dev/null)
        [ "$band" != "2g" ] && continue
        case "$path" in
            *usb*) USB_2G_RADIO="$r" ;;
            *)     ONBOARD_2G_RADIO="$r" ;;
        esac
    done

    if [ -n "$USB_2G_RADIO" ]; then
        WIFI2_RADIO="$USB_2G_RADIO"
        echo "  Detected Panda USB adapter: $WIFI2_RADIO"
        for r in $(uci show wireless | grep "=wifi-device" | cut -d. -f2 | cut -d= -f1); do
            path=$(uci get wireless.$r.path 2>/dev/null)
            type=$(uci get wireless.$r.type 2>/dev/null)
            [ "$type" = "morse" ] && continue
            case "$path" in *usb*) continue ;; esac
            uci set wireless.$r.disabled='1'
            echo "  Disabled onboard radio: $r"
        done
    elif [ -n "$ONBOARD_2G_RADIO" ]; then
        WIFI2_RADIO="$ONBOARD_2G_RADIO"
        echo "  No USB adapter found — using onboard radio: $WIFI2_RADIO"
        echo "  NOTE: onboard radio is less reliable (drops in and out). A Panda USB adapter is recommended."
    else
        echo "  WARNING: No 2.4GHz radio found — skipping client AP."
    fi

    if [ -n "$WIFI2_RADIO" ]; then
        uci set "wireless.$WIFI2_RADIO.disabled"="0"
        uci set "wireless.$WIFI2_RADIO.channel"="$WIFI_2G4_CHANNEL"
        uci set "wireless.$WIFI2_RADIO.htmode"="HT20"
        uci set "wireless.$WIFI2_RADIO.country"="US"

        WIFI2_IFACE=$(uci show wireless | grep "wireless\..*\.device='$WIFI2_RADIO'" | head -1 | cut -d. -f2)
        if [ -z "$WIFI2_IFACE" ]; then
            WIFI2_IFACE="ap_2g4"
            uci set "wireless.$WIFI2_IFACE=wifi-iface"
        elif [ "$(uci -q get "wireless.$WIFI2_IFACE.mode")" = "mesh" ]; then
            WIFI2_IFACE="ap_2g4"
            uci set "wireless.$WIFI2_IFACE=wifi-iface"
        fi
        uci set "wireless.$WIFI2_IFACE.device"="$WIFI2_RADIO"
        uci set "wireless.$WIFI2_IFACE.mode"="ap"
        uci set "wireless.$WIFI2_IFACE.ssid"="$WIFI_2G4_SSID"
        uci set "wireless.$WIFI2_IFACE.encryption"="psk2"
        uci set "wireless.$WIFI2_IFACE.key"="$WIFI_2G4_KEY"
        uci set "wireless.$WIFI2_IFACE.network"="ahwlan"
        echo "  2.4GHz AP configured: SSID=$WIFI_2G4_SSID"
    fi
else
    echo "  (2.4GHz client AP disabled — set ENABLE_2G4_AP=1 and SSID/key to enable)"
fi

echo "[4/6] Configuring bridge and BATMAN-adv..."
uci set network.ahwlan=interface
uci set network.ahwlan.proto='static'
uci set network.ahwlan.ipaddr="$MESH_IP"
uci set network.ahwlan.netmask="$MESH_NETMASK"
uci set network.ahwlan.gateway="$GATEWAY_IP"
uci set network.ahwlan.dns="$DNS_SERVERS"
uci set network.ahwlan.delegate='0'
# NOTE: Do NOT set network.ahwlan.type='bridge' — it auto-creates anonymous
# bridge devices that conflict with our explicit ahwlan_dev definition.

uci set network.bat0=interface
uci set network.bat0.proto='batadv'
uci set network.bat0.routing_algo='BATMAN_V'
uci set network.bat0.aggregated_ogms='1'
uci set network.bat0.gw_mode='client'
uci set network.bat0.orig_interval='1000'

uci set network.ahwlan.device='br-ahwlan'

# Remove anonymous bridge devices for br-ahwlan — these conflict with the
# named ahwlan_dev and cause bat0 to be left out of the bridge.
i=0
while uci get network.@device[$i] >/dev/null 2>&1; do
    dev_name=$(uci get network.@device[$i].name 2>/dev/null)
    if [ "$dev_name" = "br-ahwlan" ]; then
        echo "  Removing conflicting anonymous device[$i] for br-ahwlan"
        uci delete network.@device[$i]
        continue
    fi
    i=$((i + 1))
done

# Named bridge device with bat0 — explicit ports list (not add_list, to avoid dupes)
uci set network.ahwlan_dev=device
uci set network.ahwlan_dev.name='br-ahwlan'
uci set network.ahwlan_dev.type='bridge'
uci delete network.ahwlan_dev.ports 2>/dev/null || true
uci add_list network.ahwlan_dev.ports='bat0'
uci commit network

uci set dhcp.ahwlan=dhcp
uci set dhcp.ahwlan.interface='ahwlan'
if [ "$POINT_DHCP" = 1 ]; then
    POOL="${MESH_IP##*.}"
    echo "[5/6] Point DHCP 10.41.$POOL.100-199 (works with or without the gate) and firewall zone…"
    uci set dhcp.ahwlan.ignore='0'
    # Serve even when the gate's DHCP is also heard, so behaviour does not
    # depend on which node booted first. Both servers give the same gateway.
    uci set dhcp.ahwlan.force='1'
    uci set dhcp.ahwlan.start="$((POOL * 256 + 100))"
    uci set dhcp.ahwlan.limit='100'
    uci set dhcp.ahwlan.leasetime='1h'
    uci set dhcp.ahwlan.dhcpv6='disabled'
    uci set dhcp.ahwlan.ra='disabled'
    # No router or DNS options: dnsmasq then hands out this point's own
    # address for both, which always answers, with or without the gate, and
    # follows the point if its mesh IP changes.
    uci -q delete dhcp.ahwlan.dhcp_option
    # Never NAK a lease the gate (or another point) handed out.
    uci set dhcp.@dnsmasq[0].authoritative='0'
else
    echo "[5/6] Disabling point DHCP (gate hands out 10.41.x.x) and firewall zone…"
    uci set dhcp.ahwlan.ignore='1'
fi
uci commit dhcp
uci add_list firewall.@zone[0].network='ahwlan' 2>/dev/null || true
uci commit firewall

echo "[6/6] Committing wireless..."
uci commit wireless

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  Haven Point Setup Complete!"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "  Hostname:     $HOSTNAME"
echo "  Mesh IP:      $MESH_IP"
echo "  Gateway:      $GATEWAY_IP"
[ "$POINT_DHCP" = 1 ] && echo "  Point DHCP:   10.41.${MESH_IP##*.}.100-199 (phones get an address with or without the gate)"
[ "$ENABLE_2G4_AP" = 1 ] && [ -n "$WIFI_2G4_SSID" ] && echo "  2.4GHz SSID:  $WIFI_2G4_SSID  (USB; br-ahwlan in step 6)"
echo "  Mesh ID:      $MESH_ID"
echo ""
echo "  Optional next steps after reboot:"
echo "    - Install Reticulum:  sh setup-reticulum.sh"
echo "    - Install ATAK bridge: sh setup-cot-bridge.sh"
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "  Rebooting in 5 seconds..."
sleep 5
reboot -f
