# Using AI Agents with Haven

You can use AI coding agents (Claude Code, Cursor, Copilot, etc.) to diagnose and fix Haven mesh problems. This page tells the AI what it needs to know.

## For humans

If you're using an AI agent to help with your mesh, point it at this repo and tell it what's wrong. For example:

- "Blue node's WiFi clients have no internet, can you SSH in and fix it?"
- "I can't reach the point node's LuCI from the gate's WiFi"
- "Run haven-diag.sh on all my nodes and tell me what's broken"

The agent can SSH into nodes (if you give it credentials), run diagnostics, and apply fixes.

### Quick start

1. Clone this repo (or just give the agent the URL)
2. Tell the agent your node IPs and passwords
3. Describe the problem

---

## For AI agents

You are helping a user manage a Haven mesh network. This section gives you the context you need.

### What is Haven?

Haven is a HaLow (802.11ah) mesh network using BATMAN-adv for Layer 2 routing on OpenWrt/OpenMANET. Nodes bridge everything through `br-ahwlan` on the `10.41.x.x/16` subnet. One node (the "gate") has internet via `eth0` and runs DHCP for the whole mesh.

**Firmware/platform note:** Haven 1 (Raspberry Pi 4 / CM4) uses OpenMANET images. Haven 2 (Raspberry Pi 5) requires a different Pi 5-specific OpenWrt/Morse firmware image built from [`buildwithparallel/openwrt-morse-rpi5`](https://github.com/buildwithparallel/openwrt-morse-rpi5), or downloaded from the prebuilt [v0.3.0-alpha release](https://github.com/buildwithparallel/openwrt-morse-rpi5/releases/tag/v0.3.0-alpha). Do not tell users to flash the Haven 1 / Pi 4 OpenMANET image onto a Haven 2 / Pi 5 node.

### Node names and roles

| Name | Role | Hostname | Password |
|------|------|----------|----------|
| **green** | Gate — internet uplink, DHCP server for the whole mesh | green | havengreen |
| **blue** | Point — mesh extender, gets internet through green | blue | havenblue |

When a user says "green" they mean the gate node. When they say "blue" they mean the point node. These are the standard Haven node names.

### Network architecture

```
Internet → eth0 → [gate br-ahwlan] → bat0 → wlan0 ~~HaLow~~ wlan0 → bat0 → [point br-ahwlan] → WiFi clients
```

Every node has:
- `wlan0` — HaLow 802.11s mesh radio (sub-1GHz)
- `bat0` — BATMAN-adv virtual interface (routes mesh traffic)
- `br-ahwlan` — Linux bridge (everything lives here: bat0, WiFi APs, sometimes eth0)
- One or more WiFi APs (`phy1-ap0`, `phy2-ap0`, etc.) for client devices

### The one rule

**If it's not in `br-ahwlan`, it's not on the mesh.**

The correct data path is: `wlan0 → bat0 → br-ahwlan → WiFi AP → client`

The most common failure is wlan0 going directly into br-ahwlan (bypassing bat0). This happens when the mesh interface has `network='ahwlan'` instead of `network='batmesh0'` (gate) or `network='batmesh'` (point).

### Diagnostic script

Run this on any node for a full health check:
```bash
wget -O- https://raw.githubusercontent.com/buildwithparallel/haven-manet-ip-mesh-radio/main/scripts/node-ops/haven-diag.sh | sh
```

Or if the script is already on the node:
```bash
sh /root/haven-diag.sh
```

### How to SSH into nodes

**Interactive (password at prompt):**

```bash
# Gate (usually reachable from the LAN)
ssh root@<gate-ip>          # password: havengreen

# Point node (via gate as jump host) — from home LAN when you can't reach 10.41.x.x directly
ssh -o ProxyCommand="ssh -W %h:%p root@<gate-ip>" root@<point-mesh-ip>   # password: havenblue

# Heltec node (via gate)
ssh -o ProxyCommand="ssh -W %h:%p root@<gate-ip>" root@<heltec-mesh-ip>  # password: heltec.org
```

**With `sshpass` (scripts, agents, one-liners):** install it first (`apt install sshpass` on many Linux distros; on macOS, use a Homebrew formula that provides `sshpass` or build from source). Use **`-T`** if the environment has no TTY (some automation). Change passwords if you changed them on the node.

```bash
# Gate — from your home LAN
sshpass -p 'changeyourpassword' ssh -T -o StrictHostKeyChecking=no root@<gate-ip> 'hostname; uci get network.ahwlan.ipaddr'

# Point or any 10.41.x.x node — when YOUR laptop is already on the mesh (e.g. green)
sshpass -p 'changeyourpassword' ssh -T -o StrictHostKeyChecking=no root@<point-mesh-ip> 'batctl o'

# Point from home LAN only: jump through gate (two passwords)
sshpass -p 'changeyourpassword' ssh -T -o StrictHostKeyChecking=no \
  -o ProxyCommand="sshpass -p 'changeyourpassword' ssh -T -o StrictHostKeyChecking=no -W %h:%p root@<gate-ip>" \
  root@<point-mesh-ip> 'uci get network.ahwlan.ipaddr'
```

### Finding node IPs

```bash
# On the gate — all DHCP clients (nodes + devices)
cat /tmp/dhcp.leases

# On the gate — OpenMANET nodes only (gate, point)
strings /etc/openmanetd/openmanetd.db

# On any node — its own IP
uci get network.ahwlan.ipaddr
```

### Common problems and fixes

#### 1. wlan0 bypassing bat0 (most common)

**Symptom:** `batctl if` is empty, `ip link show wlan0` says `master br-ahwlan` instead of `master bat0`

**Cause:** Mesh interface has `network='ahwlan'` instead of the batmesh hardif. The most common trigger is running **Quick Config** in LuCI — it regenerates the wireless config and silently resets this value. Never use Quick Config on Haven nodes; use Network → Wireless or UCI directly.

**Diagnose:**
```bash
batctl if                    # should show "wlan0: active"
ip link show wlan0           # should show "master bat0"
uci show wireless | grep "mode='mesh'" | head -1 | cut -d. -f2
# Note the interface name, then:
uci get wireless.<mesh_iface>.network
# If it says 'ahwlan' — that's the problem
```

**Fix:**
```bash
# Identify the mesh interface name
MESH_IFACE=$(uci show wireless | grep "mode='mesh'" | head -1 | cut -d. -f2)

# Gate nodes use 'batmesh0', point nodes use 'batmesh'
# Check which exists:
uci show network | grep batmesh

# Set the correct one (example for gate):
uci set wireless.$MESH_IFACE.network='batmesh0'
uci commit wireless

# If no batmesh hardif exists, create it:
uci set network.batmesh0=interface
uci set network.batmesh0.proto='batadv_hardif'
uci set network.batmesh0.master='bat0'
uci commit network

# Restart
wifi down && service network restart && sleep 3 && wifi up

# Verify
batctl if          # should show: wlan0: active
ip link show bat0  # should show: master br-ahwlan
```

> **Important:** Gate (OpenMANET) uses `batmesh0`. Point nodes use `batmesh`. Always check which one exists with `uci show network | grep batmesh` before setting.

#### 2. bat0 not in bridge

**Symptom:** `brctl show br-ahwlan` doesn't list `bat0`

**Quick fix:**
```bash
ip link set bat0 up
ip link set bat0 master br-ahwlan
```

**Permanent fix:** Ensure `network.ahwlan_dev.ports` includes `bat0`:
```bash
uci set network.ahwlan_dev=device
uci set network.ahwlan_dev.name='br-ahwlan'
uci set network.ahwlan_dev.type='bridge'
uci delete network.ahwlan_dev.ports 2>/dev/null
uci add_list network.ahwlan_dev.ports='bat0'
uci commit network
service network restart
```

#### 3. Anonymous bridge device conflict

**Symptom:** Named `ahwlan_dev` exists with correct ports, but bat0 still isn't in the bridge

**Cause:** OpenWrt auto-creates anonymous `@device[N]` entries that shadow the named device

**Diagnose:**
```bash
uci show network | grep '@device'
# Look for any with name='br-ahwlan'
```

**Fix:**
```bash
i=0
while uci get network.@device[$i] 2>/dev/null; do
    name=$(uci get network.@device[$i].name 2>/dev/null)
    if [ "$name" = "br-ahwlan" ]; then
        uci delete network.@device[$i]
        echo "Deleted anonymous device[$i]"
        continue
    fi
    i=$((i + 1))
done
uci commit network
service network restart
```

#### 4. bat0 is DOWN

**Symptom:** bat0 is in the bridge but `ip link show bat0` says `state DOWN`

**Fix:**
```bash
ip link set bat0 up
```

#### 5. HaLow radio not loading

**Symptom:** `dmesg | grep morse` shows `cmd53_write/read failed (errno=-71)`

**Fix:**
```bash
rmmod morse && sleep 2 && modprobe morse && sleep 5
batctl if   # should now show wlan0: active
```

If that fails, the node needs a full power cycle (unplug from wall, not just reboot).

#### 6. No BATMAN neighbors

**Symptom:** `batctl n` is empty even though `batctl if` shows wlan0 active

**Check:** All nodes must have identical mesh settings:
```bash
iwinfo wlan0 info    # channel, mesh ID
uci show wireless | grep -E "mesh_id|key|channel|mesh_fwding"
```

Required: mesh_id=`haven`, key=`havenmesh`, same channel, mesh_fwding=`0`

### Boot health check

Install `haven-bridge-check.sh` on every node to auto-repair on boot:
```bash
scp scripts/node-ops/haven-bridge-check.sh root@<node-ip>:/etc/init.d/haven-bridge-check
ssh root@<node-ip> "chmod +x /etc/init.d/haven-bridge-check && /etc/init.d/haven-bridge-check enable"
```

### Key files on nodes

| Path | Purpose |
|------|---------|
| `uci show wireless` | Radio and mesh configuration |
| `uci show network` | Bridge, IP, and routing configuration |
| `/tmp/dhcp.leases` | DHCP clients (gate only) |
| `/etc/openmanetd/openmanetd.db` | OpenMANET node database (gate only) |
| `brctl show` | Bridge membership |
| `batctl if` / `batctl n` / `batctl o` | BATMAN interfaces, neighbors, originators |

### Documentation

| Doc | Use it for |
|-----|-----------|
| [docs/reference/finding-nodes.md](docs/reference/finding-nodes.md) | Finding node IPs, accessing LuCI |
| [docs/runbooks/troubleshooting.md](docs/runbooks/troubleshooting.md) | Full troubleshooting checklists |
| [docs/getting-started/setup-guide.md](docs/getting-started/setup-guide.md) | Setting up new nodes |
| [docs/reference/halow-reference.md](docs/reference/halow-reference.md) | Radio specs, MCS tables |
