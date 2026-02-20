#!/usr/bin/env python3
"""Reticulum verbose sender — prints full packet details"""
import RNS
import sys
import time

if len(sys.argv) < 3:
    print("Usage: python3 rns_send_verbose.py <dest_hash> <message>")
    sys.exit(1)

dest_hash = bytes.fromhex(sys.argv[1])
message = " ".join(sys.argv[2:])

# Enable verbose Reticulum logging
RNS.loglevel = RNS.LOG_VERBOSE

reticulum = RNS.Reticulum()

print("=" * 60)
print("  RETICULUM VERBOSE SENDER")
print("=" * 60)
print(f"  Target hash      : {sys.argv[1]}")
print(f"  Message          : {message}")
print(f"  Message bytes    : {len(message.encode())}")
print(f"  RNS version      : {RNS.__version__}")
print("=" * 60)
print()

# Path resolution
print(f"  [1/4] Resolving path to destination...", flush=True)
t0 = time.time()

if not RNS.Transport.has_path(dest_hash):
    RNS.Transport.request_path(dest_hash)
    print(f"         Path request sent, waiting...", flush=True)
    for i in range(20):
        time.sleep(1)
        if RNS.Transport.has_path(dest_hash):
            break
        print(f"         ...waiting ({i+1}s)", flush=True)

t_path = time.time() - t0

if not RNS.Transport.has_path(dest_hash):
    print(f"  FAILED: Could not resolve path after {t_path:.1f}s")
    sys.exit(1)

print(f"         Path resolved in {t_path:.3f}s")

# Identity recall
print(f"  [2/4] Recalling identity...", flush=True)
identity = RNS.Identity.recall(dest_hash)
if not identity:
    print("  FAILED: Could not recall identity")
    sys.exit(1)

print(f"         Identity   : {identity.hexhash}")
print(f"         Public key : {identity.get_public_key().hex()[:32]}...")
print()

# Link establishment
print(f"  [3/4] Establishing encrypted link...", flush=True)
t0 = time.time()

remote = RNS.Destination(identity, RNS.Destination.OUT, RNS.Destination.SINGLE, "demo", "msg")
print(f"         Dest hash  : {remote.hash.hex()}")

link = RNS.Link(remote)
print(f"         Link hash  : {link.hash.hex()}")

while link.status != RNS.Link.ACTIVE:
    time.sleep(0.1)
    if link.status == RNS.Link.CLOSED:
        print(f"  FAILED: Link closed after {time.time() - t0:.3f}s")
        sys.exit(1)

t_link = time.time() - t0
print(f"         Link ACTIVE in {t_link:.3f}s")
print()

# Send message
print(f"  [4/4] Sending message...", flush=True)
t0 = time.time()

packet = RNS.Packet(link, message.encode())
receipt = packet.send()

t_send = time.time() - t0

print()
print("  SENT PACKET DETAILS")
print("-" * 60)
print(f"  Packet hash      : {packet.packet_hash.hex() if packet.packet_hash else 'N/A'}")
print(f"  Packet type      : {packet.packet_type}")
print(f"  Transport type   : {packet.transport_type}")
print(f"  Header type      : {packet.header_type}")
print(f"  Context          : {packet.context}")
print(f"  Hops             : {packet.hops}")
print(f"  Raw length       : {packet.raw_len if hasattr(packet, 'raw_len') else 'N/A'}")
print(f"  Data length      : {len(packet.data) if packet.data else 0}")
print(f"  Send time        : {t_send * 1000:.1f}ms")
print()
print("  LINK DETAILS")
print("-" * 60)
print(f"  Link hash        : {link.hash.hex()}")
print(f"  Link status      : {link.status}")
print()
print("  TIMING SUMMARY")
print("-" * 60)
print(f"  Path resolution  : {t_path * 1000:.1f}ms")
print(f"  Link setup       : {t_link * 1000:.1f}ms")
print(f"  Packet send      : {t_send * 1000:.1f}ms")
print(f"  Total            : {(t_path + t_link + t_send) * 1000:.1f}ms")
print("=" * 60)
print()

# Wait for delivery
print(f"  Waiting for delivery confirmation...", flush=True)
time.sleep(3)
print(f"  Done.", flush=True)
