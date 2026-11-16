# LoRa and Meshtastic on Haven

LoRa runs alongside HaLow. HaLow carries the IP network (video, voice, internet), while LoRa carries small things like positions and short messages over long range and below the noise floor. It's a separate radio, so it also works as a fallback if the HaLow mesh drops. LoRa positions can also appear on the ATAK map (see [Meshtastic on the ATAK map](#meshtastic-on-the-atak-map)).

There are two ways to put LoRa on a Haven. They differ in **where Meshtastic runs**.

| | USB radio (MeshToad) | Sidecar board (Heltec V4, RAK4631, ...) |
|---|---|---|
| What it is | A LoRa chip on a USB adapter, no processor of its own | A complete Meshtastic node with its own processor and firmware |
| Where Meshtastic runs | On the Haven (`meshtasticd`) | On the board |
| What the Haven does | Runs Meshtastic and drives the radio over USB | Powers the board over USB |
| How a phone connects | Over the network to the Haven, port 4403, from anywhere on the mesh | Bluetooth to the board, or the board's own Wi-Fi (ESP32 boards), or through the Haven's USB (see below) |
| Tested on Haven 2 | Yes, Nov 16, 2026 | Not yet |

## USB radio: Null Hop MeshToad

The MeshToad is an SX1262 LoRa chip behind a CH341 USB adapter (USB ID `1a86:5512`). It has no processor, so it can't run Meshtastic by itself. The Haven runs [`meshtasticd`](https://meshtastic.org/docs/software/linux/installation/), the Meshtastic firmware built for Linux, and drives the radio over USB.

```text
phone (Meshtastic app) -- Wi-Fi / HaLow mesh -- Haven: meshtasticd -- USB -- MeshToad ))) LoRa
                    TCP port 4403
```

The app talks to `meshtasticd` over TCP port 4403, with the same messages it would send over Bluetooth or a USB cable. Any phone on any node's Wi-Fi can reach it across the mesh, so one Haven's LoRa radio serves the whole network.

### Set it up (Haven 2)

1. Attach the LoRa antenna, then plug the MeshToad into the Pi 5. The MeshToad has both an SMA and a U.FL (IPEX) connector: use one, never both.
2. Copy [`setup-meshtoad-haven2.sh`](setup-meshtoad-haven2.sh) to the Haven and run it. The Haven needs internet through the gate.

   ```bash
   sh setup-meshtoad-haven2.sh
   ```

   It installs `meshtasticd`, turns on the official `lora-usb-meshtoad-e22.yaml` radio profile, keeps the node's settings in `/etc/meshtasticd/data` (the default under `/var` is wiped on reboot), and starts it at boot.

3. **Connect the Meshtastic app over the network:** Connections → **Network** → **+** → the Haven's mesh IP (for example `10.41.0.6`), port `4403`. It doesn't appear automatically, so enter the address once.
4. **Set the node up from the app:** region, preset, frequency and channel to match your other LoRa nodes (see below), plus a name. The MeshToad has no GPS, so set a fixed position or turn position off.

Why the setup script needs workarounds: the Pi 5 image is OpenWrt 23.05, which has no `aarch64_cortex-a76` package feeds, so it adds OpenWrt's and Meshtastic's `aarch64_generic` 23.05 feeds (they run on the Pi 5). `meshtasticd-full` lists `kmod-bluetooth` and `kmod-spi-dev`, which aren't built for this kernel and aren't needed for a USB radio, so the script registers empty placeholder packages for them. Backups of the changed opkg files are in `/etc/haven-backup/`.

Notes:

- The connection over the network may serve one app at a time; a second phone can drop the first.
- The MeshToad draws its power from the Pi's USB alongside the HaLow radio. If the HaLow link gets unstable, power the Pi from a full 5 A supply. The radio profile also has an optional `SX126X_MAX_POWER` setting to lower the LoRa transmit power.

## Sidecar board: Heltec V4 and similar

A Heltec V4 (ESP32-S3), RAK4631 (nRF52840) or similar board runs Meshtastic firmware itself. Flash it with the [Meshtastic web flasher](https://flasher.meshtastic.org/) first. On the Haven it's a sidecar: the Haven powers it over USB, and the board is a full Meshtastic node on its own. The Haven 2 image already has the USB serial drivers for these boards. **These options haven't been tested on a Haven yet.**

How a phone reaches it:

1. **Bluetooth, straight to the board.** Simplest. The Haven only provides power. The phone has to stay within Bluetooth range of the Haven.
2. **The board's own Wi-Fi (ESP32 boards such as the Heltec V4).** In the Meshtastic app, under Network, turn on Wi-Fi and enter a Haven's Wi-Fi name and password. The board joins the mesh as a Wi-Fi client and serves the same TCP port 4403, so you connect by IP exactly like the MeshToad. On ESP32 Meshtastic, turning Wi-Fi on turns Bluetooth off. nRF52 boards like the RAK4631 have no Wi-Fi.
3. **Through the Haven's USB.** Meshtastic's USB serial connection uses the same message framing as its TCP connection, so the Haven can pass one to the other (for example with `socat`, from the board's `/dev/ttyACM0` to TCP port 4403). The app then connects to the Haven's IP. Not tested yet.

A sidecar can also run [RNode](https://github.com/markqvist/RNode_Firmware) firmware for Reticulum instead of Meshtastic. See the Haven guide's Reticulum section.

## Meshtastic on the ATAK map

The Meshtastic Android app has a built-in TAK server, so ATAK on the same phone can show Meshtastic nodes. It works with either route, as long as the app is connected to a Meshtastic radio.

1. In the Meshtastic app: **Settings → Advanced → TAK Server**. Turn on **Enable Local TAK Server** (`127.0.0.1:8089`) and **Mesh to CoT Converter**, which puts ordinary Meshtastic nodes on the map, not only TAK-role nodes.
2. **Export TAK Data Package** and import the `.zip` into ATAK. ATAK's connection dot turns green.
3. Nodes appear once they send a position. Boards without GPS (MeshToad, MeshPocket) need a fixed position or a phone sharing its location. If a node shows in the app with no distance, tap **Request position** and shorten its position broadcast interval.

Skip the old [Meshtastic ATAK plugin](https://github.com/meshtastic/ATAK-Plugin): 1.1.42 fails to bind to Meshtastic Android 2.7.14 and newer ("Failed to bind to Meshtastic IMesh service"). The app's warning about firmware 2.8.0 is for shapes, markers and routes; positions and chat work on the 2.7 firmware `meshtasticd` runs. Tested on 2026-11-16: a MeshPocket's position reached ATAK through a MeshToad on a Haven 2.

## Settings every LoRa node must share

- **Region:** `US` (or your region)
- **Modem preset:** the same on every node, for example `LongFast`
- **Frequency:** the same slot or override on every node. Keep LoRa outside your HaLow channel: with HaLow on channel 28 (about 912-920 MHz), a LoRa override of `923.0` MHz stays clear. Moving off the default slot means public Meshtastic nodes on the default won't appear.
- **Primary channel:** the same name and key on every node. The default channel shows the local public mesh too; a private channel keeps it to your nodes.
