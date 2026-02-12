# ATAK Integration over Reticulum

Use ATAK (Android Team Awareness Kit) and CivTAK over the Haven mesh network with Reticulum as the encrypted transport layer. **No special ATAK configuration is required** — the bridge transparently intercepts standard ATAK multicast traffic and relays it over Reticulum.

**[Haven Guide](https://buildwithparallel.com/products/haven)** - Video tutorials and support for the complete Haven platform.

## How It Works

ATAK devices send position reports (SA) and chat messages over standard UDP multicast. The CoT bridge joins these multicast groups, intercepts the traffic, compresses it, and sends it over an encrypted Reticulum link to the bridge on the remote node. The remote bridge decompresses and re-publishes the data to local multicast, where ATAK devices pick it up normally.

```
  Node A (GREEN)                                              Node B (BLUE)

  +-----------+                                               +-----------+
  |   ATAK    |  multicast                       multicast    |   ATAK    |
  |  (Phone)  |  239.2.3.1:6969 (SA)             ◀────────── |  (Phone)  |
  |           |  224.10.10.1:17012 (Chat)                     |           |
  +-----+-----+                                               +-----+-----+
        |                                                           |
        ▼                                                           ▼
  +-----------+     Reticulum Link (encrypted)              +-----------+
  |    CoT    |     over 802.11ah HaLow mesh                |    CoT    |
  |  Bridge   | ◀─────────────────────────────────────────► |  Bridge   |
  |  (GREEN)  |     zlib compressed + fragmented            |  (BLUE)   |
  +-----------+                                              +-----------+
```

**Key point**: ATAK devices just connect to the Haven node's WiFi and use their default multicast settings. The bridge handles everything else.

### Multicast Groups

The bridge listens on both standard ATAK multicast groups:

| Traffic Type | Multicast Group | Port | Typical Size |
|-------------|-----------------|------|-------------|
| SA (position beacons) | `239.2.3.1` | 6969 | 300-340 bytes |
| Chat messages | `224.10.10.1` | 17012 | 700-800 bytes |

### Bridge Pipeline

**Outbound (ATAK ▶ Reticulum):**
1. ATAK sends CoT XML to multicast (standard behavior)
2. Bridge intercepts the multicast packet
3. Compresses with zlib (typically 1-38% reduction)
4. If compressed size > 400 bytes, fragments into multiple packets
5. Sends over the encrypted Reticulum link

**Inbound (Reticulum ▶ ATAK):**
1. Receives encrypted packet from Reticulum link
2. Reassembles fragments if needed
3. Decompresses
4. Detects message type (CoT or CHAT)
5. Re-publishes to both multicast groups
6. Local ATAK devices receive the data

### Compression & Fragmentation

CoT XML messages can exceed Reticulum's 500-byte MTU. The bridge compresses with zlib and fragments if needed:

| Message Type | Original | Compressed | Fragments |
|-------------|----------|------------|-----------|
| SA beacon | 334 bytes | 315 bytes | 1 |
| SA beacon | 298 bytes | 290 bytes | 1 |
| Chat message | 759 bytes | 469 bytes | 2 |

Fragment header format: `F` + msg_id(4 bytes) + seq(1 byte) + total(1 byte) + data

## Requirements

- ATAK-CIV or ATAK-MIL on Android device (or any app that sends/receives CoT via multicast)
- Phone connected to Haven node WiFi (green-5ghz, blue-5ghz, etc.)
- CoT bridge running on each Haven node
- Reticulum daemon (rnsd) running on each node

## Setup

### 1. Install on the Gate Node (GREEN)

Run the setup script on GREEN first — no peer hash needed:

```bash
sh setup-cot-bridge.sh
```

Enable and start the services:

```bash
/etc/init.d/cot_bridge enable
/etc/init.d/cot_bridge start
```

Note GREEN's destination hash from the dashboard or log:

```bash
grep "Node Hash" /tmp/bridge.log
# Or run interactively to see the dashboard:
python3 /root/cot_bridge.py
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

### 3. Configure ATAK

**No special ATAK configuration is needed.** ATAK's default multicast output works out of the box:

1. Connect your phone to the Haven node's WiFi (e.g., `green-5ghz` or `blue-5ghz`)
2. Open ATAK — it will automatically send SA beacons and chat to multicast
3. The bridge intercepts and relays everything transparently

If ATAK is configured to use a custom TAK Server or unicast output instead of multicast, switch it back to the default multicast configuration in **Settings > Network Preferences > Network Connection Preferences**.

## Live Dashboard

The bridge includes a live-refreshing terminal dashboard showing real-time status. Run it interactively (not as a background service) to see the dashboard:

```bash
# Gate node (listener)
python3 /root/cot_bridge.py

# Point node (connects to gate)
python3 /root/cot_bridge.py <gate_destination_hash>
```

The dashboard displays:

```
  +================================================================+
  | ATAK CoT Bridge -- green                                       |
  +================================================================+
  |                                                                |
  | Reticulum      RNS 1.1.3                                       |
  | Node Hash      d9bd729dfc56bcacbe4b007238bf0291                |
  | Link Status    Reticulum link active                           |
  | Uptime         2m 26s                                          |
  |                                                                |
  +----------------------------------------------------------------+
  |                                                                |
  | Radio          802.11ah HaLow -- Mesh Point                    |
  | Frequency      916.000 MHz                                     |
  | Channel        28                                              |
  | Bit Rate       32.5 MBit/s                                     |
  | Signal         -1 dBm                                          |
  | Encryption     WPA3 SAE (CCMP)                                 |
  | Mesh ID        haven                                           |
  |                                                                |
  +----------------------------------------------------------------+
  |                                                                |
  | TX (ATAK > Reticulum)   18     pkts   5.8 KB                   |
  | RX (Reticulum > ATAK)   10     pkts   3.5 KB                   |
  |                                                                |
  +----------------------------------------------------------------+
  | 01:46:30  ◀ CoT 298b via Reticulum ─▶ ATAK                     |
  | 01:46:32  ◀ frag 1/2 (393b)                                    |
  | 01:46:32  ◀ frag 2/2 (80b)                                     |
  | 01:46:32  ◀ CHAT reassembled 759b ─▶ ATAK                      |
  | 01:46:48  ◀ CoT 307b via Reticulum ─▶ ATAK                     |
  | 01:47:01  ▶ CoT 334b ─▶ zlib 315b (-5%) ─▶ RNS                 |
  | 01:47:03  ▶ CHAT 759b ─▶ zlib 469b (-38%) ─▶ 2 frags ─▶ RNS    |
  +================================================================+
  Ctrl+C to exit
```

The event log labels each message as **CoT** (position beacons) or **CHAT** (chat messages) and shows the compression ratio and fragmentation details.

## Running as a Service

The setup script creates an init.d service at `/etc/init.d/cot_bridge` that reads the peer hash from `/root/.cot_peer`:

```bash
# Enable at boot
/etc/init.d/cot_bridge enable

# Start / stop / restart
/etc/init.d/cot_bridge start
/etc/init.d/cot_bridge stop
/etc/init.d/cot_bridge restart

# View logs (the dashboard output)
tail -f /tmp/bridge.log
```

When running as a service the output goes to `/tmp/bridge.log`. For the interactive dashboard (e.g., for a demo or video), stop the service and run manually:

```bash
/etc/init.d/cot_bridge stop
python3 /root/cot_bridge.py                    # Gate node
python3 /root/cot_bridge.py <peer_hash>        # Point node
```

## Peering

The bridge uses Reticulum **links** (encrypted, reliable connections) for node-to-node communication:

1. Each bridge creates a destination with a persistent cryptographic identity (saved to `/root/.cot_identity`)
2. The bridge announces its destination on the Reticulum network
3. If a peer hash is provided (via argument or `/root/.cot_peer`), the bridge establishes an outbound link
4. The peer bridge accepts the inbound link
5. CoT data flows bidirectionally over the encrypted link

Only one side needs the other's hash. Typically, Point nodes connect to the Gate node.

### Changing the Peer

```bash
# Save the peer's destination hash
echo "d9bd729dfc56bcacbe4b007238bf0291" > /root/.cot_peer

# Restart the bridge to connect
/etc/init.d/cot_bridge restart
```

To remove peering (run standalone):

```bash
rm /root/.cot_peer
/etc/init.d/cot_bridge restart
```

## Troubleshooting

### No Traffic in Dashboard

- **ATAK not connected to node WiFi** — phone must be on the Haven node's WiFi (e.g., `green-5ghz`)
- **ATAK using unicast/TAK Server** — switch ATAK back to default multicast output
- **Bridge not joined to multicast** — restart the bridge

### Link Not Establishing

1. Check Reticulum is running: `rnstatus`
2. Verify the peer hash: `cat /root/.cot_peer`
3. Check mesh connectivity: `ping <other_node_ip>`

### ATAK Devices Don't See Each Other

1. Confirm bridges are running on both nodes: `ps | grep cot_bridge`
2. Check for "Reticulum link active" in the dashboard or log
3. Verify ATAK is sending: look for TX events in the dashboard

### Destination Hash Changed

The identity is persisted in `/root/.cot_identity`. The hash remains stable unless this file is deleted. If it changes, update the peer:

```bash
echo "<new_hash>" > /root/.cot_peer
/etc/init.d/cot_bridge restart
```

## Supported Features

| Feature | Status | Notes |
|---------|--------|-------|
| Position sharing (SA) | Works | Multicast 239.2.3.1:6969 |
| Chat messages | Works | Multicast 224.10.10.1:17012 |
| Team member icons | Works | Via SA beacons |
| Markers/Points | Works | Via CoT multicast |
| File transfers | Not supported | Too large for Reticulum MTU |
| Video streaming | Not supported | Bandwidth limitations |

## Security

| Layer | Protection |
|-------|------------|
| HaLow Mesh | WPA3 SAE (CCMP) radio encryption |
| Reticulum Link | Curve25519 + AES-128 end-to-end encryption |
| ATAK | Optional additional application-layer encryption |

- The bridge does not inspect or modify CoT content
- Each bridge has a persistent cryptographic identity (`/root/.cot_identity`)
- All data between nodes is encrypted by Reticulum before traversing the HaLow mesh
