# Haven MANET IP Mesh Radio

A complete guide for building **Haven-style MANET (Mobile Ad-hoc Network) mesh radio** networks using HaLow (802.11ah) radios, Reticulum encrypted networking, and ATAK integration.

## Overview

Haven nodes create a self-healing mesh network operating on 916 MHz (sub-1GHz spectrum) with multi-kilometer range. The network supports:

- **Encrypted communications** via Reticulum
- **ATAK/CivTAK integration** for situational awareness
- **Automatic mesh routing** via BATMAN-adv
- **Internet sharing** across the mesh

```
                         Internet
                             │
                             ▼
┌────────────────────────────────────────────────────────────────┐
│                    Haven Gate (GREEN)                          │
│              Gateway Node with Internet Uplink                 │
│                                                                │
│   External IP: 192.168.0.21    Mesh IP: 10.41.0.1             │
│   Radios: HaLow 916MHz + 5GHz AP + 2.4GHz AP                  │
└────────────────────────────────────────────────────────────────┘
                             │
                             │ HaLow Mesh (916 MHz)
                             │ Range: 1-10+ km
                             ▼
┌────────────────────────────────────────────────────────────────┐
│                   Haven Point (BLUE)                           │
│                 Mesh Extender Node                             │
│                                                                │
│   Mesh IP: 10.41.73.196    Gateway: 10.41.0.1                 │
│   Radios: HaLow 916MHz + 5GHz AP                              │
└────────────────────────────────────────────────────────────────┘
                             │
                             │ 5GHz WiFi
                             ▼
                      [Mobile Devices]
                     ATAK, Phones, etc.
```

## Features

| Feature | Description |
|---------|-------------|
| **HaLow Mesh** | 802.11ah at 916 MHz for long-range backhaul |
| **Reticulum** | Encrypted, resilient networking stack |
| **ATAK Bridge** | Full ATAK/CivTAK support over Reticulum |
| **Auto-healing** | BATMAN-adv mesh routing |
| **Multi-hop** | Traffic routes through multiple nodes |
| **Internet Sharing** | NAT via gateway node |

## Quick Start

### 1. Set Up Gateway Node (Haven Gate)
See [haven-gate.md](haven-gate.md) for complete configuration.

Key settings:
- Mesh IP: `10.41.0.1/16`
- HaLow: Channel 28, Mesh ID "haven"
- Runs DHCP server
- NAT for internet access

### 2. Set Up Point Nodes (Haven Point)
See [haven-point.md](haven-point.md) for complete configuration.

Key settings:
- Mesh IP: `10.41.x.x/16` (unique per node)
- Gateway: `10.41.0.1`
- HaLow: Same channel/mesh ID as Gate

### 3. Enable Reticulum
See [Reticulum/README.md](Reticulum/README.md) for setup.

```bash
# On each node
/etc/init.d/rnsd start
/etc/init.d/rnsd enable
rnstatus
```

### 4. Start ATAK Bridge
See [Reticulum/ATAK.md](Reticulum/ATAK.md) for details.

```bash
# On each node
python3 /root/cot_bridge.py > /tmp/bridge.log 2>&1 &
```

### 5. Connect ATAK
1. Connect phone to Haven node WiFi (e.g., `green-5ghz`)
2. Open ATAK with default settings
3. Other ATAK users on the mesh will appear automatically

## Documentation

| File | Description |
|------|-------------|
| [haven-gate.md](haven-gate.md) | Gateway node configuration |
| [haven-point.md](haven-point.md) | Point/extender node configuration |
| [Reticulum/README.md](Reticulum/README.md) | Reticulum setup and operation |
| [Reticulum/ATAK.md](Reticulum/ATAK.md) | ATAK integration guide |
| [Reticulum/cot_bridge.py](Reticulum/cot_bridge.py) | Bridge script for nodes |

## Hardware

### Tested Platform
- **SBC**: Raspberry Pi 4/CM4
- **HaLow Radio**: Morse Micro MM601X (SPI)
- **5GHz Radio**: Cypress CYW43455 (onboard)
- **2.4GHz Radio**: RT5370 USB (optional)
- **OS**: OpenWrt

### Radio Specifications

| Radio | Band | Range | Throughput |
|-------|------|-------|------------|
| HaLow (802.11ah) | 916 MHz | 1-10+ km | 32.5 Mbps |
| 5GHz WiFi | 5.18 GHz | 50-100m | 300+ Mbps |
| 2.4GHz WiFi | 2.4 GHz | 100-200m | 72 Mbps |

## Network Architecture

### Layer 2 (BATMAN-adv)
- All nodes form a Layer 2 mesh
- Automatic neighbor discovery
- Self-healing routing
- Bridge interface: `br-ahwlan`

### Layer 3 (IP)
- Subnet: `10.41.0.0/16`
- Gateway node provides DHCP
- NAT for internet access

### Layer 4+ (Reticulum)
- Encrypted overlay network
- Application-layer routing
- Works over any transport

## Security

| Layer | Encryption |
|-------|------------|
| HaLow Mesh | WPA3 SAE (CCMP) |
| Reticulum | Curve25519, AES-128 |
| ATAK (optional) | Built-in encryption |

## Troubleshooting

### No Mesh Connectivity
```bash
# Check HaLow interface
iwinfo wlan0 info

# Check BATMAN neighbors
batctl n

# Verify mesh ID matches
uci get wireless.default_radio2.mesh_id
```

### No Internet on Point Nodes
```bash
# Set gateway
uci set network.ahwlan.gateway="10.41.0.1"
uci commit network
/etc/init.d/network reload
```

### ATAK Not Seeing Other Users
```bash
# Check bridge is running
ps | grep cot_bridge

# Check Reticulum peers
rnstatus

# View bridge logs
tail -f /tmp/bridge.log
```

## Contributing

This is an open documentation project. Contributions welcome:
- Hardware variations
- Configuration improvements
- Bug fixes
- Additional use cases

## License

MIT License - See [LICENSE](LICENSE) file.

## Acknowledgments

- [Reticulum Network Stack](https://reticulum.network/) by Mark Qvist
- [ATAK](https://tak.gov/) by TAK Product Center
- [OpenWrt](https://openwrt.org/) Project
- [BATMAN-adv](https://www.open-mesh.org/) mesh protocol
