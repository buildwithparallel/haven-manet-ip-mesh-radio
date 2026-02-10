# Reticulum on Haven Mesh Networks

[Reticulum](https://reticulum.network/) is a cryptography-based networking stack for building resilient networks over any medium. On Haven nodes, Reticulum provides an encrypted overlay network that operates on top of the HaLow mesh.

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

The Reticulum configuration file is located at `~/.reticulum/config`.

### Gate Node (Green) Configuration

```ini
[reticulum]
  share_instance = Yes
  enable_transport = Yes
  instance_control_port = 37428

[interfaces]
  [[HaLow Mesh Bridge]]
    type = AutoInterface
    enabled = Yes
    devices = br-ahwlan
    group_id = reticulum

  [[UDP Broadcast]]
    type = UDPInterface
    enabled = Yes
    listen_ip = 10.41.0.1
    listen_port = 4242
    forward_ip = 10.41.255.255
    forward_port = 4242
```

### Point Node (Blue) Configuration

```ini
[reticulum]
  share_instance = Yes
  enable_transport = Yes
  instance_control_port = 37428

[interfaces]
  [[HaLow Mesh Bridge]]
    type = AutoInterface
    enabled = Yes
    devices = br-ahwlan
    group_id = reticulum

  [[UDP Broadcast]]
    type = UDPInterface
    enabled = Yes
    listen_ip = 10.41.73.196
    listen_port = 4242
    forward_ip = 10.41.255.255
    forward_port = 4242
```

### Interface Types Explained

| Interface | Purpose |
|-----------|---------|
| AutoInterface | Auto-discovers peers on the bridge interface via multicast |
| UDPInterface | Broadcasts packets to all nodes on the mesh subnet |

## Running Reticulum

### As a Service (Recommended)
```bash
# Start
/etc/init.d/rnsd start

# Enable at boot
/etc/init.d/rnsd enable

# Check status
rnstatus
```

### Manually
```bash
rnsd &
```

## Monitoring

### Check Status
```bash
rnstatus
```

Example output:
```
 Shared Instance[rns/default]
    Status    : Up
    Serving   : 1 program
    Rate      : 1.00 Gbps
    Traffic   : ↑7.54 KB  ↓3.73 KB

 AutoInterface[HaLow Mesh Bridge]
    Status    : Up
    Mode      : Full
    Rate      : 10.00 Mbps
    Peers     : 1 reachable
    Traffic   : ↑3.73 KB  ↓3.73 KB

 Transport Instance running
 Uptime is 8m and 39s
```

### View Paths
```bash
rnpath -l
```

## Data Flow

When an ATAK device sends a CoT message:

```
1. ATAK sends CoT XML to multicast 239.2.3.1:6969
2. CoT Bridge receives, compresses with zlib
3. Bridge creates Reticulum packet
4. Reticulum encrypts and sends via AutoInterface
5. Packet travels over HaLow mesh
6. Remote node's Reticulum receives
7. Remote CoT Bridge decompresses
8. Bridge sends to local multicast
9. Remote ATAK receives CoT
```

## MTU Considerations

Reticulum has a 500-byte packet MTU to support low-bandwidth links like LoRa. For larger ATAK messages:

- The bridge compresses data with zlib (typically 40-60% reduction)
- Messages exceeding MTU are fragmented and reassembled
- Fragmentation adds ~10-20ms latency per fragment

## Troubleshooting

### No Peers Visible
```bash
# Check interface is up
rnstatus

# Verify bridge interface exists
ip link show br-ahwlan

# Check multicast is working
tcpdump -i br-ahwlan udp port 4242
```

### High Latency
- Check HaLow signal strength: `iwinfo wlan0 info`
- Verify no packet loss: `ping -c 100 10.41.0.1`
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
