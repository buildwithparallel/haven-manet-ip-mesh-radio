# Getting started — build a Haven mesh

1. **Flash** each node with firmware for its platform and radio. **Haven 1 / Pi 4 or CM4** uses a current [OpenMANET](https://openmanet.net/) image. **Haven 2 / Pi 5** requires the Pi 5-specific OpenWrt/Morse firmware from [`buildwithparallel/openwrt-morse-rpi5`](https://github.com/buildwithparallel/openwrt-morse-rpi5), or the prebuilt [v0.3.0-alpha release](https://github.com/buildwithparallel/openwrt-morse-rpi5/releases/tag/v0.3.0-alpha). Use Raspberry Pi Imager; if the card misbehaves, see the SD card tips in the setup guide.
2. **Gate (green):** follow [Step 1 in the setup guide](setup-guide.md#step-1-set-up-the-gate-node-green) — plug into your upstream router, run the gate script from this repo, reboot.
3. **Point (blue) nodes:** follow [Step 2](setup-guide.md#step-2-add-point-nodes-blue) — usually **copy-paste** the point script (the node may have no internet).
4. **Optional — Reticulum:** usually [Sideband/MeshChat on your devices on Haven WiFi](setup-guide.md#step-3-install-reticulum-optional) (no RNS on nodes). [Node demos and ATAK bridge](setup-guide.md#step-4-send-reticulum-messages-optional) only if you use on-node RNS.

**Full guide:** [setup-guide.md](setup-guide.md)

**Then:** [find node IPs and open LuCI](../reference/finding-nodes.md). If something fails, [troubleshooting](../runbooks/troubleshooting.md).
