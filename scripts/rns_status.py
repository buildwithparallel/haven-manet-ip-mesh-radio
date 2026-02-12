#!/usr/bin/env python3
"""Reticulum network status with live refresh and data exchange"""
import RNS
import subprocess
import re
import sys
import os
import time
import threading
import hashlib
import zlib

# ── Config ──────────────────────────────────────────────────────────
PING_INTERVAL = 3
APP_NAME = "haven"
ASPECT = "status"
IDENTITY_FILE = "/root/.rns_status_identity"

# ── HaLow info ─────────────────────────────────────────────────────
def get_halow_info():
    try:
        out = subprocess.check_output(["iwinfo", "wlan0", "info"], stderr=subprocess.DEVNULL).decode()
        info = {}
        for line in out.split("\n"):
            line = line.strip()
            if "Channel:" in line:
                m = re.search(r'Channel:\s*(\d+)\s*\(([^)]+)\)', line)
                if m:
                    info["channel"] = m.group(1)
                    info["frequency"] = m.group(2)
            if "Bit Rate:" in line:
                m = re.search(r'Bit Rate:\s*(.+?)$', line)
                if m: info["bitrate"] = m.group(1).strip()
            if "Signal:" in line:
                m = re.search(r'Signal:\s*(\S+ \S+)', line)
                if m: info["signal"] = m.group(1)
            if "Encryption:" in line:
                m = re.search(r'Encryption:\s*(.+?)$', line)
                if m: info["encryption"] = m.group(1).strip()
            if "Mode:" in line and "Mesh" in line:
                info["mode"] = "Mesh Point"
            if "ESSID:" in line:
                m = re.search(r'ESSID:\s*"([^"]+)"', line)
                if m: info["mesh_id"] = m.group(1)
            if "Hardware:" in line:
                m = re.search(r'Hardware:\s*(.+?)$', line)
                if m: info["hardware"] = m.group(1).strip()
        return info
    except:
        return {}

# ── Hostname ────────────────────────────────────────────────────────
try:
    import socket as _sock
    hostname = _sock.gethostname()
except:
    hostname = "unknown"

# ── Reticulum setup ────────────────────────────────────────────────
reticulum = RNS.Reticulum()

if os.path.exists(IDENTITY_FILE):
    identity = RNS.Identity.from_file(IDENTITY_FILE)
else:
    identity = RNS.Identity()
    identity.to_file(IDENTITY_FILE)

dest = RNS.Destination(identity, RNS.Destination.IN, RNS.Destination.SINGLE, APP_NAME, ASPECT)
dest.announce()

# ── Data exchange state ─────────────────────────────────────────────
peers = {}        # hash_hex -> {name, last_seen, rtt_ms, packets_rx, link}
packets_tx = 0
packets_rx = 0
link_status = "Waiting for peers..."

def link_established(link):
    global link_status
    link_status = "Link active"
    link.set_packet_callback(on_packet)

    # Send our hostname so the peer knows who we are
    link.identify(identity)
    RNS.Packet(link, f"HELLO:{hostname}".encode()).send()

def on_packet(message, packet):
    global packets_rx, link_status
    packets_rx += 1
    try:
        msg = message.decode()
        if msg.startswith("HELLO:"):
            peer_name = msg[6:]
            link_status = f"Linked with {peer_name}"
        elif msg.startswith("PING:"):
            # Reply with PONG
            parts = msg.split(":")
            if packet.link and packet.link.status == RNS.Link.ACTIVE:
                RNS.Packet(packet.link, f"PONG:{parts[1]}:{hostname}".encode()).send()
        elif msg.startswith("PONG:"):
            parts = msg.split(":")
            sent_time = float(parts[1])
            peer_name = parts[2] if len(parts) > 2 else "unknown"
            rtt = (time.time() - sent_time) * 1000
            peers[peer_name] = {
                "last_seen": time.time(),
                "rtt_ms": rtt,
            }
    except:
        pass

dest.set_link_established_callback(link_established)

