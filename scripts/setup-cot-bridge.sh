#!/bin/sh
#
# ATAK CoT Bridge Setup Script
# Installs the multicast-to-Reticulum bridge for ATAK/CivTAK
#
# Prerequisites:
#   - Run setup-haven-gate.sh or setup-haven-point.sh first
#   - Run setup-reticulum.sh first
#
# Usage: sh setup-cot-bridge.sh
#

set -e

echo "═══════════════════════════════════════════════════════════════════"
echo "  ATAK CoT Bridge Setup"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

if [ ! -f /etc/openwrt_release ]; then
    echo "ERROR: This script must be run on OpenWrt/OpenMANET"
    exit 1
fi

# Check for Reticulum
if ! command -v rnsd >/dev/null 2>&1; then
    echo "ERROR: Reticulum not found. Run setup-reticulum.sh first."
    exit 1
fi

echo "[1/2] Installing CoT Bridge script..."
cat > /root/cot_bridge.py << 'BRIDGE'
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
            header = struct.pack(">IBB", msg_id, i, total)
            chunks.append(header + data[start:end])
        return chunks

    def reassemble(self, packet_data):
        """Reassemble fragments, returns complete data or None"""
        if len(packet_data) < 6:
            return packet_data

        msg_id, seq, total = struct.unpack(">IBB", packet_data[:6])
        data = packet_data[6:]

        if total == 1:
            return data

        if msg_id not in self.fragments:
            self.fragments[msg_id] = {}

        self.fragments[msg_id][seq] = data

        if len(self.fragments[msg_id]) == total:
            result = b''.join(self.fragments[msg_id][i] for i in range(total))
            del self.fragments[msg_id]
            return result

        return None

    def packet_received(self, data, packet):
        """Handle incoming Reticulum packet"""
        try:
            reassembled = self.reassemble(data)
            if reassembled is None:
                return

            decompressed = self.decompress(reassembled)
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
BRIDGE
chmod +x /root/cot_bridge.py

echo "[2/2] Creating CoT Bridge service..."
cat > /etc/init.d/cot_bridge << 'EOF'
#!/bin/sh /etc/rc.common
START=99
STOP=10

start() {
    echo "Starting CoT Bridge..."
    cd /root
    python3 /root/cot_bridge.py > /tmp/bridge.log 2>&1 &
    echo "CoT Bridge started (PID: $!)"
}

stop() {
    echo "Stopping CoT Bridge..."
    pkill -f cot_bridge.py 2>/dev/null || true
    echo "CoT Bridge stopped"
}

restart() {
    stop
    sleep 1
    start
}
EOF
chmod +x /etc/init.d/cot_bridge

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  ATAK CoT Bridge Setup Complete!"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "  Script:  /root/cot_bridge.py"
echo "  Service: /etc/init.d/cot_bridge"
echo "  Logs:    /tmp/bridge.log"
echo ""
echo "  Start the bridge:"
echo "    /etc/init.d/cot_bridge enable"
echo "    /etc/init.d/cot_bridge start"
echo ""
echo "  ATAK Configuration:"
echo "    - Use default settings (SA Multicast enabled)"
echo "    - No custom inputs/outputs needed"
echo "    - Devices auto-discover via 239.2.3.1:6969"
echo ""
echo "  Monitor:"
echo "    tail -f /tmp/bridge.log"
echo ""
echo "═══════════════════════════════════════════════════════════════════"
