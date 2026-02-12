#!/usr/bin/env python3
"""CoT Bridge — ATAK over Reticulum via HaLow mesh"""
import RNS
import socket
import struct
import subprocess
import re
import sys
import os
import time
import zlib
import hashlib
import threading

COT_SA_MULTICAST = "239.2.3.1"
COT_SA_PORT = 6969
COT_CHAT_MULTICAST = "224.10.10.1"
COT_CHAT_PORT = 17012
APP_NAME = "atak"
ASPECT = "cot"
IDENTITY_FILE = "/root/.cot_identity"
MAX_PAYLOAD = 400
DISPLAY_INTERVAL = 2
MAX_LOG_LINES = 12

# ── Hostname ────────────────────────────────────────────────────────
try:
    hostname = socket.gethostname()
except:
    hostname = "unknown"

# ── HaLow radio info ───────────────────────────────────────────────
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
        return info
    except:
        return {}

# ── Reticulum setup ────────────────────────────────────────────────
reticulum = RNS.Reticulum()

if os.path.exists(IDENTITY_FILE):
    identity = RNS.Identity.from_file(IDENTITY_FILE)
else:
    identity = RNS.Identity()
    identity.to_file(IDENTITY_FILE)

destination = RNS.Destination(identity, RNS.Destination.IN, RNS.Destination.SINGLE, APP_NAME, ASPECT)

# ── Multicast sockets ──────────────────────────────────────────────
def make_mcast_socket(mcast_addr, port):
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_UDP)
    s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    s.bind(("", port))
    mreq = struct.pack("4sl", socket.inet_aton(mcast_addr), socket.INADDR_ANY)
    s.setsockopt(socket.IPPROTO_IP, socket.IP_ADD_MEMBERSHIP, mreq)
    s.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_LOOP, 0)
    s.settimeout(0.1)
    return s

sa_socket = make_mcast_socket(COT_SA_MULTICAST, COT_SA_PORT)
chat_socket = make_mcast_socket(COT_CHAT_MULTICAST, COT_CHAT_PORT)

# ── Shared state ───────────────────────────────────────────────────
lock = threading.Lock()
tx_packets = 0
rx_packets = 0
tx_bytes = 0
rx_bytes = 0
link_status = "Waiting for peer..."
active_link = None
outbound_link = None
fragment_buffer = {}
event_log = []
start_time = time.time()

def ts():
    return time.strftime("%H:%M:%S")

def add_event(msg):
    with lock:
        event_log.append(f"  {ts()}  {msg}")
        if len(event_log) > MAX_LOG_LINES:
            event_log.pop(0)

# ── Display thread ─────────────────────────────────────────────────
W = 62  # inner content width (between | and |)

def row(text=""):
    """Pad or truncate text to exactly W chars, wrap in |...|"""
    return "  | " + text[:W].ljust(W) + " |"

def sep(ch="="):
    return "  +" + ch * (W + 2) + "+"

def display_loop():
    sys.stdout.write("\033[?25l\033[2J")
    sys.stdout.flush()
    while True:
        time.sleep(DISPLAY_INTERVAL)
        halow = get_halow_info()
        freq = halow.get('frequency', 'N/A')
        chan = halow.get('channel', '?')
        sig = halow.get('signal', 'N/A')
        rate = halow.get('bitrate', 'N/A')
        enc = halow.get('encryption', 'N/A')
        mesh = halow.get('mesh_id', 'N/A')
        mode = halow.get('mode', 'N/A')
        uptime = int(time.time() - start_time)
        m, s = divmod(uptime, 60)
        h, m = divmod(m, 60)
        up_str = f"{h}h {m}m {s}s" if h else f"{m}m {s}s"

        with lock:
            ltx = tx_packets
            lrx = rx_packets
            ltxb = tx_bytes
            lrxb = rx_bytes
            lstatus = link_status
            logs = list(event_log)

        txkb = f"{ltxb/1024:.1f} KB" if ltxb >= 1024 else f"{ltxb} B"
        rxkb = f"{lrxb/1024:.1f} KB" if lrxb >= 1024 else f"{lrxb} B"

        lines = [
            sep("="),
            row(f"ATAK CoT Bridge -- {hostname}"),
            sep("="),
            row(),
            row(f"Reticulum      RNS {RNS.__version__}"),
            row(f"Node Hash      {destination.hash.hex()}"),
            row(f"Link Status    {lstatus}"),
            row(f"Uptime         {up_str}"),
            row(),
            sep("-"),
            row(),
            row(f"Radio          802.11ah HaLow -- {mode}"),
            row(f"Frequency      {freq}"),
            row(f"Channel        {chan}"),
            row(f"Bit Rate       {rate}"),
            row(f"Signal         {sig}"),
            row(f"Encryption     {enc}"),
            row(f"Mesh ID        {mesh}"),
            row(),
            sep("-"),
            row(),
            row(f"TX (ATAK > Reticulum)   {ltx:<6} pkts   {txkb}"),
            row(f"RX (Reticulum > ATAK)   {lrx:<6} pkts   {rxkb}"),
            row(),
            sep("-"),
        ]
        for line in logs:
            lines.append(row(line.strip()))
        for _ in range(MAX_LOG_LINES - len(logs)):
            lines.append(row())
        lines.append(sep("="))
        lines.append("  Ctrl+C to exit")

        # Cursor home, draw all lines, clear everything below
        buf = "\033[H"
        for line in lines:
            buf += line + "\033[K\n"
        buf += "\033[J"  # clear from cursor to end of screen
        sys.stdout.write(buf)
        sys.stdout.flush()

