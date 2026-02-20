#!/usr/bin/env python3
"""Reticulum verbose receiver — prints full packet details with hex dump"""
import RNS
import sys
import time

reticulum = RNS.Reticulum()
identity = RNS.Identity()

dest = RNS.Destination(identity, RNS.Destination.IN, RNS.Destination.SINGLE, "demo", "msg")

print("=" * 60)
print("  RETICULUM VERBOSE RECEIVER")
print("=" * 60)
print(f"  Destination hash : {dest.hash.hex()}")
print(f"  Identity hash    : {identity.hexhash}")
print(f"  Public key       : {identity.get_public_key().hex()[:32]}...")
print(f"  RNS version      : {RNS.__version__}")
print("=" * 60)
print(f"  Waiting for incoming links...")
print()

def hex_dump(data, prefix="  "):
    """Print a formatted hex dump with ASCII sidebar"""
    for i in range(0, len(data), 16):
        chunk = data[i:i+16]
        hex_part = " ".join(f"{b:02x}" for b in chunk)
        ascii_part = "".join(chr(b) if 32 <= b < 127 else "." for b in chunk)
        print(f"{prefix}{i:04x}  {hex_part:<48s}  |{ascii_part}|")

def link_established(link):
    print()
    print("-" * 60)
    print("  LINK ESTABLISHED")
    print("-" * 60)
    print(f"  Link hash        : {link.hash.hex()}")
    print(f"  Link status      : {link.status}")
    print(f"  Link type        : {'Incoming' if link.initiator == False else 'Outgoing'}")
    print("-" * 60)
    print()

    def on_packet(message, packet):
        now = time.time()
        msg = message.decode()
        print()
        print("=" * 60)
        print("  MESSAGE RECEIVED")
        print("=" * 60)
        print(f"  Timestamp        : {time.strftime('%H:%M:%S', time.localtime(now))}.{int((now % 1) * 1000):03d}")
        print(f"  Content          : {msg}")
        print(f"  Size (bytes)     : {len(message)}")
        print()
        print("  PACKET DETAILS")
        print("-" * 60)
        print(f"  Packet hash      : {packet.packet_hash.hex() if packet.packet_hash else 'N/A'}")
        print(f"  Packet type      : {packet.packet_type}")
        print(f"  Transport type   : {packet.transport_type}")
        print(f"  Header type      : {packet.header_type}")
        print(f"  Context          : {packet.context}")
        print(f"  Hops             : {packet.hops}")
        if hasattr(packet, 'raw') and packet.raw:
            print(f"  Raw wire length  : {len(packet.raw)} bytes")
        if hasattr(packet, 'ciphertext') and packet.ciphertext:
            print(f"  Ciphertext len   : {len(packet.ciphertext)} bytes")
        print(f"  Plaintext len    : {len(packet.data) if packet.data else 0} bytes")
        if packet.receiving_interface:
            iface = packet.receiving_interface
            print()
            print("  RECEIVING INTERFACE")
            print("-" * 60)
            print(f"  Name             : {iface.name if hasattr(iface, 'name') else iface}")
            print(f"  Type             : {type(iface).__name__}")
            if hasattr(iface, 'bitrate'):
                print(f"  Bitrate          : {iface.bitrate}")
        if hasattr(packet, 'raw') and packet.raw:
            print()
            print("  RAW PACKET (as seen on the wire)")
            print("-" * 60)
            hex_dump(packet.raw)
        if hasattr(packet, 'ciphertext') and packet.ciphertext:
            print()
            print("  CIPHERTEXT (encrypted payload)")
            print("-" * 60)
            hex_dump(packet.ciphertext)
        print()
        print("  PLAINTEXT (decrypted)")
        print("-" * 60)
        hex_dump(message)
        print()
        print("  LINK DETAILS")
        print("-" * 60)
        print(f"  Link hash        : {packet.link.hash.hex() if packet.link else 'N/A'}")
        print(f"  Link status      : {packet.link.status if packet.link else 'N/A'}")
        print("=" * 60)
        print()
        sys.stdout.flush()

    link.set_packet_callback(on_packet)

dest.set_link_established_callback(link_established)

print(f"  Announcing destination...", flush=True)
dest.announce()
print(f"  Announce sent. Listening for connections.", flush=True)
print()

try:
    while True:
        time.sleep(0.1)
except KeyboardInterrupt:
    print("\n  Shutting down.")
