# Haven Gate (Green) - Gateway Node

The **Haven Gate** is the primary gateway node that provides internet uplink to the mesh network. It runs OpenWrt and serves as the DHCP server and default gateway for all mesh clients.

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
│    ├── wlan0 (HaLow 916MHz mesh)     │
│    ├── phy1-ap0 (5GHz AP)            │
│    └── phy2-ap0 (2.4GHz AP)          │
└───────────────────────────────────────┘
    │
    ▼ HaLow 916 MHz Mesh
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
| Frequency | 916 MHz (Channel 28) |
| Mode | Mesh Point |
| Mesh ID | haven |
| Encryption | WPA3 SAE (CCMP) |
| Key | havenmesh |
| Beacon Interval | 1000ms |

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

See [Reticulum/README.md](Reticulum/README.md) for details.

### ATAK Bridge
CoT bridge for ATAK/CivTAK integration over Reticulum.

- Script: `/root/cot_bridge_multicast.py`
- Listens: Multicast 239.2.3.1:6969
- Forwards to: Reticulum broadcast destination

See [Reticulum/ATAK.md](Reticulum/ATAK.md) for details.

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
3. Verify channel is the same (28)
