#!/bin/sh
#
# Haven Gate (GREEN) - Gateway Node Setup Script
# Run this on a fresh OpenMANET install to configure it as the mesh gateway
#
# Usage: curl -sL <url> | sh
#    or: sh setup-haven-gate.sh
#

set -e

#═══════════════════════════════════════════════════════════════════════════════
# CONFIGURATION - Modify these values as needed
#═══════════════════════════════════════════════════════════════════════════════

# Node identity
HOSTNAME="green"
ROOT_PASSWORD="green"

# Mesh network settings
MESH_ID="haven"
MESH_KEY="havenmesh"
MESH_IP="10.41.0.1"
MESH_NETMASK="255.255.0.0"

# HaLow radio settings (802.11ah)
# Channels vary by region - see docs for your country
HALOW_CHANNEL="28"        # US: 1-51, channel 28 = 916 MHz
HALOW_HTMODE="HT20"       # HT20=2MHz, HT40=4MHz, HT80=8MHz

# WiFi AP settings
WIFI_5GHZ_SSID="green-5ghz"
WIFI_5GHZ_KEY="green-5ghz"
WIFI_5GHZ_CHANNEL="36"

WIFI_24GHZ_SSID="green-2.4ghz"
WIFI_24GHZ_KEY="green-2.4ghz"
WIFI_24GHZ_CHANNEL="6"

# DHCP settings for mesh clients
DHCP_START="100"
DHCP_LIMIT="150"
DHCP_LEASETIME="12h"

#═══════════════════════════════════════════════════════════════════════════════
# SCRIPT START - No modifications needed below
#═══════════════════════════════════════════════════════════════════════════════

echo "═══════════════════════════════════════════════════════════════════"
echo "  Haven Gate (GREEN) Setup Script"
echo "  Gateway Node with Internet Uplink"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

# Check we're on OpenWrt
if [ ! -f /etc/openwrt_release ]; then
    echo "ERROR: This script must be run on OpenWrt/OpenMANET"
    exit 1
fi

echo "[1/8] Setting hostname and password..."
uci set system.@system[0].hostname="$HOSTNAME"
uci commit system
echo "$ROOT_PASSWORD" | passwd root

echo "[2/8] Configuring HaLow mesh radio (802.11ah)..."
# Find the HaLow radio (morse driver)
HALOW_RADIO=$(uci show wireless | grep "morse" | head -1 | cut -d. -f2)
if [ -z "$HALOW_RADIO" ]; then
    echo "WARNING: No HaLow radio found, skipping HaLow configuration"
else
    echo "  Found HaLow radio: $HALOW_RADIO"

    # Configure radio
    uci set wireless.$HALOW_RADIO.disabled='0'
    uci set wireless.$HALOW_RADIO.channel="$HALOW_CHANNEL"
    uci set wireless.$HALOW_RADIO.htmode="$HALOW_HTMODE"

    # Find or create mesh interface
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

echo "[3/8] Configuring 5GHz access point..."
# Find 5GHz radio (usually brcmfmac on Pi)
WIFI5_RADIO=$(uci show wireless | grep -E "brcmfmac|ath10k|mt76" | grep "\.type=" | head -1 | cut -d. -f2)
if [ -z "$WIFI5_RADIO" ]; then
    echo "WARNING: No 5GHz radio found, skipping"
else
    echo "  Found 5GHz radio: $WIFI5_RADIO"

    uci set wireless.$WIFI5_RADIO.disabled='0'
    uci set wireless.$WIFI5_RADIO.channel="$WIFI_5GHZ_CHANNEL"
    uci set wireless.$WIFI5_RADIO.htmode='VHT80'

    # Find or create AP interface
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

echo "[4/8] Configuring bridge and mesh network..."
# Create/configure the bridge interface
uci set network.ahwlan=interface
uci set network.ahwlan.proto='static'
uci set network.ahwlan.ipaddr="$MESH_IP"
uci set network.ahwlan.netmask="$MESH_NETMASK"
uci set network.ahwlan.type='bridge'
uci set network.ahwlan.delegate='0'

# Configure BATMAN-adv
uci set network.bat0=interface
uci set network.bat0.proto='batadv'
uci set network.bat0.routing_algo='BATMAN_IV'
uci set network.bat0.aggregated_ogms='1'
uci set network.bat0.gw_mode='server'
uci set network.bat0.gw_bandwidth='100mbit/100mbit'
uci set network.bat0.orig_interval='1000'

