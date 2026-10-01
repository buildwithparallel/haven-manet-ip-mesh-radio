# Pi 4 / CM4 Platform Notes (Haven 1)

Haven 1 runs on the Raspberry Pi 4 or Compute Module 4 with an SPI-attached HaLow HAT.

> Haven 1 is fully supported. If you already have Haven 1 nodes, they mesh seamlessly with Haven 2 nodes — the HaLow radio protocol and BATMAN-adv mesh are the same across both platforms.

## Hardware

- **SBC**: Raspberry Pi 4 Model B or Compute Module 4
- **HaLow Radio**: Morse Micro MM601X via SPI (Seeed WM-WM6108 HAT)
- **5GHz WiFi**: Cypress CYW43455 (onboard)
- **2.4GHz WiFi**: RT5370 USB adapter (optional)
- **Firmware**: Use a current [OpenMANET](https://openmanet.net/) image for Pi 4 / CM4. Haven 2 / Pi 5 uses a different OpenWrt/Morse firmware image; see [Pi 5 notes](pi5-notes.md#firmware).

## HaLow HAT SPI Failures

The Seeed WM-WM6108 HAT can fail on boot with:

```
morse_spi spi0.0: cmd53_write/read failed (errno=-71)
```

**Cause:** Case standoffs can bow the HAT off the GPIO header, causing intermittent SPI contact.

**Fix:** Add 1mm standoff height; press the HAT firmly onto the header before closing the case.

**Recovery (if HAT is seated):** `rmmod morse && modprobe morse` — no full power cycle needed.

## Setup

Haven 1 uses the same setup scripts as Haven 2:

- **Gate:** [setup-haven-gate.sh](../../scripts/node-setup/setup-haven-gate.sh)
- **Point:** [setup-haven-point.sh](../../scripts/node-setup/setup-haven-point.sh)

See the [setup guide](../getting-started/setup-guide.md) for step-by-step instructions.
