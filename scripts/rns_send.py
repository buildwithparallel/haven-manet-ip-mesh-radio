#!/usr/bin/env python3
"""Reticulum sender — connects and sends a message"""
import RNS, sys, time

if len(sys.argv) < 3:
    print("Usage: python3 rns_send.py <dest_hash> <message>")
    sys.exit(1)

dest_hash = bytes.fromhex(sys.argv[1])
message = " ".join(sys.argv[2:])

reticulum = RNS.Reticulum()

print(f"Resolving path...", flush=True)
if not RNS.Transport.has_path(dest_hash):
    RNS.Transport.request_path(dest_hash)
    for _ in range(10):
        time.sleep(1)
        if RNS.Transport.has_path(dest_hash):
            break

identity = RNS.Identity.recall(dest_hash)
if not identity:
    print("Could not resolve identity")
    sys.exit(1)

remote = RNS.Destination(identity, RNS.Destination.OUT, RNS.Destination.SINGLE, "demo", "msg")
link = RNS.Link(remote)

print("Connecting...", flush=True)
while link.status != RNS.Link.ACTIVE:
    time.sleep(0.1)
    if link.status == RNS.Link.CLOSED:
        print("Link failed")
        sys.exit(1)

print(f"Sending: {message}", flush=True)
RNS.Packet(link, message.encode()).send()
print("Sent!", flush=True)
time.sleep(2)