# Add bat0 to bridge
uci set network.ahwlan.device='br-ahwlan'
uci add_list network.ahwlan_dev=device 2>/dev/null || true
uci set network.ahwlan_dev=device
uci set network.ahwlan_dev.name='br-ahwlan'
uci set network.ahwlan_dev.type='bridge'
uci add_list network.ahwlan_dev.ports='bat0'

uci commit network

echo "[5/8] Configuring DHCP server..."
uci set dhcp.ahwlan=dhcp
uci set dhcp.ahwlan.interface='ahwlan'
uci set dhcp.ahwlan.start="$DHCP_START"
uci set dhcp.ahwlan.limit="$DHCP_LIMIT"
uci set dhcp.ahwlan.leasetime="$DHCP_LEASETIME"
uci set dhcp.ahwlan.force='1'
uci commit dhcp

echo "[6/8] Configuring firewall and NAT..."
# Add ahwlan to lan zone for routing
uci add_list firewall.@zone[0].network='ahwlan' 2>/dev/null || true

# Ensure masquerading is enabled on wan
uci set firewall.@zone[1].masq='1'
uci set firewall.@zone[1].mtu_fix='1'

# Allow forwarding from mesh to wan
FORWARD_FOUND=$(uci show firewall | grep "forwarding.*src='lan'.*dest='wan'" || true)
if [ -z "$FORWARD_FOUND" ]; then
    uci add firewall forwarding
    uci set firewall.@forwarding[-1].src='lan'
    uci set firewall.@forwarding[-1].dest='wan'
fi

uci commit firewall

echo "[7/8] Installing Reticulum..."
opkg update
opkg install python3 python3-pip 2>/dev/null || echo "Python may already be installed"

# Install Reticulum
pip3 install rns 2>/dev/null || pip3 install --break-system-packages rns 2>/dev/null || echo "Reticulum may already be installed"

# Create Reticulum config
mkdir -p /root/.reticulum
cat > /root/.reticulum/config << 'RETICULUMCONFIG'
[reticulum]
  share_instance = Yes
  enable_transport = Yes
  instance_control_port = 37428

[interfaces]
  [[HaLow Mesh Bridge]]
    type = AutoInterface
    enabled = Yes
    devices = br-ahwlan
    group_id = reticulum

  [[UDP Broadcast]]
    type = UDPInterface
    enabled = Yes
    listen_ip = 0.0.0.0
    listen_port = 4242
    forward_ip = 10.41.255.255
    forward_port = 4242
RETICULUMCONFIG

# Create Reticulum service
cat > /etc/init.d/rnsd << 'RNSSERVICE'
#!/bin/sh /etc/rc.common
START=99
STOP=10
USE_PROCD=1

