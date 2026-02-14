# Haven Point (Blue) - Mesh Node

The **Haven Point** is a mesh extender node that connects to the Haven Gate over the HaLow mesh backhaul and provides local WiFi access to clients. It relies on the Gate node for internet connectivity and DHCP.

**[Haven Guide](https://buildwithparallel.com/products/haven)** - Video tutorials, schematics, enclosures, and support.

## Overview

| Property | Value |
|----------|-------|
| Hostname | blue |
| Role | Mesh Extender / Access Point |
| Mesh IP | Assigned by openmanetd (run `uci get network.ahwlan.ipaddr` to check) |
| Gateway | Assigned by openmanetd (run `uci get network.ahwlan.gateway` to check) |
| SSH | root / blue |

## Network Architecture

```
                    Internet
                        │
                        ▼
                [Haven Gate (GREEN)]
                  <gate-mesh-ip>
                        │
                        │ HaLow Sub-1GHz Mesh
                        ▼
┌───────────────────────────────────────┐
│           Haven Point (BLUE)          │
│                                       │
│  br-ahwlan: <point-mesh-ip>/16       │
│    ├── bat0 (BATMAN-adv)             │
│    ├── wlan0 (HaLow sub-1GHz mesh)   │
│    └── phy1-ap0 (5GHz AP)            │
└───────────────────────────────────────┘
                        │
                        ▼ 5GHz WiFi
                   [Clients]
```

> **Note:** OpenMANET dynamically assigns mesh IPs on all nodes via its address reservation system. Run `uci get network.ahwlan.ipaddr` on any node to find its current mesh IP.

## Radio Configuration

### HaLow Mesh Radio (802.11ah)
The primary backhaul radio connecting to other Haven nodes.

| Property | Value |
|----------|-------|
| Interface | wlan0 |
| Driver | morse (Morse Micro) |
| Hardware | Morse Micro SPI-MM601X |
| Frequency | Must match Gate node |
| Mode | Mesh Point |
| Mesh ID | haven |
| Encryption | WPA3 SAE (CCMP) |
| Key | havenmesh |
| Beacon Interval | 1000ms |

**Important**: The HaLow channel and width must match the Gate node exactly for mesh connectivity. See [haven-gate.md](haven-gate.md) for frequency band options.

```bash
# Check HaLow link quality
iwinfo wlan0 info

# Expected output:
# Signal: -18 dBm (excellent)
# Link Quality: 70/70
# Bit Rate: 32.5 MBit/s
```

### 5GHz Access Point
Client access point for local devices.

| Property | Value |
|----------|-------|
| Interface | phy1-ap0 |
| Hardware | Cypress CYW43455 |
| Frequency | 5.180 GHz (Channel 36) |
| Mode | Access Point |
| SSID | blue-5ghz |
| Encryption | WPA2 PSK |
| Key | blue-5ghz |
| HT Mode | VHT80 |

## Network Configuration

### Bridge Interface (br-ahwlan)
All interfaces bridged for Layer 2 connectivity.

```
network.ahwlan=interface
network.ahwlan.proto='static'
network.ahwlan.device='br-ahwlan'
network.ahwlan.ipaddr=<assigned by openmanetd>
network.ahwlan.netmask='255.255.0.0'
network.ahwlan.gateway=<gate-mesh-ip>
network.ahwlan.dns='8.8.8.8 8.8.4.4'
```

### Important: Default Gateway
The setup script sets the gateway to the gate's mesh IP. Since openmanetd may reassign the gate's IP, verify the gateway is correct:

```bash
uci get network.ahwlan.gateway
ip route | grep default
```

Traffic flow: `Clients → Blue → HaLow Mesh → Green → Internet`

## Services

### Reticulum
Reticulum network stack for encrypted mesh communication.

- Config: `~/.reticulum/config`
- Service: `/etc/init.d/rnsd`

Configuration:
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
    listen_ip = <point-mesh-ip>
    listen_port = 4242
    forward_ip = 10.41.255.255
    forward_port = 4242
```

### ATAK Bridge
CoT bridge for ATAK integration over Reticulum.

- Script: `/root/cot_bridge.py`
- Identity: `/root/.cot_identity` (persistent across reboots)
- Peer config: `/root/.cot_peer` (contains Gate's destination hash)
- Service: `/etc/init.d/cot_bridge`
- Logs: `/tmp/bridge.log`
- Listens: UDP port 4349
- Connects to: Gate bridge via Reticulum link

Point nodes connect to the Gate bridge by storing its destination hash in
`/root/.cot_peer`. The service reads this file on startup and establishes
the link automatically.

```bash
# Set Gate's destination hash (get it from: head -1 /tmp/bridge.log on Gate)
echo "d9bd729dfc56bcacbe4b007238bf0291" > /root/.cot_peer
/etc/init.d/cot_bridge restart
```

## Management

### SSH Access
Point nodes don't have external IPs. Access via the Gate node:

```bash
# From your computer (via Gate as jump host)
ssh -o ProxyCommand="ssh -W %h:%p root@<gate-upstream-ip>" root@<point-mesh-ip>
# Password: blue

# Or from the Gate node directly
ssh root@<point-mesh-ip>
```

Find the point's mesh IP with `uci get network.ahwlan.ipaddr` on the point node, or check the boot screen on a connected monitor.

### Useful Commands
```bash
# Check mesh connectivity
ping $(uci get network.ahwlan.gateway)  # Ping gate
iwinfo wlan0 info           # HaLow link quality

# Check Reticulum
rnstatus

# Check routing
ip route

# View bridge log
tail -f /tmp/bridge.log
```

## Troubleshooting

### No Internet Connectivity
1. Check gateway is set:
   ```bash
   ip route | grep default
   # Should show: default via <gate-mesh-ip>
   uci get network.ahwlan.gateway
   ```
2. If missing, find the gate's mesh IP (`uci get network.ahwlan.ipaddr` on the gate), then set it:
   ```bash
   uci set network.ahwlan.gateway="<gate-mesh-ip>"
   uci commit network
   /etc/init.d/network reload
   ```
3. Verify DNS:
   ```bash
   uci set network.ahwlan.dns="8.8.8.8 8.8.4.4"
   uci commit network
   ```

### Cannot Reach Gate Node
1. Check HaLow mesh is connected:
   ```bash
   iwinfo wlan0 info | grep -E "Signal|Quality"
   ```
2. Check BATMAN neighbors:
   ```bash
   batctl n
   ```
3. Verify mesh credentials match Gate node

### Clients Get "Connected, No Internet"
1. Android/iOS check connectivity via captive portal detection
2. Usually a DNS issue - ensure DNS is configured:
   ```bash
   uci show network.ahwlan.dns
   ```
