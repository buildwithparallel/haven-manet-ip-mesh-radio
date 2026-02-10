#!/usr/bin/env python3
"""
CoT Bridge - ATAK Multicast to Reticulum

This bridge captures ATAK's default multicast CoT (Cursor-on-Target) traffic
and relays it over Reticulum to other Haven mesh nodes.

Usage:
    python3 cot_bridge.py

The bridge automatically:
- Joins ATAK's multicast group (239.2.3.1:6969)
- Compresses CoT XML with zlib
- Fragments large messages for Reticulum's 500-byte MTU
- Broadcasts to all Reticulum peers

Deploy this script on each Haven node that should bridge ATAK traffic.
"""

import RNS
import socket
import threading
import time
import zlib
import hashlib

# ATAK default multicast settings
MULTICAST_GROUP = "239.2.3.1"
MULTICAST_PORT = 6969

# Reticulum application identifiers
APP_NAME = "atak_bridge"
ASPECT = "broadcast"

# Maximum payload per Reticulum packet (MTU is 500, leave room for headers)
MAX_PAYLOAD = 400

print("TAK-Reticulum Multicast Bridge starting...", flush=True)

# Initialize Reticulum
reticulum = RNS.Reticulum()

# Create a PLAIN destination for broadcast (no encryption identity needed)
# All bridges use the same app name/aspect so they can hear each other
broadcast_destination = RNS.Destination(
    None,
    RNS.Destination.IN,
    RNS.Destination.PLAIN,
    APP_NAME, ASPECT
)

print(f"Broadcast destination: {broadcast_destination.hash.hex()}", flush=True)

# Set up multicast UDP socket
sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_UDP)
sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)

# Allow multiple processes to bind (useful for testing)
try:
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEPORT, 1)
except AttributeError:
    pass  # SO_REUSEPORT not available on all platforms

# Configure multicast
sock.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_TTL, 2)
sock.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_LOOP, 1)

# Bind to multicast port
sock.bind(("0.0.0.0", MULTICAST_PORT))

# Join multicast group on all interfaces
mreq = socket.inet_aton(MULTICAST_GROUP) + socket.inet_aton("0.0.0.0")
sock.setsockopt(socket.IPPROTO_IP, socket.IP_ADD_MEMBERSHIP, mreq)

print(f"Joined multicast {MULTICAST_GROUP}:{MULTICAST_PORT}", flush=True)

# Fragment reassembly buffer
# Key: message ID (hex), Value: {frags: {seq: data}, total: int, time: float}
fragment_buffer = {}


def reassemble(msg_id, seq, total, data):
    """
    Reassemble fragmented messages.

    Returns complete message when all fragments received, None otherwise.
    """
    global fragment_buffer
    key = msg_id.hex()

    if key not in fragment_buffer:
        fragment_buffer[key] = {
            'frags': {},
            'total': total,
            'time': time.time()
        }

    fragment_buffer[key]['frags'][seq] = data

    # Check if all fragments received
    if len(fragment_buffer[key]['frags']) == total:
        # Reassemble in order
        full = b''.join(fragment_buffer[key]['frags'][i] for i in range(total))
        del fragment_buffer[key]
        return full

    return None


def downlink_callback(data, packet):
    """
    Handle incoming Reticulum packets.

    Receives data from other bridges and sends to local multicast.
    """
    try:
        # Check for fragmented message
        # Fragment header: 'F' + msg_id(4) + seq(1) + total(1) + data
        if data[0:1] == b'F' and len(data) > 6:
            msg_id = data[1:5]
            seq = data[5]
            total = data[6]
            frag_data = data[7:]

            print(f"[Downlink] RX frag {seq+1}/{total}", flush=True)

            full = reassemble(msg_id, seq, total, frag_data)
            if full:
                # Decompress if zlib compressed
                if full[:2] in (b"\x78\x9c", b"\x78\x01", b"\x78\xda"):
                    full = zlib.decompress(full)
                print(f"[Downlink] Reassembled {len(full)} bytes -> multicast", flush=True)
                sock.sendto(full, (MULTICAST_GROUP, MULTICAST_PORT))
        else:
            # Single packet (not fragmented)
            if data[:2] in (b"\x78\x9c", b"\x78\x01", b"\x78\xda"):
                data = zlib.decompress(data)
            print(f"[Downlink] RX {len(data)} bytes -> multicast", flush=True)
            sock.sendto(data, (MULTICAST_GROUP, MULTICAST_PORT))

    except Exception as e:
        print(f"[Downlink] Error: {e}", flush=True)


# Register callback for incoming Reticulum packets
broadcast_destination.set_packet_callback(downlink_callback)


def uplink_thread():
    """
    Handle outgoing traffic.

    Receives multicast from ATAK and sends to Reticulum.
    """
    print("[Uplink] Thread started", flush=True)

    while True:
        try:
            # Receive from multicast
            data, addr = sock.recvfrom(4096)
            print(f"[Uplink] RX {len(data)} bytes from {addr}", flush=True)

            # Compress the CoT XML
            compressed = zlib.compress(data, 9)

            if len(compressed) <= MAX_PAYLOAD:
                # Fits in single packet
                packet = RNS.Packet(broadcast_destination, compressed)
                packet.send()
                print(f"[Uplink] TX {len(compressed)} bytes to Reticulum", flush=True)
            else:
                # Need to fragment
                msg_id = hashlib.md5(data).digest()[:4]  # 4-byte message ID
                frag_size = MAX_PAYLOAD - 7  # Account for fragment header

                chunks = [compressed[i:i+frag_size]
                         for i in range(0, len(compressed), frag_size)]

                for seq, chunk in enumerate(chunks):
                    # Fragment header: 'F' + msg_id + seq + total + data
                    pkt_data = b'F' + msg_id + bytes([seq, len(chunks)]) + chunk
                    packet = RNS.Packet(broadcast_destination, pkt_data)
                    packet.send()
                    time.sleep(0.02)  # Small delay between fragments

                print(f"[Uplink] TX {len(chunks)} fragments to Reticulum", flush=True)

        except Exception as e:
            print(f"[Uplink] Error: {e}", flush=True)

        # Clean up old incomplete fragments (older than 30 seconds)
        now = time.time()
        for key in list(fragment_buffer.keys()):
            if now - fragment_buffer[key]['time'] > 30:
                del fragment_buffer[key]


# Start uplink thread
uplink = threading.Thread(target=uplink_thread, daemon=True)
uplink.start()

print("Bridge running!", flush=True)

# Keep main thread alive
try:
    while True:
        time.sleep(1)
except KeyboardInterrupt:
    print("\nShutting down...", flush=True)
