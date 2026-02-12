#!/usr/bin/env python3
"""Reticulum receiver — listens and prints incoming messages"""
import RNS, sys, time

reticulum = RNS.Reticulum()
identity = RNS.Identity()

dest = RNS.Destination(identity, RNS.Destination.IN, RNS.Destination.SINGLE, "demo", "msg")
print(f"Listening...")
print(f"Destination hash: {dest.hash.hex()}", flush=True)

def link_established(link):
    print("Link established!", flush=True)
    link.set_packet_callback(lambda msg, pkt: print(f"\n>>> {msg.decode()}", flush=True))

dest.set_link_established_callback(link_established)
dest.announce()

while True:
    time.sleep(0.1)
