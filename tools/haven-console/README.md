# Parallel Toolchain

The source folder, program and service are named `haven-console`.

One page for a Haven mesh, served from the gate. Open it on a phone on any Haven Wi-Fi:

```text
https://10.41.0.1:8443
```

Use your gate's mesh address if it is not `10.41.0.1`. Accept the certificate warning once: the gate makes its own self-signed certificate, and browsers only allow the microphone on HTTPS.

A bar at the top, on every tab, shows the Wi-Fi network this phone joined with band, channel and signal, the HaLow frequency and channel width, and whether the route to the gate is direct or relayed.

The page opens on the Mesh tab and has four tabs.

| Tab | What it shows |
|---|---|
| **Mesh** | Every node with its HaLow neighbors (signal, send and receive rates, link state) and its live route to the gate, from its own routing table. |
| **Route** | Made for demos and filming. Pick a node and see how its traffic reaches the gate as a large diagram: the node on the left, the gate on the right, relays across the top, and traffic flowing along the path in use. Backup paths are drawn faintly. A banner says DIRECT, RELAYED, LINK LOST (with a running timer) or NO ROUTE, and after a switch it shows how many seconds the reroute took. Read from the gate's own batman-adv table every second. **Present full screen** hides everything else. |
| **Performance** | The node this phone is on, the phone's Wi-Fi signal there, the route from that node to the gate (Direct or Relayed through another node), the HaLow signal toward the gate, a speed test to the gate, and internet pings from the gate. |
| **Talk** | Push-to-talk between phones over WebRTC, and a count track the gate can play into the channel. |

The Performance tab is adapted from the Field performance app in the Field Network Operator Blueprint.

## How it works

```text
phone ──Wi-Fi── any node ──HaLow── … ── gate
                  │                        │
            /cgi-bin/haven-node     /cgi-bin/haven-node
            (agent: radios,         haven-console :8443
             routes, Wi-Fi           reads every agent,
             clients)                serves the page
```

- **Agent** (`agent/haven-node.uc`), on every node: a small `ucode` script served by the node's existing web server at `/cgi-bin/haven-node`. It reports the node's hardware model, its HaLow peers and channel (from `morse_cli`), its selected batman-adv route to each other node, which gateway it uses, its access points (network name, frequency, channel) and the Wi-Fi clients on each. It works on OpenMANET (`wlh0`) and Heltec firmware (`wlan0`). It never prints a key.
- **Console** (`haven-console`), on the gate only: a single Go program. Every 3 seconds it reads each node's agent, one node at a time. It finds nodes from the gate's DHCP leases and neighbor table, so there is no node list to keep up to date. It knows which node a phone is on by matching the phone's MAC (from the DHCP lease) against each node's Wi-Fi clients.
- **Talk**: the gate only introduces browsers to each other. The audio goes directly phone to phone, as Opus with forward error correction. The count track is the gate itself joining the channel and streaming a pre-encoded Opus file, using [Pion](https://github.com/pion/webrtc).

## Install

From a computer that can SSH to the gate, with Go installed (or with `build/haven-console-linux-arm64` already built):

```sh
cd tools/haven-console
./install.sh root@<gate address> <point mesh IP> <point mesh IP> ...
```

Example from a home LAN, where the gate's uplink address is `192.168.0.43`:

```sh
./install.sh root@192.168.0.43 10.41.244.64 10.41.0.4
```

Points are reached through the gate. Each node asks for its root password unless you use SSH keys. The script checks every agent and the console at the end.

To remove everything: `./install.sh --remove root@<gate> <points...>`.

The console needs a Raspberry Pi gate (aarch64). The agent runs on any node with `ucode`, `iw`, `batctl` and `uhttpd`, which OpenMANET and Heltec HaLow firmware both have. Nothing is installed with opkg, and no UCI, radio, firewall or key setting changes. Phones reach port 8443 through the `lan` firewall zone, which already accepts input. A firmware update that starts fresh removes the files, so run the installer again afterward.

## Reading the Performance tab

| What you see | What it means |
|---|---|
| You are on | The node whose Wi-Fi this phone joined. |
| Route to the gate | The batman-adv route from that node, by name. **Direct** is one HaLow hop. **Relayed** means another node forwards the traffic, which roughly halves the speed on each extra hop. |
| HaLow signal | What your node hears from the next node toward the gate. Around −50 dBm is strong, −80 dBm is weak. |
| Route speed | batman-adv's estimate for that route, in Mb/s. |
| Speed test | 4 MB down and 2 MB up between the phone and the gate. It stops at the gate, so it does not include the internet line. On a 1 or 2 MHz channel it fills the link while it runs. |
| Internet | The gate pings Cloudflare, Google and Quad9 in turn, every 5 seconds. A new line means the answer made it back to the phone across the mesh. |

## Limits

- **No password.** Anyone on the mesh Wi-Fi can open the page and join Talk. It is not reachable from the internet.
- **iPhones stop web audio when the screen locks.** The page asks the screen to stay on during Talk.
- **Talk is a full mesh of calls,** fine for a handful of phones, not dozens.
- **A node without the agent** still appears in the Mesh tab as a neighbor (by name if alfred knows it, otherwise by MAC), but its own links and the route through it are not shown. If you add the agent to a node later, it can take up to 10 minutes to appear. To show it right away, run `/etc/init.d/haven-console restart` on the gate.
- **Names.** Nodes are named by their agents. The console saves every name it learns to `/etc/haven-console/names`, so a node that goes silent keeps its name. You can also name nodes by hand in `/etc/haven-names` (`<HaLow MAC> <name>` per line).
- **The Mesh tab refreshes every 3 seconds,** so a fast reroute can land between readings. The Route tab reads the gate every second.

## Files

| Path | Role |
|---|---|
| `main.go` | Flags, certificate, routes |
| `mesh.go` | Reads the agents, builds the Performance and Mesh answers |
| `bench.go` | Speed test and pings |
| `route.go` | Route tab: reads the gate's batman-adv table every second and logs reroutes |
| `talk.go` | Talk signaling hub |
| `count.go` | The gate's count track peer |
| `count.ogg` | Counting recording, Opus in 20 ms Ogg pages |
| `web/` | The page and the Parallel mark |
| `agent/haven-node.uc` | Node agent |
| `init.d-haven-console` | procd service for the gate |
| `install.sh` | Installer and remover |

To build by hand:

```sh
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o build/haven-console-linux-arm64 .
```

To replace the count track, re-encode with 20 ms pages, which the console sends one per packet:

```sh
ffmpeg -i input.m4a -ac 1 -ar 48000 -c:a libopus -b:a 48k -application voip -frame_duration 20 -page_duration 20000 count.ogg
```
