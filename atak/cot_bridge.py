#!/usr/bin/env python3
"""CoT Bridge with fragmentation for large ATAK messages"""
import RNS
import socket
import sys
import os
import time
import zlib
import hashlib

COT_LISTEN_PORT = 4349
COT_FORWARD_PORT = 4349
APP_NAME = "atak"
ASPECT = "cot"
IDENTITY_FILE = "/root/.cot_identity"
MAX_PAYLOAD = 400  # Safe size for link packets

reticulum = RNS.Reticulum()

if os.path.exists(IDENTITY_FILE):
    identity = RNS.Identity.from_file(IDENTITY_FILE)
else:
    identity = RNS.Identity()
    identity.to_file(IDENTITY_FILE)

destination = RNS.Destination(identity, RNS.Destination.IN, RNS.Destination.SINGLE, APP_NAME, ASPECT)
print(f"Destination hash: {destination.hash.hex()}", flush=True)

cot_socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
cot_socket.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
cot_socket.setsockopt(socket.SOL_SOCKET, socket.SO_BROADCAST, 1)
cot_socket.bind(("0.0.0.0", COT_LISTEN_PORT))
cot_socket.settimeout(0.1)

print(f"Listening UDP:{COT_LISTEN_PORT}, forwarding to BROADCAST:{COT_FORWARD_PORT}", flush=True)

active_link = None
outbound_link = None
remote_dest = None
fragment_buffer = {}

def reassemble(msg_id, seq, total, data):
    """Reassemble fragmented messages"""
    global fragment_buffer
    key = msg_id.hex()
    if key not in fragment_buffer:
        fragment_buffer[key] = {'frags': {}, 'total': total, 'time': time.time()}
    fragment_buffer[key]['frags'][seq] = data

    if len(fragment_buffer[key]['frags']) == total:
        full = b''.join(fragment_buffer[key]['frags'][i] for i in range(total))
        del fragment_buffer[key]
        return full
    return None

def link_packet_callback(message, packet):
    try:
        # Check for fragment: F + msg_id(4) + seq(1) + total(1) + data
        if message[0:1] == b'F' and len(message) > 6:
            msg_id, seq, total = message[1:5], message[5], message[6]
            data = message[7:]
            print(f"RX frag {seq+1}/{total}", flush=True)
            full = reassemble(msg_id, seq, total, data)
            if full:
                # Decompress
                if full[:2] in (b"\x78\x9c", b"\x78\x01", b"\x78\xda"):
                    full = zlib.decompress(full)
                print(f"Reassembled: {len(full)} bytes", flush=True)
                cot_socket.sendto(full, ("10.41.255.255", COT_FORWARD_PORT))
        else:
            # Single packet
            if message[:2] in (b"\x78\x9c", b"\x78\x01", b"\x78\xda"):
                message = zlib.decompress(message)
            print(f"RX: {len(message)} bytes", flush=True)
            cot_socket.sendto(message, ("10.41.255.255", COT_FORWARD_PORT))
    except Exception as e:
        print(f"RX Error: {e}", flush=True)

def link_established(link):
    global active_link
    print(f"Inbound link established", flush=True)
    active_link = link
    link.set_packet_callback(link_packet_callback)

destination.set_link_established_callback(link_established)
destination.announce()

if len(sys.argv) > 1:
    remote_hash = bytes.fromhex(sys.argv[1])
    print(f"Connecting to: {sys.argv[1][:16]}...", flush=True)
    if not RNS.Transport.has_path(remote_hash):
        RNS.Transport.request_path(remote_hash)
        for _ in range(10):
            time.sleep(1)
            if RNS.Transport.has_path(remote_hash):
                break
    remote_identity = RNS.Identity.recall(remote_hash)
    if remote_identity:
        remote_dest = RNS.Destination(remote_identity, RNS.Destination.OUT, RNS.Destination.SINGLE, APP_NAME, ASPECT)
        outbound_link = RNS.Link(remote_dest)
        outbound_link.set_packet_callback(link_packet_callback)
        outbound_link.set_link_established_callback(lambda l: print("Outbound link ready!", flush=True))

print("Bridge running...", flush=True)

while True:
    try:
        data, addr = cot_socket.recvfrom(8192)

        link = outbound_link if (outbound_link and outbound_link.status == RNS.Link.ACTIVE) else active_link
        if not link or link.status != RNS.Link.ACTIVE:
            continue

        # Compress
        compressed = zlib.compress(data, 9)
        print(f"UDP RX: {len(data)}b -> {len(compressed)}b compressed", flush=True)

        if len(compressed) <= MAX_PAYLOAD:
            RNS.Packet(link, compressed).send()
            print(f"TX: {len(compressed)} bytes", flush=True)
        else:
            # Fragment
            msg_id = hashlib.md5(data).digest()[:4]
            frag_size = MAX_PAYLOAD - 7
            chunks = [compressed[i:i+frag_size] for i in range(0, len(compressed), frag_size)]
            total = len(chunks)
            print(f"TX: fragmenting into {total} packets", flush=True)
            for seq, chunk in enumerate(chunks):
                pkt = b'F' + msg_id + bytes([seq, total]) + chunk
                RNS.Packet(link, pkt).send()
                time.sleep(0.02)
            print(f"TX: sent {total} fragments", flush=True)

    except socket.timeout:
        pass
    except Exception as e:
        print(f"Error: {e}", flush=True)

    # Clean old fragments
    now = time.time()
    fragment_buffer = {k:v for k,v in fragment_buffer.items() if now - v['time'] < 30}
