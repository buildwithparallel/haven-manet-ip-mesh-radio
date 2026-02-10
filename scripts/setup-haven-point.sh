#!/bin/sh
#
# Haven Point (BLUE) - Mesh Extender Node Setup Script
# Run this on a fresh OpenMANET install to configure it as a mesh extender
#
# Usage: curl -sL <url> | sh
#    or: sh setup-haven-point.sh
#

set -e

#═══════════════════════════════════════════════════════════════════════════════
# CONFIGURATION - Modify these values as needed
#═══════════════════════════════════════════════════════════════════════════════

# Node identity
HOSTNAME="blue"
ROOT_PASSWORD="blue"

# Mesh network settings - MUST MATCH GATE NODE
MESH_ID="haven"
MESH_KEY="havenmesh"
MESH_IP="10.41.0.2"           # Unique IP for this point node
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
# SCRIPT START - No modifications needed below
#═══════════════════════════════════════════════════════════════════════════════

echo "═══════════════════════════════════════════════════════════════════"
echo "  Haven Point (BLUE) Setup Script"
echo "  Mesh Extender Node"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

# Check we're on OpenWrt
if [ ! -f /etc/openwrt_release ]; then
    echo "ERROR: This script must be run on OpenWrt/OpenMANET"
    exit 1
fi

echo "[1/7] Setting hostname and password..."
uci set system.@system[0].hostname="$HOSTNAME"
uci commit system
echo "$ROOT_PASSWORD" | passwd root

echo "[2/7] Configuring HaLow mesh radio (802.11ah)..."
# Find the HaLow radio (morse driver)
HALOW_RADIO=$(uci show wireless | grep "morse" | head -1 | cut -d. -f2)
if [ -z "$HALOW_RADIO" ]; then
    echo "WARNING: No HaLow radio found, skipping HaLow configuration"
else
    echo "  Found HaLow radio: $HALOW_RADIO"

    # Configure radio - MUST MATCH GATE
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

echo "[3/7] Configuring 5GHz access point..."
# Find 5GHz radio
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

echo "[4/7] Configuring bridge and mesh network..."
# Create/configure the bridge interface
uci set network.ahwlan=interface
uci set network.ahwlan.proto='static'
uci set network.ahwlan.ipaddr="$MESH_IP"
uci set network.ahwlan.netmask="$MESH_NETMASK"
uci set network.ahwlan.gateway="$GATEWAY_IP"
uci set network.ahwlan.dns="$DNS_SERVERS"
uci set network.ahwlan.type='bridge'
uci set network.ahwlan.delegate='0'

# Configure BATMAN-adv (client mode - gate is server)
uci set network.bat0=interface
uci set network.bat0.proto='batadv'
uci set network.bat0.routing_algo='BATMAN_IV'
uci set network.bat0.aggregated_ogms='1'
uci set network.bat0.gw_mode='client'
uci set network.bat0.orig_interval='1000'

# Add bat0 to bridge
uci set network.ahwlan.device='br-ahwlan'
uci add_list network.ahwlan_dev=device 2>/dev/null || true
uci set network.ahwlan_dev=device
uci set network.ahwlan_dev.name='br-ahwlan'
uci set network.ahwlan_dev.type='bridge'
uci add_list network.ahwlan_dev.ports='bat0'

uci commit network

echo "[5/7] Disabling DHCP (Gate handles this)..."
# Point nodes should NOT run DHCP
uci set dhcp.ahwlan=dhcp
uci set dhcp.ahwlan.interface='ahwlan'
uci set dhcp.ahwlan.ignore='1'
uci commit dhcp

echo "[6/7] Installing Reticulum..."
opkg update
opkg install python3 python3-pip 2>/dev/null || echo "Python may already be installed"

# Install Reticulum
pip3 install rns 2>/dev/null || pip3 install --break-system-packages rns 2>/dev/null || echo "Reticulum may already be installed"

# Create Reticulum config
mkdir -p /root/.reticulum
cat > /root/.reticulum/config << RETICULUMCONFIG
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

echo "[7/7] Installing ATAK CoT Bridge..."
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
echo "  Gateway:      $GATEWAY_IP"
echo "  5GHz SSID:    $WIFI_5GHZ_SSID"
echo "  Mesh ID:      $MESH_ID"
echo ""
echo "  Next steps:"
echo "  1. Reboot the node:  reboot"
echo "  2. After reboot, enable services:"
echo "     /etc/init.d/rnsd enable && /etc/init.d/rnsd start"
echo "     /etc/init.d/cot_bridge enable && /etc/init.d/cot_bridge start"
echo ""
echo "  SSH access after reboot (via Gate node):"
echo "     ssh -J root@<gate-wan-ip> root@$MESH_IP"
echo "     Or from Gate: ssh root@$MESH_IP"
echo ""
echo "═══════════════════════════════════════════════════════════════════"