# ── Outbound link (if peer hash provided) ──────────────────────────
outbound_link = None
if len(sys.argv) > 1:
    peer_hash = bytes.fromhex(sys.argv[1])
    link_status = "Connecting to peer..."

    def connect_to_peer():
        global outbound_link, link_status
        if not RNS.Transport.has_path(peer_hash):
            RNS.Transport.request_path(peer_hash)
            for _ in range(15):
                time.sleep(1)
                if RNS.Transport.has_path(peer_hash):
                    break

        peer_identity = RNS.Identity.recall(peer_hash)
        if peer_identity:
            peer_dest = RNS.Destination(peer_identity, RNS.Destination.OUT, RNS.Destination.SINGLE, APP_NAME, ASPECT)
            outbound_link = RNS.Link(peer_dest)
            outbound_link.set_packet_callback(on_packet)
            outbound_link.set_link_established_callback(lambda l: on_outbound_ready(l))
        else:
            link_status = "Could not find peer"

    def on_outbound_ready(link):
        global link_status
        link_status = "Link active"
        RNS.Packet(link, f"HELLO:{hostname}".encode()).send()

    t = threading.Thread(target=connect_to_peer, daemon=True)
    t.start()

# ── Ping loop ──────────────────────────────────────────────────────
def ping_loop():
    global packets_tx
    while True:
        time.sleep(PING_INTERVAL)
        link = outbound_link
        if link and link.status == RNS.Link.ACTIVE:
            try:
                RNS.Packet(link, f"PING:{time.time()}:{hostname}".encode()).send()
                packets_tx += 1
            except:
                pass

ping_thread = threading.Thread(target=ping_loop, daemon=True)
ping_thread.start()

# ── Read config interfaces ─────────────────────────────────────────
config_interfaces = []
try:
    with open("/root/.reticulum/config") as f:
        current = None
        for line in f:
            line = line.rstrip()
            if line.strip().startswith("[[") and line.strip().endswith("]]"):
                current = {"name": line.strip()[2:-2]}
                config_interfaces.append(current)
            elif current and "=" in line and not line.strip().startswith("#"):
                key, val = line.strip().split("=", 1)
                current[key.strip()] = val.strip()
except:
    pass

# ── Display loop ───────────────────────────────────────────────────
try:
    while True:
        halow = get_halow_info()

        # Clear screen
        print("\033[2J\033[H", end="", flush=True)

        print()
        print(f"  Reticulum Network Status — {hostname}")
        print(f"  {'='*54}")
        print(f"  Version         : RNS {RNS.__version__}")
        print(f"  Node hash       : {dest.hash.hex()}")
        print(f"  Status          : {link_status}")
        print()
        print(f"  Radio Transport Layer")
        print(f"  {'-'*54}")
        print(f"    Hardware      : {halow.get('hardware', 'N/A')}")
        print(f"    Mode          : {halow.get('mode', 'N/A')}")
        print(f"    Mesh ID       : {halow.get('mesh_id', 'N/A')}")
        print(f"    Frequency     : {halow.get('frequency', 'N/A')}")
        print(f"    Channel       : {halow.get('channel', 'N/A')}")
        print(f"    Bit Rate      : {halow.get('bitrate', 'N/A')}")
        print(f"    Signal        : {halow.get('signal', 'N/A')}")
        print(f"    Encryption    : {halow.get('encryption', 'N/A')}")
        print()
        print(f"  Reticulum Interfaces")
        print(f"  {'-'*54}")
        for iface in config_interfaces:
            itype = iface.get("type", "Unknown")
            print(f"    [{iface['name']}]")
            print(f"      Type        : {itype}")
            if "devices" in iface:
                print(f"      Device      : {iface['devices']}")
            if "listen_ip" in iface and itype != "TCPServerInterface":
                print(f"      Listen      : {iface['listen_ip']}:{iface.get('listen_port','')}")
            if "forward_ip" in iface:
                print(f"      Forward     : {iface['forward_ip']}:{iface.get('forward_port','')}")
            if itype == "TCPServerInterface":
                print(f"      Listen      : {iface.get('listen_ip','')}:{iface.get('listen_port','')}")
        print()
        print(f"  Data Exchange")
        print(f"  {'-'*54}")
        print(f"    Packets TX    : {packets_tx}")
        print(f"    Packets RX    : {packets_rx}")

        if peers:
            for name, info in peers.items():
                age = time.time() - info["last_seen"]
                status = "alive" if age < 10 else "stale"
                print(f"    Peer [{name}]  : RTT {info['rtt_ms']:.1f}ms  ({status})")
        else:
            print(f"    Peers         : Discovering...")

        print()
        print(f"  {'─'*54}")
        print(f"  Refreshing every {PING_INTERVAL}s — Ctrl+C to exit")
        print()

        time.sleep(PING_INTERVAL)

except KeyboardInterrupt:
    print("\n  Shutting down...")
