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
ROOT_PASSWORD="blue"

# Mesh network settings - MUST MATCH GATE NODE
MESH_ID="haven"
MESH_KEY="havenmesh"
MESH_IP="10.41.0.2"           # Unique for each point node
MESH_NETMASK="255.255.0.0"
GATEWAY_IP="10.41.0.1"        # Haven Gate IP
DNS_SERVERS="8.8.8.8 8.8.4.4"

# HaLow radio settings - MUST MATCH GATE NODE
HALOW_CHANNEL="28"
HALOW_HTMODE="HT20"

# WiFi AP settings
WIFI_5GHZ_SSID="blue-5ghz"
WIFI_5GHZ_KEY="blue-5ghz"
WIFI_5GHZ_CHANNEL="36"

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

echo "[1/5] Setting hostname and password..."
uci set system.@system[0].hostname="$HOSTNAME"
uci commit system
echo "$ROOT_PASSWORD" | passwd root

echo "[2/5] Configuring HaLow mesh radio (802.11ah)..."
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
    uci set wireless.$HALOW_IFACE.network='ahwlan'
    uci set wireless.$HALOW_IFACE.mesh_fwding='0'
    uci set wireless.$HALOW_IFACE.beacon_int='1000'
fi

echo "[3/5] Configuring 5GHz access point..."
WIFI5_RADIO=$(uci show wireless | grep -E "brcmfmac|ath10k|mt76" | grep "\.type=" | head -1 | cut -d. -f2)
if [ -z "$WIFI5_RADIO" ]; then
    echo "WARNING: No 5GHz radio found, skipping"
else
    echo "  Found 5GHz radio: $WIFI5_RADIO"
    uci set wireless.$WIFI5_RADIO.disabled='0'
    uci set wireless.$WIFI5_RADIO.channel="$WIFI_5GHZ_CHANNEL"
    uci set wireless.$WIFI5_RADIO.htmode='VHT80'

    WIFI5_IFACE=$(uci show wireless | grep "wireless\..*\.device='$WIFI5_RADIO'" | grep -v mesh | head -1 | cut -d. -f2)
    if [ -z "$WIFI5_IFACE" ]; then
        WIFI5_IFACE="ap_5ghz"
        uci set wireless.$WIFI5_IFACE=wifi-iface
    fi

    uci set wireless.$WIFI5_IFACE.device="$WIFI5_RADIO"
    uci set wireless.$WIFI5_IFACE.mode='ap'
    uci set wireless.$WIFI5_IFACE.ssid="$WIFI_5GHZ_SSID"
    uci set wireless.$WIFI5_IFACE.encryption='psk2'
    uci set wireless.$WIFI5_IFACE.key="$WIFI_5GHZ_KEY"
    uci set wireless.$WIFI5_IFACE.network='ahwlan'
fi

echo "[4/5] Configuring bridge and BATMAN-adv..."
uci set network.ahwlan=interface
uci set network.ahwlan.proto='static'
uci set network.ahwlan.ipaddr="$MESH_IP"
uci set network.ahwlan.netmask="$MESH_NETMASK"
uci set network.ahwlan.gateway="$GATEWAY_IP"
uci set network.ahwlan.dns="$DNS_SERVERS"
uci set network.ahwlan.type='bridge'
uci set network.ahwlan.delegate='0'

uci set network.bat0=interface
uci set network.bat0.proto='batadv'
uci set network.bat0.routing_algo='BATMAN_IV'
uci set network.bat0.aggregated_ogms='1'
uci set network.bat0.gw_mode='client'
uci set network.bat0.orig_interval='1000'

uci set network.ahwlan.device='br-ahwlan'
uci set network.ahwlan_dev=device
uci set network.ahwlan_dev.name='br-ahwlan'
uci set network.ahwlan_dev.type='bridge'
uci add_list network.ahwlan_dev.ports='bat0' 2>/dev/null || true
uci commit network

echo "[5/5] Disabling DHCP (Gate handles this)..."
uci set dhcp.ahwlan=dhcp
uci set dhcp.ahwlan.interface='ahwlan'
uci set dhcp.ahwlan.ignore='1'
uci commit dhcp

uci commit wireless

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  Haven Point Setup Complete!"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "  Hostname:     $HOSTNAME"
echo "  Mesh IP:      $MESH_IP"
echo "  Gateway:      $GATEWAY_IP"
echo "  5GHz SSID:    $WIFI_5GHZ_SSID"
echo "  Mesh ID:      $MESH_ID"
echo ""
echo "  Reboot now:   reboot"
echo ""
echo "  Optional next steps after reboot:"
echo "    - Install Reticulum:  sh setup-reticulum.sh"
echo "    - Install ATAK bridge: sh setup-cot-bridge.sh"
echo ""
echo "═══════════════════════════════════════════════════════════════════"