display_thread = threading.Thread(target=display_loop, daemon=True)
display_thread.start()

# ── Reticulum callbacks ────────────────────────────────────────────
def reassemble(msg_id, seq, total, data):
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

def detect_type(data):
    """Detect if decompressed CoT XML is a chat message or SA beacon"""
    try:
        sample = data[:512] if len(data) > 512 else data
        if b"GeoChat" in sample or b"__chat" in sample:
            return "CHAT"
    except:
        pass
    return "CoT"

def link_packet_callback(message, packet):
    global rx_packets, rx_bytes
    try:
        if message[0:1] == b'F' and len(message) > 6:
            msg_id, seq, total = message[1:5], message[5], message[6]
            data = message[7:]
            add_event(f"◀ frag {seq+1}/{total} ({len(data)}b)")
            full = reassemble(msg_id, seq, total, data)
            if full:
                if full[:2] in (b"\x78\x9c", b"\x78\x01", b"\x78\xda"):
                    full = zlib.decompress(full)
                label = detect_type(full)
                with lock:
                    rx_packets += 1
                    rx_bytes += len(full)
                add_event(f"◀ {label} reassembled {len(full)}b ─▶ ATAK")
                sa_socket.sendto(full, (COT_SA_MULTICAST, COT_SA_PORT))
                chat_socket.sendto(full, (COT_CHAT_MULTICAST, COT_CHAT_PORT))
        else:
            if message[:2] in (b"\x78\x9c", b"\x78\x01", b"\x78\xda"):
                message = zlib.decompress(message)
            label = detect_type(message)
            with lock:
                rx_packets += 1
                rx_bytes += len(message)
            add_event(f"◀ {label} {len(message)}b via Reticulum ─▶ ATAK")
            sa_socket.sendto(message, (COT_SA_MULTICAST, COT_SA_PORT))
            chat_socket.sendto(message, (COT_CHAT_MULTICAST, COT_CHAT_PORT))
    except Exception as e:
        add_event(f"◀ ERR {e}")

def link_established(link):
    global active_link, link_status
    active_link = link
    link.set_packet_callback(link_packet_callback)
    with lock:
        link_status = "Reticulum link active"
    add_event("LINK inbound link established")

destination.set_link_established_callback(link_established)
destination.announce()

if len(sys.argv) > 1:
    remote_hash = bytes.fromhex(sys.argv[1])
    with lock:
        link_status = f"Connecting to {sys.argv[1][:16]}..."
    add_event(f"LINK resolving peer {sys.argv[1][:16]}...")
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
        def on_outbound_ready(link):
            global link_status
            with lock:
                link_status = "Reticulum link active"
            add_event("LINK outbound link ready ─ bridge active")
        outbound_link.set_link_established_callback(on_outbound_ready)
    else:
        with lock:
            link_status = "Could not resolve peer"
        add_event("LINK peer identity not found")

add_event("Bridge started ─ listening for ATAK traffic")

# ── Main loop ──────────────────────────────────────────────────────
def send_cot(data, label):
    global tx_packets, tx_bytes, outbound_link, active_link
    link = outbound_link if (outbound_link and outbound_link.status == RNS.Link.ACTIVE) else active_link
    if not link or link.status != RNS.Link.ACTIVE:
        return

    compressed = zlib.compress(data, 9)
    ratio = int((1 - len(compressed) / len(data)) * 100)
    with lock:
        tx_packets += 1
        tx_bytes += len(data)

    if len(compressed) <= MAX_PAYLOAD:
        RNS.Packet(link, compressed).send()
        add_event(f"▶ {label} {len(data)}b ─▶ zlib {len(compressed)}b (-{ratio}%) ─▶ RNS")
    else:
        msg_id = hashlib.md5(data).digest()[:4]
        frag_size = MAX_PAYLOAD - 7
        chunks = [compressed[i:i+frag_size] for i in range(0, len(compressed), frag_size)]
        total = len(chunks)
        for seq, chunk in enumerate(chunks):
            pkt = b'F' + msg_id + bytes([seq, total]) + chunk
            RNS.Packet(link, pkt).send()
            time.sleep(0.02)
        add_event(f"▶ {label} {len(data)}b ─▶ zlib {len(compressed)}b (-{ratio}%) ─▶ {total} frags ─▶ RNS")

try:
    while True:
        for sock, label in [(sa_socket, "CoT"), (chat_socket, "CHAT")]:
            try:
                data, addr = sock.recvfrom(8192)
                send_cot(data, label)
            except socket.timeout:
                pass
            except Exception as e:
                add_event(f"ERR {e}")

        now = time.time()
        fragment_buffer = {k:v for k,v in fragment_buffer.items() if now - v['time'] < 30}
except KeyboardInterrupt:
    sys.stdout.write("\033[?25h")  # restore cursor
    print("\n  Shutting down...")
