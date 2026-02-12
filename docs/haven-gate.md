# Haven Gate (Green) - Gateway Node

The **Haven Gate** is the primary gateway node that provides internet uplink to the mesh network. It runs OpenWrt and serves as the DHCP server and default gateway for all mesh clients.

**[Haven Guide](https://buildwithparallel.com/products/haven)** - Video tutorials, schematics, enclosures, and support.

## Overview

| Property | Value |
|----------|-------|
| Hostname | green |
| Role | Gateway / Internet Uplink |
| Mesh IP | 10.41.0.1 |
| External IP | 192.168.0.21 (DHCP from upstream) |
| SSH | root / green |

## Network Architecture

```
Internet
    │
    ▼
[Upstream Router]
    │ 192.168.0.0/24
    ▼
[eth0: 192.168.0.21]
    │
┌───┴───────────────────────────────────┐
│           Haven Gate (GREEN)          │
│                                       │
│  br-ahwlan: 10.41.0.1/16             │
│    ├── bat0 (BATMAN-adv)             │
│    ├── wlan0 (HaLow sub-1GHz mesh)   │
│    ├── phy1-ap0 (5GHz AP)            │
│    └── phy2-ap0 (2.4GHz AP)          │
└───────────────────────────────────────┘
    │
    ▼ HaLow Sub-1GHz Mesh
[Other Haven Nodes]
```

## Radio Configuration

### HaLow Mesh Radio (802.11ah)
The primary backhaul radio operating in sub-1GHz spectrum for long-range mesh connectivity.

| Property | Value |
|----------|-------|
| Interface | wlan0 |
| Driver | morse (Morse Micro) |
| Hardware | Morse Micro SPI-MM601X |
| Frequency | Region-dependent (see below) |
| Mode | Mesh Point |
| Mesh ID | haven |
| Encryption | WPA3 SAE (CCMP) |
| Key | havenmesh |
| Beacon Interval | 1000ms |

#### HaLow Frequency Bands by Region

| Region | Frequency Range | Example Channels |
|--------|-----------------|------------------|
| US/FCC | 902-928 MHz | 1-51 |
| EU/ETSI | 863-868 MHz | Varies |
| Japan | 920-928 MHz | Varies |
| Australia | 915-928 MHz | Varies |

#### Channel Widths

| Width | Throughput | Range | Best For |
|-------|------------|-------|----------|
| 1 MHz | ~1.5 Mbps | Maximum | Long-range backhaul |
| 2 MHz | ~4 Mbps | Very Long | Balanced range/speed |
| 4 MHz | ~15 Mbps | Long | Higher throughput |
| 8 MHz | ~32 Mbps | Medium | Local high-speed |

Configure frequency via OpenWrt:
```bash
uci set wireless.radio2.channel='28'     # Set channel number
uci set wireless.radio2.htmode='HT20'    # Set channel width (1/2/4/8 MHz)
uci commit wireless
wifi reload
```

```bash
# OpenWrt wireless config
uci show wireless.radio2
uci show wireless.default_radio2
```

### 5GHz Access Point
Client access point for local devices.

| Property | Value |
|----------|-------|
| Interface | phy1-ap0 |
| Hardware | Cypress CYW43455 |
| Frequency | 5.180 GHz (Channel 36) |
| Mode | Access Point |
| SSID | green-5ghz |
| Encryption | WPA2 PSK |
| Key | green-5ghz |
| HT Mode | VHT80 |

### 2.4GHz Access Point
Secondary client access point for legacy devices.

| Property | Value |
|----------|-------|
| Interface | phy2-ap0 |
| Hardware | Generic USB RT5370 |
| Frequency | 2.437 GHz (Channel 6) |
| Mode | Access Point |
| SSID | green-2.4ghz |
| Encryption | WPA2 PSK |
| Key | green-2.4ghz |
| HT Mode | HT20 |

## Network Configuration

### Bridge Interface (br-ahwlan)
All mesh and client interfaces are bridged together.

```bash
# View bridge members
brctl show br-ahwlan

# OpenWrt config
uci show network.ahwlan
```

Configuration:
```
network.ahwlan=interface
network.ahwlan.proto='static'
network.ahwlan.device='br-ahwlan'
network.ahwlan.ipaddr='10.41.0.1'
network.ahwlan.netmask='255.255.0.0'
```

### DHCP Server
The gate node runs the DHCP server for all mesh clients.

```
dhcp.ahwlan=dhcp
dhcp.ahwlan.interface='ahwlan'
dhcp.ahwlan.start='100'
dhcp.ahwlan.limit='16'
dhcp.ahwlan.leasetime='12h'
dhcp.ahwlan.force='1'
```

### Firewall / NAT
The gate node performs NAT for internet access:
- Mesh clients (10.41.0.0/16) → NAT → eth0 → Internet

## Services

### Reticulum
Reticulum network stack runs as a transport daemon.

- Config: `~/.reticulum/config`
- Service: `/etc/init.d/rnsd`
- Status: `rnstatus`

See [../Reticulum/README.md](../Reticulum/README.md) for details.

### ATAK Bridge
CoT bridge for ATAK/CivTAK integration over Reticulum.

- Script: `/root/cot_bridge.py`
- Identity: `/root/.cot_identity` (persistent across reboots)
- Peer config: `/root/.cot_peer` (optional — Gate typically has no peer)
- Service: `/etc/init.d/cot_bridge`
- Logs: `/tmp/bridge.log`
- Listens: UDP port 4349
- Forwards to: Reticulum link → peer bridge

The Gate bridge runs without a peer hash — it listens for inbound links from
Point nodes. Get its destination hash with `head -1 /tmp/bridge.log` and
provide it to each Point node during their CoT bridge setup.

See [../ATAK/README.md](../ATAK/README.md) for details.

## Management

### SSH Access
```bash
ssh root@192.168.0.21
# Password: green
```

### Useful Commands
```bash
# Check mesh status
iwinfo wlan0 info
batctl n          # BATMAN neighbors

# Check Reticulum
rnstatus

# Check bridge
brctl show br-ahwlan

# View logs
logread -f
tail -f /tmp/bridge.log
```

## Troubleshooting

### No Internet for Mesh Clients
1. Check NAT/masquerade is enabled in firewall
2. Verify forwarding: `cat /proc/sys/net/ipv4/ip_forward`
3. Check firewall rules: `nft list ruleset`

### HaLow Mesh Not Forming
1. Verify mesh ID matches on all nodes: `iwinfo wlan0 info`
2. Check encryption key matches
3. Verify channel and width are identical on all nodes:
   ```bash
   uci get wireless.radio2.channel
   uci get wireless.radio2.htmode
   ```
