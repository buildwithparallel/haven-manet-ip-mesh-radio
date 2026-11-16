# ADS-B to CoT

Display aircraft in TAK — ADS-B feed to TAK gateway.

ADSBCOT is available for OpenMANET via `opkg`, but it is not installed by default. Once installed and enabled, you can forward aircraft tracks into the Team Awareness Kit (TAK) ecosystem with minimal effort. The integration is intentionally opinionated: it assumes an RTL-SDR-based receiver and multicasts the resulting Cursor on Target (CoT) data across the mesh for TAK clients to consume.

For deeper details, refer to the [official ADSBCOT documentation](https://adsbcot.readthedocs.io/).

**Demo:** [ADS-B to CoT on Haven — YouTube](https://www.youtube.com/watch?v=ugHFq4Bsd8o)

![RTL-SDR dongle connected to a compact ADS-B antenna](../../assets/rtl-sdr-adsb-dongle.png)

## Features

- Converts ADS-B messages to CoT format for TAK clients
- Preserves aircraft track, course, speed vectors, and metadata
- Compatible with ATAK, TAKX, WinTAK, and iTAK
- Supports multiple ADS-B data aggregators and CoT receivers
- Accepts over-the-air RF ADS-B via SDR hardware
- Runs on Python 3.7+ across Windows and Linux

## Getting Started on OpenMANET

### 1. Install ADSBCOT

Install via LuCI (**System > Software > Update lists** > search `adsbtocot` > Install) or CLI:

```bash
opkg update
opkg install adsbtocot
```

If using an older `adsbtocot` package revision that does not pull crypto dependencies automatically:

```bash
opkg install python3-cryptography
```

### 2. Connect the SDR

Plug an RTL-SDR dongle into your Raspberry Pi (USB 3 preferred).

A dedicated ADS-B antenna is not required for basic reception. A 915 MHz antenna works fine for this setup, especially when you have a clear view of the sky. In the demo video, the antenna used is the [Yogurt Cup Antenna from Heads Down Comms Up](https://fivefourcommunications.com/product/yca-lorawan-868-915mhz/) — a compact sub-gigahertz antenna.

![Field ADS-B receiver setup with antenna, RTL-SDR, and Haven node](../../assets/rtl-sdr-adsb-field-setup.png)

### 3. Enable and Start Services

ADS-B to CoT relies on two OpenWrt services:

| Service | Role |
|---------|------|
| `dump1090` | Collects ADS-B frames from the SDR |
| `adsbcot` | Converts ADS-B feed to CoT |

Enable via the OpenWrt GUI (**System > Startup**) or CLI:

```bash
/etc/init.d/dump1090 enable
/etc/init.d/dump1090 start
/etc/init.d/adsbcot enable
/etc/init.d/adsbcot start
```

### 4. Verify Service State

```bash
which adsbcot
pgrep -af "adsbcot|dump1090"
logread -e adsbcot | tail -n 40
```

### 5. Confirm in TAK

Open ATAK, WinTAK, or iTAK and verify aircraft CoT markers are appearing on the map.

![Aircraft markers displayed in ATAK from ADS-B to CoT](../../assets/atak-adsb-aircraft-markers.png)

![Aircraft visible overhead while testing ADS-B reception](../../assets/adsb-aircraft-overhead.png)

## Getting Started on Haven 2 (Raspberry Pi 5)

Haven 2 runs the [Pi 5 OpenWrt/Morse image](https://github.com/buildwithparallel/openwrt-morse-rpi5), not OpenMANET, so the `opkg install adsbtocot` step above does not apply. The image already includes `rtl-sdr`, `dump1090` and Python 3 with `cryptography`. Two things are missing or broken out of the box, and [`setup-haven2.sh`](setup-haven2.sh) handles both:

- **`adsbcot` is no longer on PyPI,** so `pip3 install adsbcot` fails with "No matching distribution". The script installs the tagged release (9.2.2) from the [upstream GitHub repo](https://github.com/snstac/adsbcot) instead.
- **The image's `dump1090` service exits at startup.** Its default config passes `--html-dir`, which this `dump1090` (FlightAware 4.0) rejects. The script clears that option.

Tested end to end on a Haven 2 (Pi 5 + MM8108 USB) on Nov 16, 2026: aircraft showed up in ATAK on a phone on the mesh.

### 1. Connect the SDR

Attach the antenna, then plug the RTL-SDR into the Pi 5. Check it shows up:

```bash
lsusb | grep -i rtl
```

The Pi 5's HaLow radio is also on USB. If the HaLow link drops after you plug in the SDR, power the Pi from a full 5 A supply or put the SDR on a powered USB hub.

### 2. Run the setup script

The node needs internet through the gate for the `adsbcot` download. Copy [`setup-haven2.sh`](setup-haven2.sh) to the node and run it:

```bash
sh setup-haven2.sh
```

It installs `adsbcot`, turns `dump1090` on, writes `/etc/adsbcot.ini`, adds `/etc/init.d/adsbcot`, and starts both services at boot. At the end it prints how many aircraft `dump1090` has decoded.

`adsbcot` reads the aircraft list `dump1090` writes (`/var/run/dump1090/aircraft.json`) and sends Cursor on Target to ATAK's default SA multicast group, `239.2.3.1:6969`:

```ini
[adsbcot]
FEED_URL = file:///var/run/dump1090/aircraft.json
POLL_INTERVAL = 2
COT_URL = udp+wo://239.2.3.1:6969
```

### 3. Check it

```bash
pidof dump1090                                    # dump1090 is running
ls /var/run/dump1090/aircraft.json                # it is writing aircraft
tcpdump -n -i br-ahwlan -c 3 udp port 6969        # CoT is going out on the mesh
```

batman-adv floods multicast across the mesh, so the feed reaches every node's Wi-Fi. Open ATAK on a phone joined to any Haven Wi-Fi; aircraft appear without any ATAK setup, because ATAK listens on that group by default.

`adsbcot` logs `No value for lat=None lon=None` for aircraft that have not sent a position yet. That is normal.

## Optional: Offline Base Map for the Browser Map

`dump1090` also serves a browser map (FlightAware SkyAware) at `http://<haven-ip>:8080`, for any phone or laptop on the mesh. The planes come from the Haven, but its background map tiles come from the internet, so in the field the planes sit on a blank background.

[`skyaware-offline/`](skyaware-offline) adds an **Offline (Natural Earth)** base map: land, water, coastlines, lakes, state lines, highways, towns and airports around your area, from [Natural Earth](https://www.naturalearthdata.com/) (public domain). It's usually under 1 MB, so it loads quickly over HaLow.

1. **On a computer with internet,** build the map for your area. Whole degrees are fine for the center:

   ```bash
   cd skyaware-offline
   python3 build-offline-map.py <lat> <lon>
   ```

   It downloads Natural Earth once (about 100 MB, cached) and writes a small `offline/` folder.

2. **Copy the folder to the Haven and install it:**

   ```bash
   scp -O -r skyaware-offline root@<haven-ip>:/tmp/
   ssh root@<haven-ip> sh /tmp/skyaware-offline/install.sh <lat> <lon>
   ```

`install.sh` copies the map data into `/usr/share/dump1090/geojson/offline/` and patches SkyAware:

- the offline map becomes the first base layer, so browsers start on it (the online maps stay in the layers menu);
- the map opens centered on your area instead of SkyAware's default (Italy);
- browsers reload the patched files instead of a cached copy.

It's safe to run again. Originals are saved in `/etc/haven-backup/skyaware/`. A firmware upgrade replaces SkyAware, so run it again after one. Tested on a Haven 2 on Nov 16, 2026.

If the map shows "AJAX call failed" on Android, the phone is probably sending traffic over mobile data because the Haven's Wi-Fi has no internet. Choose **Stay connected** for the Haven's Wi-Fi, or turn off mobile data.

## Optional: The Map on a Tesla Screen (HTTPS by Name)

The Tesla browser only loads `https://` and refuses private `10.x` addresses. [`https/`](https/README.md) serves the map at `https://<your-name>:8443` with a Let's Encrypt certificate and a `100.64.x.x` address the Tesla accepts, all working offline on the mesh. Tested on a Tesla on Nov 16, 2026.

## What to Expect

- **No internet required.** ADS-B is passive RF reception — the Haven node picks up aircraft transponder signals directly over the air and multicasts the CoT data to TAK clients on its WiFi. No uplink needed.
- **Standard antennas work fine.** No dedicated ADS-B antenna is required for basic testing — the HaLow or LoRa antenna already on the node is enough to pick up traffic with a clear view of the sky. The demo video uses the [Yogurt Cup Antenna from Heads Down Comms Up](https://fivefourcommunications.com/product/yca-lorawan-868-915mhz/) (sub-gigahertz). A 1090 MHz-tuned ADS-B antenna will improve range and sensitivity, but is far from necessary to get started.
- **Range.** In open conditions expect roughly 50–100 miles of aircraft coverage. Results vary by antenna height, obstructions, and aircraft altitude.
- **What you'll see in ATAK.** Each aircraft appears as a marker on the map. Tap it to see track, course, speed, altitude, and aircraft metadata pulled from the ADS-B transponder.

## Requirements

- RTL-SDR dongle (USB 3 recommended)
- Antenna — any 915 MHz, HaLow, or LoRa antenna works for basic testing; a 1090 MHz-tuned ADS-B antenna improves range
- A Haven node, gate or point: Haven 1 (OpenMANET) or Haven 2 (Pi 5 OpenWrt/Morse image)
- TAK client (ATAK, WinTAK, iTAK, or TAKX) connected to the Haven node's WiFi

## Troubleshooting

### No Aircraft Appearing in TAK

1. **SDR not detected** — check `lsusb` for the RTL-SDR device
2. **dump1090 not running** — `pgrep -af dump1090`; check `logread -e dump1090`
3. **adsbcot not running** — `pgrep -af adsbcot`; check `logread -e adsbcot`
4. **No aircraft in range** — verify dump1090 is receiving frames: `wget -qO- http://localhost:8080/data.json 2>/dev/null | head`
5. **TAK client not on mesh WiFi** — phone/tablet must be connected to the Haven node's AP
