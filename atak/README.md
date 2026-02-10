# ATAK Integration over Reticulum

This guide explains how to use ATAK (Android Team Awareness Kit) and CivTAK over the Haven mesh network using Reticulum as the transport layer.

**[Haven Guide](https://buildwithparallel.com/products/haven)** - Video tutorials and support for the complete Haven platform.

## Overview

ATAK uses the Cursor-on-Target (CoT) protocol for sharing situational awareness data (positions, markers, messages). By default, ATAK sends CoT via multicast UDP. The Haven bridge captures this traffic and relays it over Reticulum to other mesh nodes.

```
┌──────────┐    multicast     ┌──────────────┐    Reticulum    ┌──────────────┐    multicast     ┌──────────┐
│  ATAK    │ ──────────────►  │  CoT Bridge  │ ──────────────► │  CoT Bridge  │ ──────────────►  │  ATAK    │
│ (Phone)  │ 239.2.3.1:6969   │   (GREEN)    │    HaLow Mesh   │    (BLUE)    │  239.2.3.1:6969  │ (Phone)  │
└──────────┘                  └──────────────┘                 └──────────────┘                  └──────────┘
```

## Requirements

- ATAK-CIV or ATAK-MIL on Android device
- Phone connected to Haven node WiFi (green-5ghz, blue-5ghz, etc.)
- CoT bridge running on each Haven node
- Reticulum daemon (rnsd) running on each node

## Setup

### 1. Install the Bridge Script

Copy the bridge script to each Haven node:

```bash
scp cot_bridge.py root@192.168.0.21:/root/
scp cot_bridge.py root@10.41.73.196:/root/  # via jump host
```

### 2. Start the Bridge

On each node:
```bash
cd /root
python3 cot_bridge.py > /tmp/bridge.log 2>&1 &
```

Or create a service (see below).

### 3. Configure ATAK

**Important**: Use ATAK's default settings. The bridge listens on ATAK's standard multicast address.

1. Open ATAK Settings
2. Go to Network Preferences
3. Ensure SA Multicast is **enabled** (default)
4. Remove any custom outputs/inputs

No special ATAK configuration is needed - it works with defaults.

## How It Works

### ATAK Multicast
ATAK sends Situational Awareness (SA) data via UDP multicast:
- Address: `239.2.3.1`
- Port: `6969`

### Bridge Operation

**Uplink (Phone → Reticulum):**
1. Bridge joins multicast group 239.2.3.1:6969
2. Receives CoT XML from ATAK
3. Compresses with zlib
4. Fragments if > 400 bytes
5. Sends via Reticulum PLAIN broadcast

**Downlink (Reticulum → Phone):**
1. Receives Reticulum packet
2. Reassembles fragments if needed
3. Decompresses
4. Sends to multicast 239.2.3.1:6969
5. ATAK receives and displays

### Compression & Fragmentation

ATAK messages are XML and can be 700-1000+ bytes. Reticulum has a 500-byte MTU.

| Original | Compressed | Fragments |
|----------|------------|-----------|
| 757 bytes | 497 bytes | 2 |
| 951 bytes | 475 bytes | 2 |
| 334 bytes | 315 bytes | 1 |

## Bridge Script

Save as `/root/cot_bridge.py` on each node:

```python
#!/usr/bin/env python3
"""CoT Bridge - ATAK multicast to Reticulum"""
import RNS
import socket
import threading
import time
import zlib
import hashlib

MULTICAST_GROUP = "239.2.3.1"
MULTICAST_PORT = 6969
APP_NAME = "atak_bridge"
ASPECT = "broadcast"
MAX_PAYLOAD = 400

# See cot_bridge.py for full implementation
```

See [cot_bridge.py](cot_bridge.py) for the complete script.

## Running as a Service

Create `/etc/init.d/cot_bridge`:

```bash
#!/bin/sh /etc/rc.common
START=99
STOP=10

start() {
    cd /root
    python3 /root/cot_bridge.py > /tmp/bridge.log 2>&1 &
    echo "CoT Bridge started"
}

stop() {
    killall -9 cot_bridge.py 2>/dev/null
    echo "CoT Bridge stopped"
}
```

Enable:
```bash
chmod +x /etc/init.d/cot_bridge
/etc/init.d/cot_bridge enable
/etc/init.d/cot_bridge start
```

## Monitoring

### View Bridge Logs
```bash
tail -f /tmp/bridge.log
```

Example output:
```
TAK-Reticulum Multicast Bridge starting...
Broadcast destination: d92c596e9f5aa6e030ec2da12b222199
Joined multicast 239.2.3.1:6969
[Uplink] Thread started
Bridge running!
[Uplink] RX 757 bytes from ('10.41.0.105', 6969)
[Uplink] TX 497 bytes to Reticulum
[Downlink] RX 497 bytes -> multicast
```

### Verify Traffic Flow
```bash
# Watch multicast traffic
tcpdump -i br-ahwlan 'udp port 6969' -n

# Check Reticulum
rnstatus
```

## Troubleshooting

### ATAK Devices Don't See Each Other

1. **Check bridges are running:**
   ```bash
   ps | grep cot_bridge
   tail /tmp/bridge.log
   ```

2. **Verify multicast is enabled in ATAK:**
   - Settings → Network Preferences
   - SA should be enabled (default)

3. **Check Reticulum connectivity:**
   ```bash
   rnstatus
   # Should show "Peers: 1 reachable"
   ```

4. **Test multicast locally:**
   ```bash
   tcpdump -i br-ahwlan 'udp port 6969' -n
   # Should see traffic when ATAK is open
   ```

### Messages Not Delivering

1. **Check compression/fragmentation:**
   ```bash
   tail -f /tmp/bridge.log | grep -E "RX|TX|frag"
   ```

2. **Verify bridge on both nodes:**
   - Both must be running
   - Both must have Reticulum connected

### Chat Messages Not Working

ATAK chat may use different mechanisms:
- Peer-to-peer TCP (won't work through bridge)
- Custom outputs (need configuration)

For reliable chat, keep multicast enabled and use the "All Chat Rooms" option.

## Supported Features

| Feature | Status | Notes |
|---------|--------|-------|
| Position sharing (SA) | ✅ Working | Default multicast |
| Markers/Points | ✅ Working | Sent via SA |
| Team member icons | ✅ Working | Sent via SA |
| Chat messages | ✅ Working | Via multicast |
| File transfers | ❌ Not supported | Too large for Reticulum MTU |
| Video streaming | ❌ Not supported | Bandwidth limitations |

## Security Considerations

- **Reticulum provides transport encryption** between nodes
- **ATAK multicast is unencrypted** on the local network
- For full end-to-end encryption, use ATAK's built-in encryption features
- The bridge does not inspect or modify CoT content

## Alternative: Sideband ATAK Plugin

For tighter Reticulum integration, consider the [Sideband-ATAK-plugin](https://github.com/IntelKML/Sideband-ATAK-plugin) which runs directly on the Android device.
