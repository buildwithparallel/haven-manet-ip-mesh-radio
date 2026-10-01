# Haven MANET IP Mesh Radio

Build decentralized, long-range mesh networks with **Haven** — a complete open-source solution for creating self-healing IP networks that share internet access across kilometers without any central infrastructure.

> [!TIP]
> **Get on the air:** flash the correct firmware for your platform, then the **[setup guide](docs/getting-started/setup-guide.md)**. **Haven 1 / Pi 4:** use [OpenMANET](https://openmanet.net/). **Haven 2 / Pi 5:** use the [Pi 5 OpenWrt/Morse firmware](https://github.com/buildwithparallel/openwrt-morse-rpi5). **Find a node:** **[Finding & accessing nodes](docs/reference/finding-nodes.md)**. **Print a case** (100% free): **[Enclosures](#enclosures)** or [Haven case on Printables](https://www.printables.com/model/1468595-haven-case-for-raspberry-pi-based-manet-by-paralle). **Build help & community:** [Haven Guide](https://buildwithparallel.com/products/haven) (videos, schematics, Discord, support).

## What is Haven?

Haven is a mesh networking platform that combines:

- **HaLow radios** (802.11ah) operating in sub-1GHz spectrum for multi-kilometer range
- **BATMAN-adv** for automatic Layer 2 mesh routing
- **OpenWrt-based firmware** for reliable embedded networking: OpenMANET on Haven 1 / Pi 4, and the Pi 5 OpenWrt/Morse image on Haven 2
- **USB sidecar support** for LoRa radios, RTL-SDR, and other peripherals
- **Optional ADS-B to CoT** for aircraft tracking in TAK
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
| **Expandable** | USB peripherals: LoRa sidecars, SDR, cellular |

## Hardware

Haven runs on two platforms. Both use the same mesh protocol and interoperate fully.

### Haven 1 (Raspberry Pi 4 / CM4) — Recommended

The current recommended platform. Proven stable in the field.

- **SBC**: Raspberry Pi 4 Model B or Compute Module 4
- **HaLow Radio**: Morse Micro MM601X (SPI HAT)
- **2.4GHz WiFi**: RT5370 USB adapter (Panda Wireless or similar)
- **Platform details**: [Pi 4 notes](docs/reference/pi4-notes.md)

### Haven 2 (Raspberry Pi 5) — Experimental

USB-attached HaLow radio with a flexible USB peripheral bus. Still being validated — not yet recommended for production deployments.

- **SBC**: Raspberry Pi 5 (4GB or 8GB)
- **HaLow Radio**: Morse Micro MM8108 (USB)
- **2.4GHz WiFi**: RT5370 USB adapter (Panda Wireless or similar)
- **USB Hub**: Recommended for additional peripherals
- **Firmware**: Requires the Pi 5-specific OpenWrt/Morse image, not the Haven 1 / Pi 4 OpenMANET image. Build it from [`buildwithparallel/openwrt-morse-rpi5`](https://github.com/buildwithparallel/openwrt-morse-rpi5), or download the prebuilt [v0.3.0-alpha release](https://github.com/buildwithparallel/openwrt-morse-rpi5/releases/tag/v0.3.0-alpha).
- **Platform details**: [Pi 5 notes](docs/reference/pi5-notes.md)

## Haven Nodes

Haven nodes are compact, rugged units built for field deployment. Each node includes HaLow (sub-1GHz) and WiFi radios, USB and power ports, and versatile mounting (GoPro-style bracket and bolt holes).

![Haven node](assets/node-hero.png)

| | | |
|:---:|:---:|:---:|
| ![Node in hand](assets/node-hand.png) | ![Node vehicle mount](assets/node-vehicle-mount.png) | ![Node ports and mount](assets/node-ports-mount.png) |
| Handheld | Vehicle deployment | Ports and mounting |

## Enclosures

The official [Haven case for Raspberry Pi-based MANET](https://www.printables.com/model/1468595-haven-case-for-raspberry-pi-based-manet-by-paralle) on [Printables](https://www.printables.com/) is **100% free** — **public domain**: no cost to download, and you may print, modify, and share without restriction. Enclosure design: [MOROSX](https://morosx.com/).

- **[Download the Haven case (Printables)](https://www.printables.com/model/1468595-haven-case-for-raspberry-pi-based-manet-by-paralle)** — STLs and part notes are on that page
- The [Haven Guide](https://buildwithparallel.com/products/haven) also places the case in the full build (radios, power, mounting, etc.)

## Network Architecture

```mermaid
flowchart TB
  Internet((Internet / WAN)) -->|uplink| G["Haven GATE — eth0, HaLow mesh, WiFi APs, br-ahwlan, DHCP"]
  G ---|"802.11ah sub-1 GHz (typ. 1-10+ km)"| P["Haven POINT — HaLow mesh, WiFi APs, br-ahwlan 10.41.x.x/16"]
  P --> C((Phones, laptops, ATAK))
  P -.-|"USB sidecar (optional)"| L((LoRa / Reticulum))
  G -.-|"RTL-SDR (optional)"| A((ADS-B aircraft tracking))
```

## Quick Start

All Haven setup scripts assume each node is flashed with fresh firmware for its hardware. **Haven 1 / Raspberry Pi 4 or CM4** nodes use [OpenMANET](https://openmanet.net/). **Haven 2 / Raspberry Pi 5** nodes require the Pi 5-specific OpenWrt/Morse firmware built from [`buildwithparallel/openwrt-morse-rpi5`](https://github.com/buildwithparallel/openwrt-morse-rpi5), or downloaded from the [v0.3.0-alpha release](https://github.com/buildwithparallel/openwrt-morse-rpi5/releases/tag/v0.3.0-alpha). Do not flash the Haven 1 / Pi 4 OpenMANET image onto a Haven 2 / Pi 5 node.

Flash the image onto a microSD card using Raspberry Pi Imager, insert it into the node, and power on. If the card still looks like it has old data after flashing, use Raspberry Pi Imager's **Erase** (or SD **format/erase** utility) on the card first, then write the image.

| Step | What | How |
|------|------|-----|
| **1** | Set up the Gate node | Flash platform firmware → run first-boot / mesh wizard → plug into router → SSH in → run setup script → [Setup Guide](docs/getting-started/setup-guide.md#step-1-set-up-the-gate-node-green) |
| **2** | Add Point nodes | Plug into laptop, paste setup script → [Setup Guide](docs/getting-started/setup-guide.md#step-2-add-point-nodes-blue) |
| **3** | ADS-B to CoT *(optional)* | Aircraft tracking in TAK via RTL-SDR → [ADS-B](integrations/adsb/README.md) |
| **4** | Reticulum *(optional)* | Apps on your phone/laptop (Sideband, MeshChat) on Haven WiFi → [Reticulum](integrations/reticulum/README.md) |
| **5** | ATAK Bridge *(optional)* | Situational awareness → [ATAK](integrations/atak/README.md) |
| **6** | LoRa sidecar *(optional)* | USB LoRa MCU for Reticulum → [Tested devices](docs/reference/tested-usb-devices.md) |

> After any step, use LuCI's web interface to change passwords, WiFi SSIDs, and other settings. See **[Finding & Accessing Nodes](docs/reference/finding-nodes.md)** to reach each node.

> [!CAUTION]
> The defaults below are for first boot only. **Change the root password and WiFi credentials** in LuCI before you rely on this in the field.

<details>
<summary><strong>Default <code>root</code> credentials and WiFi (first boot)</strong></summary>

| Node | Password | WiFi SSID | WiFi Password |
|------|----------|-----------|---------------|
| Gate (green) | `havengreen` | `green` | `greengreen` |
| Point (blue) | `havenblue` | `blue` | `blueblue` |

</details>

## Documentation

| Document | What it covers |
|----------|----------------|
| **[Docs home (sitemap)](docs/README.md)** | Where everything lives: getting started, reference, runbooks, advanced |
| **[Setup Guide](docs/getting-started/setup-guide.md)** | Step-by-step: gate setup, point nodes, Reticulum, ATAK |
| **[Finding & Accessing Nodes](docs/reference/finding-nodes.md)** | How to find node IPs and reach LuCI |
| **[Pi 5 Notes](docs/reference/pi5-notes.md)** | Haven 2 platform specifics, known issues, fixes |
| **[Pi 4 Notes](docs/reference/pi4-notes.md)** | Haven 1 platform specifics |
| **[Tested USB Devices](docs/reference/tested-usb-devices.md)** | LoRa boards, WiFi adapters, SDR — tested and working |
| **[OpenWrt Build Packages](docs/reference/openwrt-build-packages.md)** | Recommended packages for custom firmware builds |
| **[HaLow Reference](docs/reference/halow-reference.md)** | Radio specs, channel widths, MCS tables, software versions |
| **[Troubleshooting](docs/runbooks/troubleshooting.md)** | Mental model, diagnostics, fix checklists |
| **[Range Optimization](docs/advanced/range-optimization.md)** | Antenna selection, TX power, channel width tuning |
| **[Reticulum](integrations/reticulum/README.md)** | Encrypted overlay communications |
| **[ATAK Bridge](integrations/atak/README.md)** | ATAK/CivTAK situational awareness over Reticulum |
| **[ADS-B to CoT](integrations/adsb/README.md)** | Aircraft tracking in TAK via RTL-SDR |
| **[Scripts](scripts/README.md)** | Script reference and Reticulum demo tools |
| **[AI Agents](AGENTS.md)** | Context for AI agents (Claude, Cursor, etc.) to diagnose and fix your mesh |

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

Contributions welcome. Examples:

- [ ] Hardware compatibility testing
- [ ] Documentation improvements
- [ ] Bug fixes and features
- [ ] New use case write-ups

## License

MIT License - See [LICENSE](LICENSE) file.

## Acknowledgments

- [OpenMANET](https://openmanet.net/) - Mesh networking firmware
- [Reticulum](https://reticulum.network/) by Mark Qvist
- [ATAK](https://tak.gov/) by TAK Product Center
- [Morse Micro](https://www.morsemicro.com/) - HaLow radio technology
- [OpenWrt](https://openwrt.org/) Project
- [MOROSX](https://morosx.com/) - Haven enclosure design
- [BATMAN-adv](https://www.open-mesh.org/) mesh protocol
