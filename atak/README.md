# ATAK Integration over Reticulum

This guide explains how to use ATAK (Android Team Awareness Kit) and CivTAK over the Haven mesh network using Reticulum as the transport layer.

**[Haven Guide](https://buildwithparallel.com/products/haven)** - Video tutorials and support for the complete Haven platform.

## Overview

ATAK uses the Cursor-on-Target (CoT) protocol for sharing situational awareness data (positions, markers, messages). The Haven CoT bridge captures local CoT UDP traffic and relays it over a Reticulum link to the peer bridge on another node.

```
┌──────────┐   UDP 4349   ┌──────────────┐  Reticulum Link  ┌──────────────┐   UDP 4349   ┌──────────┐
│  ATAK    │ ───────────► │  CoT Bridge  │ ◄──────────────► │  CoT Bridge  │ ───────────► │  ATAK    │
│ (Phone)  │  broadcast   │   (GREEN)    │    HaLow Mesh     │    (BLUE)    │  broadcast   │ (Phone)  │
└──────────┘              └──────────────┘                   └──────────────┘              └──────────┘
```

The bridge uses Reticulum **links** (encrypted, reliable connections) rather than broadcast packets. One node acts as the listener and the other initiates the connection using the listener's destination hash.

## Requirements

- ATAK-CIV or ATAK-MIL on Android device (or any app that sends/receives CoT via UDP)
- Phone connected to Haven node WiFi (green-5ghz, blue-5ghz, etc.)
- CoT bridge running on each Haven node
- Reticulum daemon (rnsd) running on each node

## Setup

### 1. Install on the Gate Node (GREEN)

Run the setup script on GREEN first — no peer hash needed:

```bash
sh setup-cot-bridge.sh
```

Enable and start the bridge:

```bash
/etc/init.d/cot_bridge enable
/etc/init.d/cot_bridge start
```

Note GREEN's destination hash:

```bash
head -1 /tmp/bridge.log
# Destination hash: d9bd729dfc56bcacbe4b007238bf0291
```

### 2. Install on Point Nodes (BLUE, etc.)

Run the setup script on each Point node, passing GREEN's destination hash:

```bash
sh setup-cot-bridge.sh d9bd729dfc56bcacbe4b007238bf0291
```

Enable and start:

```bash
/etc/init.d/cot_bridge enable
/etc/init.d/cot_bridge start
```

Check the log to confirm the link is established:

```bash
tail -f /tmp/bridge.log
# Destination hash: a98064462a184ee6a02228629e1390cf
# Connecting to: d9bd729dfc56bcac...
# Outbound link ready!
# Bridge running...
```

### 3. Configure ATAK

ATAK needs to send CoT to the bridge via UDP on port 4349.

1. Open ATAK Settings
2. Go to **Network Preferences** → **Network Connection Preferences**
3. Add a new **TAK Server** or **UDP output** pointed at `127.0.0.1:4349` (if running on the same device), or at the node's mesh IP (e.g., `10.41.0.1:4349`)
4. Alternatively, configure ATAK to broadcast on port 4349

## How It Works

### Peering

The bridge uses Reticulum's link mechanism for reliable, encrypted communication between nodes:

1. Each bridge creates a **SINGLE** destination with a persistent identity (saved to `/root/.cot_identity`)
2. The bridge announces its destination on the Reticulum network
3. If a peer hash is provided (via argument or `/root/.cot_peer`), the bridge establishes an outbound link to that peer
4. The peer's bridge accepts the inbound link
5. CoT data flows bidirectionally over the link

Only one side needs the other's hash. Typically, Point nodes connect to the Gate node.

### Bridge Operation

**Uplink (Local app → Reticulum):**
1. Bridge listens on UDP port 4349
2. Receives CoT data
3. Compresses with zlib (typically 40-60% reduction)
4. Fragments if compressed size > 400 bytes
5. Sends fragments over the Reticulum link

**Downlink (Reticulum → Local app):**
1. Receives packet over the Reticulum link
2. Reassembles fragments if needed
3. Decompresses
4. Sends via UDP broadcast to `10.41.255.255:4349`
5. Local apps receive the CoT data

### Compression & Fragmentation

CoT messages (XML) can be 700-1000+ bytes. Reticulum has a 500-byte MTU.

| Original | Compressed | Fragments |
|----------|------------|-----------|
| 757 bytes | 497 bytes | 2 |
| 951 bytes | 475 bytes | 2 |
| 334 bytes | 315 bytes | 1 |

Fragment header format: `F` + msg_id(4 bytes) + seq(1 byte) + total(1 byte) + data

## Changing the Peer

To point a bridge at a different peer, or to add peering after initial setup:

```bash
# Save the peer's destination hash
echo "d9bd729dfc56bcacbe4b007238bf0291" > /root/.cot_peer

# Restart the bridge to connect
/etc/init.d/cot_bridge restart

# Verify the link
tail -f /tmp/bridge.log
```

To remove peering (run standalone):

```bash
rm /root/.cot_peer
/etc/init.d/cot_bridge restart
```

## Running as a Service

The setup script creates `/etc/init.d/cot_bridge` which automatically reads the peer hash from `/root/.cot_peer` if present.

```bash
# Enable at boot
/etc/init.d/cot_bridge enable

# Start / stop / restart
/etc/init.d/cot_bridge start
/etc/init.d/cot_bridge stop
/etc/init.d/cot_bridge restart
```

## Monitoring

### View Bridge Logs
```bash
tail -f /tmp/bridge.log
```

Example output (Point node connecting to Gate):
```
Destination hash: a98064462a184ee6a02228629e1390cf
Listening UDP:4349, forwarding to BROADCAST:4349
Connecting to: d9bd729dfc56bcac...
Outbound link ready!
Bridge running...
UDP RX: 757b -> 497b compressed
TX: fragmenting into 2 packets
TX: sent 2 fragments
RX frag 1/2
RX frag 2/2
Reassembled: 951 bytes
```

### Verify Traffic Flow
```bash
# Watch CoT traffic on the mesh
tcpdump -i br-ahwlan 'udp port 4349' -n

# Check Reticulum peers
rnstatus
```

## Troubleshooting

### Bridge Says "Bridge running..." But No Traffic

The bridge is waiting for a link. Either:
- No peer hash is configured — set one with `echo "<hash>" > /root/.cot_peer` and restart
- The peer bridge isn't running — start it on the other node
- Reticulum can't find the path — check `rnstatus` on both nodes

### Link Not Establishing

1. **Check Reticulum connectivity:**
   ```bash
   rnstatus
   # Should show active interfaces
   ```

2. **Verify the peer hash is correct:**
   ```bash
   cat /root/.cot_peer
   # Compare with: head -1 /tmp/bridge.log on the other node
   ```

3. **Check mesh connectivity:**
   ```bash
   ping 10.41.0.1   # Can you reach the other node?
   ```

### ATAK Devices Don't See Each Other

1. **Check bridges are running on both nodes:**
   ```bash
   ps | grep cot_bridge
   tail /tmp/bridge.log
   ```

2. **Confirm the link is active:**
   ```bash
   grep -E "link|ready" /tmp/bridge.log
   # Should see "Outbound link ready!" or "Inbound link established"
   ```

3. **Verify ATAK is sending to the right port:**
   ```bash
   tcpdump -i br-ahwlan 'udp port 4349' -n
   # Should see packets when ATAK is open
   ```

### Destination Hash Changed After Reboot

The identity is persisted in `/root/.cot_identity`. The hash should remain stable across reboots unless this file is deleted. If it was deleted, update the peer hash on the other node(s):

```bash
# On the node whose hash changed, get the new hash:
head -1 /tmp/bridge.log

# On the peer node, update:
echo "<new_hash>" > /root/.cot_peer
/etc/init.d/cot_bridge restart
```

## Supported Features

| Feature | Status | Notes |
|---------|--------|-------|
| Position sharing (SA) | Works | Via CoT over link |
| Markers/Points | Works | Via CoT over link |
| Team member icons | Works | Via CoT over link |
| Chat messages | Works | Via CoT over link |
| File transfers | Not supported | Too large for Reticulum MTU |
| Video streaming | Not supported | Bandwidth limitations |

## Security

- **Reticulum links are encrypted** — CoT data is protected in transit between nodes
- **Identity-based** — Each bridge has a persistent cryptographic identity (`/root/.cot_identity`)
- The bridge does not inspect or modify CoT content
- For additional end-to-end encryption, use ATAK's built-in encryption features

## Alternative: Sideband ATAK Plugin

For tighter Reticulum integration, consider the [Sideband-ATAK-plugin](https://github.com/IntelKML/Sideband-ATAK-plugin) which runs directly on the Android device.
