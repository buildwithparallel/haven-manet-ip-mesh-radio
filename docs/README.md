# Haven documentation

> [!NOTE]
> Optional overlays (**Reticulum**, **ATAK/CoT**, **ADS-B**) are documented under **[`integrations/`](../integrations/)** in this repo. **3D case (100% free, public domain):** see **[Enclosures](../README.md#enclosures)** in the main readme.

### How the docs fit together

```mermaid
flowchart LR
  A["Flash platform firmware"] --> B["Getting started"]
  B --> C["Reference: IPs, LuCI, CLI"]
  C --> D{Issue?}
  D -->|yes| E["Runbooks: troubleshooting"]
  D -->|no| F["Advanced: RF & antennas"]
  B --> G["Integrations: Reticulum, ATAK, ADS-B"]
  F --> G
```

| I want to… | Start here |
|------------|------------|
| **Flash the right firmware and set up a mesh (gate + points)** | [Getting started](getting-started/README.md) — main walkthrough: [setup guide](getting-started/setup-guide.md) |
| **Find a node's IP, LuCI, SSH** | [Reference — Finding & accessing nodes](reference/finding-nodes.md) |
| **Look up radio / node settings** | [Reference](reference/) — [HaLow](reference/halow-reference.md), [gate](reference/haven-gate.md), [point](reference/haven-point.md) |
| **Pi 5 specifics (Haven 2)** | [Pi 5 notes](reference/pi5-notes.md) |
| **Pi 4 / CM4 specifics (Haven 1)** | [Pi 4 notes](reference/pi4-notes.md) |
| **See tested USB devices (LoRa, WiFi, SDR)** | [Tested USB devices](reference/tested-usb-devices.md) |
| **Build custom OpenWrt firmware** | [OpenWrt build packages](reference/openwrt-build-packages.md) |
| **Fix a problem** | [Runbooks — Troubleshooting](runbooks/troubleshooting.md) |
| **Tune range, antennas, RF** | [Advanced](advanced/) — [range](advanced/range-optimization.md), [antenna smart routing](advanced/antenna-smart-routing.md) |
| **Reticulum (optional overlay)** | [integrations/reticulum/](../integrations/reticulum/README.md) |
| **ATAK / CoT (optional)** | [integrations/atak/](../integrations/atak/README.md) |
| **ADS-B to CoT (optional)** | [integrations/adsb/](../integrations/adsb/README.md) |

Firmware is not built in this repo. **Haven 1 / Raspberry Pi 4 or CM4** uses [OpenMANET](https://openmanet.net/) / [OpenMANET firmware](https://github.com/OpenMANET/firmware). **Haven 2 / Raspberry Pi 5** requires a different OpenWrt/Morse image built from [`buildwithparallel/openwrt-morse-rpi5`](https://github.com/buildwithparallel/openwrt-morse-rpi5), or downloaded from the [v0.3.0-alpha release](https://github.com/buildwithparallel/openwrt-morse-rpi5/releases/tag/v0.3.0-alpha).

Shell scripts for nodes live under [`scripts/`](../scripts/README.md) (`node-setup/`, `node-ops/`, `optional/`, `tools/`).
