# Haven MANET IP Mesh Radio

Build decentralized, long-range mesh networks with **Haven** - a complete open-source solution for creating self-healing IP networks that share internet access across kilometers without any central infrastructure.

**[Haven Guide](https://buildwithparallel.com/products/haven)** - Video tutorials, schematics, 3D printable enclosures, Discord community, and direct support.

## What is Haven?

Haven is a mesh networking platform that combines:

- **HaLow radios** (802.11ah) operating in sub-1GHz spectrum for multi-kilometer range
- **BATMAN-adv** for automatic Layer 2 mesh routing
- **OpenMANET** firmware (OpenWrt-based) for reliable embedded networking
- **Optional Reticulum** for encrypted overlay communications
- **Optional ATAK/CivTAK** integration for situational awareness

### Why Haven?

| Feature | Benefit |
|---------|---------|
| **Decentralized** | No central server, no single point of failure |
| **Long Range** | 1-10+ km node-to-node with HaLow radios |
| **Self-Healing** | Automatic route discovery and failover |
| **Internet Sharing** | One uplink serves the entire mesh |
| **Fully Open Source** | No proprietary lock-in, audit everything |
| **Multi-hop** | Traffic routes through intermediate nodes |
| **Low Power** | Sub-1GHz radios are power efficient |

## Haven Nodes

Haven nodes are compact, rugged units built for field deployment. Each node includes HaLow (sub-1GHz) and WiFi radios, USB and power ports, and versatile mounting (GoPro-style bracket and bolt holes).

![Haven node](assets/node-hero.png)

| | | |
|:---:|:---:|:---:|
| ![Node in hand](assets/node-hand.png) | ![Node vehicle mount](assets/node-vehicle-mount.png) | ![Node ports and mount](assets/node-ports-mount.png) |
| Handheld | Vehicle deployment | Ports and mounting |

## Network Architecture

```
                              Internet
                                  │
                                  ▼
    ┌─────────────────────────────────────────────────────────┐
    │              HAVEN GATE — green (Gateway)                │
    │                                                         │
    │   ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐   │
    │   │  eth0   │  │  HaLow  │  │  5GHz   │  │  2.4GHz │   │
    │   │ uplink  │  │  mesh   │  │   AP    │  │   AP    │   │
    │   └─────────┘  └─────────┘  └─────────┘  └─────────┘   │
    │        │            │            │            │         │
    │        └────────────┴────────────┴────────────┘         │
    │                    br-ahwlan bridge                     │
    │                    IP assigned by openmanetd            │
    │                    DHCP Server                          │
    └─────────────────────────────────────────────────────────┘
                                  │
                                  │ HaLow Sub-1GHz Mesh
                                  │ (1-10+ km range)
                                  ▼
    ┌─────────────────────────────────────────────────────────┐
    │              HAVEN POINT — blue (Extender)               │
    │                                                         │
    │             ┌─────────┐          ┌─────────┐            │
    │             │  HaLow  │          │  5GHz   │            │
    │             │  mesh   │          │   AP    │            │
    │             └─────────┘          └─────────┘            │
    │                  │                    │                 │
    │                  └────────────────────┘                 │
    │                    br-ahwlan bridge                     │
    │                    10.41.x.x/16                         │
    └─────────────────────────────────────────────────────────┘
                                  │
                                  ▼
                           [Mobile Devices]
                          Phones, Laptops, ATAK
```

## Getting Started

All Haven setup scripts assume each node is flashed with a fresh/recent version of [OpenMANET](https://openmanet.org/). Flash the image onto a microSD card using Raspberry Pi Imager, insert it into the node, and power on.

Full step-by-step instructions are in [scripts/README.md](scripts/README.md). Quick summary below.

| Step | What | Details |
|------|------|---------|
| **1** | [Set up the Gate node](#step-1-set-up-the-gate-node) | Your first node — shares internet with the mesh |
| **2** | [Add Point nodes](#step-2-add-point-nodes) | Extend the mesh — no internet needed |
| **3** | [Install Reticulum](#step-3-install-reticulum-optional) *(optional)* | Encrypted overlay communications |
| **4** | [Send Reticulum messages](#step-4-send-reticulum-messages-optional) *(optional)* | Test encrypted messaging across the mesh |
| **5** | [Install the ATAK bridge](#step-5-install-the-atak-bridge-optional) *(optional)* | ATAK/CivTAK situational awareness |

### Step 1: Set Up the Gate Node

The gate (green) is your first node — it shares internet with the rest of the mesh.

1. Plug the gate node into your **upstream router via Ethernet**
2. Find the gate's IP in your **router's device list**
3. SSH in (`ssh root@<gate-ip>`) or open `http://<gate-ip>` → **Services → Terminal**
4. Run:
```bash
wget -O setup.sh https://raw.githubusercontent.com/buildwithparallel/haven-manet-ip-mesh-radio/main/scripts/setup-haven-gate.sh
sh setup.sh && reboot
```

### Step 2: Add Point Nodes

Point nodes (blue) extend the mesh — no internet connection needed.

1. Plug Ethernet **directly from your computer to the point node**
2. Open a browser to `http://10.41.254.1` → **Services → Terminal**
3. The point node has no internet, so paste the script directly:
   - Open the [raw setup script](https://raw.githubusercontent.com/buildwithparallel/haven-manet-ip-mesh-radio/main/scripts/setup-haven-point.sh) on your computer
   - Select all, copy, paste into the terminal, press Enter
4. Type `reboot` when finished

**Verify:** Connect to **green-5ghz** WiFi (password: `green-5ghz`) and browse to the point node's mesh IP. If LuCI loads, your mesh is working.

**Connect your device:** After setup, join `green-5ghz` or `blue-5ghz` WiFi from your computer or phone. Your device should get a `10.41.x.x` IP address and can reach any node's web interface at `http://<node-mesh-ip>`. If the gate is plugged into your home router, you can also stay on your home WiFi and access the gate at the IP shown in your router's device list. See [scripts/README.md → Connect Your Device](scripts/README.md#connect-your-device) for details and troubleshooting.

### Step 3: Install Reticulum (Optional)

Adds an encrypted overlay network to the mesh. Run on each node:

```bash
wget -O /tmp/setup-reticulum.sh https://raw.githubusercontent.com/buildwithparallel/haven-manet-ip-mesh-radio/main/scripts/setup-reticulum.sh
sh /tmp/setup-reticulum.sh
/etc/init.d/rnsd enable && /etc/init.d/rnsd start
```

See [Reticulum/README.md](Reticulum/README.md) for configuration and interface details.

### Step 4: Send Reticulum Messages (Optional)

Test encrypted messaging across the mesh using the included demo scripts (`rns_status.py`, `rns_send.py`, `rns_receive.py`). See [scripts/README.md → Step 4](scripts/README.md#step-4-send-reticulum-messages-optional) for full usage and example output.

### Step 5: Install the ATAK Bridge (Optional)

Bridges ATAK/CivTAK situational awareness traffic over Reticulum. Requires [Step 3](#step-3-install-reticulum-optional).

```bash
wget -O /tmp/setup-cot-bridge.sh https://raw.githubusercontent.com/buildwithparallel/haven-manet-ip-mesh-radio/main/scripts/setup-cot-bridge.sh
sh /tmp/setup-cot-bridge.sh
/etc/init.d/cot_bridge enable && /etc/init.d/cot_bridge start
```

See [ATAK/README.md](ATAK/README.md) for peering, dashboards, and troubleshooting.

> After any step, use LuCI's web interface to change passwords, WiFi SSIDs, and other settings. See [Finding Node IPs](#finding-node-ips) to access each node.

### Finding Node IPs

OpenMANET dynamically assigns mesh IPs. To find any node's IP:

- **On the node itself:** `uci get network.ahwlan.ipaddr`
- **From any node on the mesh:** `strings /etc/openmanetd/openmanetd.db | grep -E 'green|blue'`
- **Visually:** Check the boot screen on a connected monitor

Example `strings` output:
```
2c:c6:82:8a:2a:f6 blue 10.41.126.198
```

**Default credentials** (user: `root`):

| Node | Password | WiFi SSID | WiFi Password |
|------|----------|-----------|---------------|
| Gate (green) | `havengreen` | `green-5ghz` | `green-5ghz` |
| Point (blue) | `havenblue` | `blue-5ghz` | `blue-5ghz` |

### Manual Setup

For manual configuration without the setup scripts:

| Document | Description |
|----------|-------------|
| [docs/haven-gate.md](docs/haven-gate.md) | Gate node manual configuration |
| [docs/haven-point.md](docs/haven-point.md) | Point node manual configuration |
| [Reticulum/README.md](Reticulum/README.md) | Reticulum configuration and usage |
| [ATAK/README.md](ATAK/README.md) | ATAK/CivTAK bridge setup |

## HaLow (802.11ah) Radio Specifications

HaLow operates in sub-1GHz ISM bands, providing significantly greater range than traditional WiFi.

### Supported Frequency Bands

| Region | Frequency Range | Common Channels |
|--------|-----------------|-----------------|
| US/FCC | 902-928 MHz | 1-51 |
| EU/ETSI | 863-868 MHz | Varies |
| Japan | 920-928 MHz | Varies |
| Australia | 915-928 MHz | Varies |

### Channel Widths

Max PHY rate depends on the HaLow SoC. Haven ships with the **MM6108** (MCS 0–7, 64-QAM max). The **MM8108** adds MCS 8–9 (256-QAM) for higher peak rates and an integrated 26 dBm PA.

| Width | MM6108 Max | MM8108 Max | Range | Use Case |
|-------|------------|------------|-------|----------|
| 1 MHz | 3.3 Mbps | 4.4 Mbps | Maximum | Long-range backhaul |
| 2 MHz | 7.2 Mbps | 8.7 Mbps | Very Long | Balanced |
| 4 MHz | 15.0 Mbps | 20.0 Mbps | Long | Higher throughput |
| 8 MHz | 32.5 Mbps | 43.3 Mbps | Medium | Local high-speed |

> Real-world throughput is typically 40–60% of PHY rate. See [scripts/README.md](scripts/README.md#channel-width-vs-range) for the full MCS reference table.

### Default Configuration
- **Channel**: 28 (916 MHz center frequency)
- **Width**: 2 MHz (HT20)
- **Encryption**: WPA3 SAE (CCMP)

## Software Stack

All components are open source:

| Component | Version | Description |
|-----------|---------|-------------|
| **OpenMANET** | 24.10 (1.6.1) | OpenWrt-based mesh firmware |
| **OpenWrt** | 24.10 | Base embedded Linux distribution |
| **Linux Kernel** | 6.6.102 | Operating system kernel |
| **Morse Micro Driver** | 1.16.4 | HaLow radio driver |
| **BATMAN-adv** | 2025.4 | Layer 2 mesh routing protocol |
| **Python** | 3.11.14 | Runtime for Reticulum |
| **Reticulum** | 1.1.3 | Encrypted networking stack |

## Hardware Requirements

### Tested Platform
- **SBC**: Raspberry Pi CM4 / Pi 4
- **HaLow Radio**: Morse Micro MM601X (SPI interface)
- **5GHz WiFi**: Cypress CYW43455 (onboard on Pi)
- **2.4GHz WiFi**: RT5370 USB adapter (optional)

### Minimum Requirements
- ARM or x86 device with SPI interface
- HaLow radio module (Morse Micro recommended)
- Standard WiFi for client access

## Use Cases

- **Disaster Response**: Deploy mesh networks where infrastructure is damaged
- **Remote Operations**: Connect sites across kilometers without internet
- **Events**: Temporary networks for large gatherings
- **Maritime**: Ship-to-ship and ship-to-shore communications
- **Agriculture**: Connect sensors and equipment across large properties
- **Community Networks**: Neighborhood internet sharing

## Security

| Layer | Protection |
|-------|------------|
| HaLow Mesh | WPA3 SAE (CCMP) - strongest WiFi encryption |
| Reticulum | Curve25519 + AES-128 end-to-end encryption |
| ATAK | Optional additional encryption |

## Support & Community

- **[Haven Guide](https://buildwithparallel.com/products/haven)** - Complete build guide with videos
- **Discord** - Join the community (link in Haven Guide)
- **Direct Support** - Available through Parallel

## Contributing

Contributions welcome:
- Hardware compatibility testing
- Documentation improvements
- Bug fixes and features
- Use case examples

## License

MIT License - See [LICENSE](LICENSE) file.

## 3D Printable Enclosure

The official Haven case is released into the **public domain** — free for anyone to print, modify, and distribute. Designed for Parallel by [MOROSX](https://morosx.com/).

[Download on Printables](https://www.printables.com/model/1468595-haven-case-for-raspberry-pi-based-manet-by-paralle)

## Acknowledgments

- [OpenMANET](https://openmanet.org/) - Mesh networking firmware
- [Reticulum](https://reticulum.network/) by Mark Qvist
- [ATAK](https://tak.gov/) by TAK Product Center
- [Morse Micro](https://www.morsemicro.com/) - HaLow radio technology
- [OpenWrt](https://openwrt.org/) Project
- [MOROSX](https://morosx.com/) - Haven enclosure design
- [BATMAN-adv](https://www.open-mesh.org/) mesh protocol
