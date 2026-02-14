#!/bin/sh
#
# Haven Gate - Gateway Node Setup Script
# Configures basic mesh networking with internet sharing
#
# Usage: sh setup-haven-gate.sh
#

set -e

#═══════════════════════════════════════════════════════════════════════════════
# CONFIGURATION - Modify these values as needed
#═══════════════════════════════════════════════════════════════════════════════

# Node identity
HOSTNAME="green"
ROOT_PASSWORD="havengreen"

# Mesh network settings
MESH_ID="haven"
MESH_KEY="havenmesh"
MESH_IP="10.41.0.1"
MESH_NETMASK="255.255.0.0"

# HaLow radio settings (802.11ah)
HALOW_CHANNEL="28"        # US: 1-51, channel 28 = 916 MHz
HALOW_HTMODE="HT20"       # HT20=2MHz, HT40=4MHz, HT80=8MHz

# WiFi AP settings
WIFI_5GHZ_SSID="green-5ghz"
WIFI_5GHZ_KEY="green-5ghz"
WIFI_5GHZ_CHANNEL="36"

# DHCP settings for mesh clients
DHCP_START="100"
DHCP_LIMIT="16"
DHCP_LEASETIME="12h"

#═══════════════════════════════════════════════════════════════════════════════
# SCRIPT START
#═══════════════════════════════════════════════════════════════════════════════

echo "═══════════════════════════════════════════════════════════════════"
echo "  Haven Gate Setup - Mesh Gateway Node"
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
    uci set wireless.$HALOW_IFACE.network='ahwlan'
    uci set wireless.$HALOW_IFACE.mesh_fwding='0'
    uci set wireless.$HALOW_IFACE.beacon_int='1000'
fi

echo "[3/6] Configuring 5GHz access point..."
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

echo "[4/6] Configuring bridge and BATMAN-adv..."
uci set network.ahwlan=interface
uci set network.ahwlan.proto='static'
uci set network.ahwlan.ipaddr="$MESH_IP"
uci set network.ahwlan.netmask="$MESH_NETMASK"
uci set network.ahwlan.type='bridge'
uci set network.ahwlan.delegate='0'

uci set network.bat0=interface
uci set network.bat0.proto='batadv'
uci set network.bat0.routing_algo='BATMAN_V'
uci set network.bat0.aggregated_ogms='1'
uci set network.bat0.gw_mode='server'
uci set network.bat0.orig_interval='1000'

uci set network.ahwlan.device='br-ahwlan'
uci set network.ahwlan_dev=device
uci set network.ahwlan_dev.name='br-ahwlan'
uci set network.ahwlan_dev.type='bridge'
uci add_list network.ahwlan_dev.ports='bat0' 2>/dev/null || true
uci commit network

echo "[5/6] Configuring DHCP server..."
uci set dhcp.ahwlan=dhcp
uci set dhcp.ahwlan.interface='ahwlan'
uci set dhcp.ahwlan.ignore='0'
uci set dhcp.ahwlan.start="$DHCP_START"
uci set dhcp.ahwlan.limit="$DHCP_LIMIT"
uci set dhcp.ahwlan.leasetime="$DHCP_LEASETIME"
uci set dhcp.ahwlan.force='1'
uci commit dhcp

echo "[6/6] Configuring firewall and NAT..."
uci add_list firewall.@zone[0].network='ahwlan' 2>/dev/null || true
uci set firewall.@zone[1].masq='1'
uci set firewall.@zone[1].mtu_fix='1'
uci commit firewall

uci commit wireless

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  Haven Gate Setup Complete!"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "  Hostname:     $HOSTNAME"
echo "  Mesh IP:      $MESH_IP"
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
