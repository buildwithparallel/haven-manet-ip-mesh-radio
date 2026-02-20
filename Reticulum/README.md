# Reticulum on Haven Mesh Networks

[Reticulum](https://reticulum.network/) is a cryptography-based networking stack for building resilient networks over any medium. On Haven nodes, Reticulum provides an encrypted overlay network that operates on top of the HaLow mesh.

**[Haven Guide](https://buildwithparallel.com/products/haven)** - Video tutorials and support for the complete Haven platform.

## Why Reticulum?

- **End-to-end encryption** - All traffic is encrypted by default
- **Transport agnostic** - Works over WiFi, LoRa, serial, or any packet-based medium
- **No central infrastructure** - Fully decentralized, works offline
- **Small footprint** - Runs on resource-constrained devices
- **Future-proof** - Can integrate LoRa RNodes for extreme range

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
│         (ATAK, Sideband, LXMF, Custom Apps)                │
├─────────────────────────────────────────────────────────────┤
│                    Reticulum Stack                          │
│    ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│    │ AutoInterface│  │ UDPInterface │  │ TCPInterface │    │
│    │ (br-ahwlan)  │  │ (broadcast)  │  │  (clients)   │    │
│    └──────────────┘  └──────────────┘  └──────────────┘    │
├─────────────────────────────────────────────────────────────┤
│                    Network Layer                            │
│              br-ahwlan (Linux Bridge)                       │
│    ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│    │   bat0       │  │   wlan0      │  │   phy1-ap0   │    │
│    │ (BATMAN-adv) │  │  (HaLow)     │  │   (5GHz)     │    │
│    └──────────────┘  └──────────────┘  └──────────────┘    │
├─────────────────────────────────────────────────────────────┤
│                    Physical Layer                           │
│         HaLow 916 MHz          5GHz/2.4GHz WiFi            │
└─────────────────────────────────────────────────────────────┘
```

## Installation

Reticulum is pre-installed on Haven nodes. To install manually:

```bash
# Install Python and pip
opkg update
opkg install python3 python3-pip

# Install Reticulum
pip3 install rns
```

## Configuration

The Reticulum configuration file is located at `/root/.reticulum/config` on each node. Edit it to control which radio or network interface Reticulum uses.

```bash
vi /root/.reticulum/config
```

Reticulum is **radio-agnostic**. It doesn't know or care what radio is underneath — it just needs a Linux network interface to bind to. You give it a device name (a bridge, an ethernet adapter, a WiFi interface) and Reticulum uses multicast to auto-discover other nodes on that interface. That's it.

There is nothing HaLow-specific in the Reticulum config. The interface name `[[HaLow Mesh Bridge]]` is just a human-readable label — you could call it `[[My Cool Interface]]` and it would work the same. The `type = AutoInterface` tells Reticulum to use multicast discovery, and `devices = br-ahwlan` tells it which Linux network device to use. Reticulum has no idea what radio is on the other end of that device.

### How HaLow Gets to Reticulum

On Haven nodes, the HaLow radio goes through several layers before Reticulum sees it:

```
HaLow radio (wlan0)  →  BATMAN mesh (bat0)  →  Linux bridge (br-ahwlan)  →  Reticulum
```

- **wlan0** — the physical HaLow 916 MHz radio
- **bat0** — BATMAN-adv mesh routing layer on top of wlan0
- **br-ahwlan** — a Linux bridge containing bat0, defined in OpenWrt's `/etc/config/network`

Reticulum only sees `br-ahwlan`. It sends multicast packets on that bridge, and they happen to travel over HaLow because that's what's bridged in. If you replaced the HaLow radio with a LoRa adapter or an ethernet cable and bridged it into `br-ahwlan`, Reticulum would work without any config change.

### Default Config (same on all nodes)

```ini
[reticulum]
  share_instance = Yes
  enable_transport = Yes
  instance_control_port = 37428

[interfaces]
  # The name in double brackets is just a label — call it anything
  [[HaLow Mesh Bridge]]
    type = AutoInterface        # Use multicast to find peers
    enabled = Yes
    devices = br-ahwlan         # Linux network device to bind to
    group_id = reticulum

  [[UDP Broadcast]]
    type = UDPInterface
    enabled = Yes
    listen_ip = 0.0.0.0
    listen_port = 4242
    forward_ip = 10.41.255.255
    forward_port = 4242
```

> **Note:** The config is identical on green (gate) and blue (point) nodes. Using `listen_ip = 0.0.0.0` binds to all interfaces, so the config works regardless of which IP openmanetd assigns.

### Using a Different Radio

To run Reticulum over a different radio, just change `devices` to that radio's Linux network interface:

| Radio | Interface | Config |
|-------|-----------|--------|
| HaLow (default) | `br-ahwlan` | `devices = br-ahwlan` |
| Standard WiFi | `wlan1` | `devices = wlan1` |
| Ethernet | `eth0` | `devices = eth0` |

After editing, restart Reticulum:

```bash
/etc/init.d/rnsd restart
```

### Interface Types Explained

| Interface | Purpose |
|-----------|---------|
| AutoInterface | Auto-discovers peers on a network device via multicast — radio-agnostic |
| UDPInterface | Broadcasts packets to all nodes on the mesh subnet |

## Running Reticulum

### As a Service (Recommended)
```bash
# Start
/etc/init.d/rnsd start

# Enable at boot
/etc/init.d/rnsd enable

# Check status
python3 /root/rns_status.py
```

### Manually
```bash
rnsd &
```

## Monitoring

### Live Dashboard (rns_status.py)

A live-refreshing dashboard that shows Reticulum status, HaLow radio details, configured interfaces, and real-time data exchange between nodes. See [`scripts/rns_status.py`](../scripts/rns_status.py) for the full script.

```bash
# Standalone — shows status and waits for peers
python3 /root/rns_status.py

# Connect to a peer — enables live PING/PONG exchange
python3 /root/rns_status.py <peer_hash>
```

The dashboard displays:
- Reticulum version, node hash, and link status
- HaLow radio: hardware, frequency, channel, bit rate, signal strength, encryption
- Configured Reticulum interfaces (AutoInterface, UDPInterface, etc.)
- Live packet TX/RX counters and per-peer RTT

### Message Transfer Demo

Simple sender/receiver scripts for testing Reticulum links across the mesh. See [`scripts/rns_send.py`](../scripts/rns_send.py) and [`scripts/rns_receive.py`](../scripts/rns_receive.py).

```bash
# Receiver — prints destination hash, then waits
python3 /root/rns_receive.py

# Sender — resolves path, establishes link, sends message
python3 /root/rns_send.py <dest_hash> Your message here
```

### View Paths
```bash
rnpath -l
```

## Data Flow

When an ATAK device sends a CoT message:

```
1. ATAK sends CoT XML to multicast (SA: 239.2.3.1:6969, Chat: 224.10.10.1:17012)
2. CoT Bridge intercepts multicast, compresses with zlib
3. Bridge fragments if compressed size > 400 bytes
4. Bridge sends over encrypted Reticulum link
5. Reticulum encrypts and transmits via AutoInterface over HaLow mesh
6. Remote node's Reticulum receives and decrypts
7. Remote CoT Bridge reassembles fragments, decompresses
8. Bridge re-publishes to local multicast groups
9. Remote ATAK devices receive CoT data
```

No special ATAK configuration is needed — devices use their default multicast settings. The bridge transparently intercepts and relays traffic.

## MTU Considerations

Reticulum has a 500-byte packet MTU to support low-bandwidth links like LoRa. For larger ATAK messages:

- The bridge compresses data with zlib (typically 1-38% reduction for CoT XML)
- Messages exceeding 400 bytes after compression are fragmented and reassembled
- SA beacons (~300-340 bytes) typically fit in a single packet
- Chat messages (~700-800 bytes) usually require 2 fragments
- Fragmentation adds ~20ms latency per fragment

## Troubleshooting

### No Peers Visible
```bash
# Check interface is up
python3 /root/rns_status.py

# Verify bridge interface exists
ip link show br-ahwlan

# Check multicast is working
tcpdump -i br-ahwlan udp port 4242
```

### High Latency
- Check HaLow signal strength: `iwinfo wlan0 info`
- Verify no packet loss: `ping -c 100 $(uci get network.ahwlan.gateway)`
- Large messages require fragmentation - this adds latency

### Reticulum Won't Start
```bash
# Check for errors
rnsd -v

# Verify config syntax
python3 -c "import RNS; RNS.Reticulum()"
```

## Future Enhancements

- **LoRa Integration**: Add RNode interface for extreme-range backup
- **Sideband Messaging**: Direct encrypted messaging via Reticulum
- **LXMF**: Store-and-forward messaging for offline nodes