start_service() {
    procd_open_instance
    procd_set_param command /usr/bin/rnsd
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
RNSSERVICE
chmod +x /etc/init.d/rnsd

echo "[8/8] Installing ATAK CoT Bridge..."
cat > /root/cot_bridge.py << 'COTBRIDGE'
#!/usr/bin/env python3
"""
CoT Bridge - ATAK multicast to Reticulum
Bridges ATAK's default multicast SA to Reticulum broadcast
"""
import RNS
import socket
import struct
import threading
import time
import zlib
import hashlib

# Configuration
MULTICAST_GROUP = "239.2.3.1"
MULTICAST_PORT = 6969
APP_NAME = "atak_bridge"
ASPECT = "broadcast"
MAX_PAYLOAD = 400

class CoTBridge:
    def __init__(self):
        self.reticulum = RNS.Reticulum()
        self.identity = RNS.Identity()

        # Create broadcast destination
        self.broadcast_dest = RNS.Destination(
            None,
            RNS.Destination.OUT,
            RNS.Destination.PLAIN,
            APP_NAME,
            ASPECT
        )

        # Set up packet callback for incoming
        self.listen_dest = RNS.Destination(
            None,
            RNS.Destination.IN,
            RNS.Destination.PLAIN,
            APP_NAME,
            ASPECT
        )
        self.listen_dest.set_packet_callback(self.packet_received)

        # Multicast socket for sending to ATAK
        self.mcast_send_sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_UDP)
        self.mcast_send_sock.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_TTL, 2)

        # Fragment reassembly buffer
        self.fragments = {}

        print(f"Broadcast destination: {self.broadcast_dest.hash.hex()}")

    def compress(self, data):
        return zlib.compress(data, level=9)

    def decompress(self, data):
        return zlib.decompress(data)

    def fragment(self, data, msg_id):
        """Fragment data into chunks with headers"""
        chunks = []
        total = (len(data) + MAX_PAYLOAD - 1) // MAX_PAYLOAD
        for i in range(total):
            start = i * MAX_PAYLOAD
            end = min(start + MAX_PAYLOAD, len(data))
            # Header: msg_id (4) + seq (1) + total (1) + data
            header = struct.pack(">IBB", msg_id, i, total)
            chunks.append(header + data[start:end])
        return chunks

    def reassemble(self, packet_data):
        """Reassemble fragments, returns complete data or None"""
        if len(packet_data) < 6:
            return packet_data  # Not fragmented

        msg_id, seq, total = struct.unpack(">IBB", packet_data[:6])
        data = packet_data[6:]

        if total == 1:
            return data

        if msg_id not in self.fragments:
            self.fragments[msg_id] = {}

        self.fragments[msg_id][seq] = data

        if len(self.fragments[msg_id]) == total:
            # All fragments received
            result = b''.join(self.fragments[msg_id][i] for i in range(total))
            del self.fragments[msg_id]
            return result

        return None

    def packet_received(self, data, packet):
        """Handle incoming Reticulum packet"""
        try:
            reassembled = self.reassemble(data)
            if reassembled is None:
                return  # Waiting for more fragments

            decompressed = self.decompress(reassembled)

            # Send to multicast
            self.mcast_send_sock.sendto(
                decompressed,
                (MULTICAST_GROUP, MULTICAST_PORT)
            )
            print(f"[Downlink] RX {len(data)} bytes -> multicast")
        except Exception as e:
            print(f"[Downlink] Error: {e}")

    def uplink_thread(self):
        """Listen for ATAK multicast and send to Reticulum"""
        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_UDP)
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        sock.bind(('', MULTICAST_PORT))

        # Join multicast group on all interfaces
        mreq = struct.pack("4sl", socket.inet_aton(MULTICAST_GROUP), socket.INADDR_ANY)
        sock.setsockopt(socket.IPPROTO_IP, socket.IP_ADD_MEMBERSHIP, mreq)

        print(f"Joined multicast {MULTICAST_GROUP}:{MULTICAST_PORT}")
        print("[Uplink] Thread started")

        while True:
            try:
                data, addr = sock.recvfrom(65535)
                print(f"[Uplink] RX {len(data)} bytes from {addr}")

                compressed = self.compress(data)
                msg_id = hash(data) & 0xFFFFFFFF

                fragments = self.fragment(compressed, msg_id)
                for frag in fragments:
                    packet = RNS.Packet(self.broadcast_dest, frag)
                    packet.send()

                print(f"[Uplink] TX {len(compressed)} bytes ({len(fragments)} fragments)")
            except Exception as e:
                print(f"[Uplink] Error: {e}")

    def run(self):
        print("TAK-Reticulum Multicast Bridge starting...")

        uplink = threading.Thread(target=self.uplink_thread, daemon=True)
        uplink.start()

        print("Bridge running!")

        while True:
            time.sleep(1)

if __name__ == "__main__":
    bridge = CoTBridge()
    bridge.run()
COTBRIDGE
chmod +x /root/cot_bridge.py

# Create CoT bridge service
cat > /etc/init.d/cot_bridge << 'COTSERVICE'
#!/bin/sh /etc/rc.common
START=99
STOP=10

start() {
    cd /root
    python3 /root/cot_bridge.py > /tmp/bridge.log 2>&1 &
    echo "CoT Bridge started"
}

stop() {
    killall python3 2>/dev/null || true
    echo "CoT Bridge stopped"
}
COTSERVICE
chmod +x /etc/init.d/cot_bridge

# Commit all wireless changes
uci commit wireless

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  Setup Complete!"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "  Hostname:     $HOSTNAME"
echo "  Mesh IP:      $MESH_IP"
echo "  5GHz SSID:    $WIFI_5GHZ_SSID"
echo "  Mesh ID:      $MESH_ID"
echo ""
echo "  Next steps:"
echo "  1. Reboot the node:  reboot"
echo "  2. After reboot, enable services:"
echo "     /etc/init.d/rnsd enable && /etc/init.d/rnsd start"
echo "     /etc/init.d/cot_bridge enable && /etc/init.d/cot_bridge start"
echo ""
echo "  SSH access after reboot:"
echo "     ssh root@$MESH_IP  (from mesh)"
echo "     ssh root@<wan-ip>  (from upstream network)"
echo ""
echo "═══════════════════════════════════════════════════════════════════"
