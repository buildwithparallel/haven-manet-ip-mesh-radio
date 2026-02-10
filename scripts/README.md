# Haven Setup Scripts

Automated setup scripts for configuring Haven mesh nodes from a fresh OpenMANET install.

**Full Haven Guide**: [buildwithparallel.com/products/haven](https://buildwithparallel.com/products/haven) - includes videos, schematics, 3D printable enclosures, Discord community, and direct support.

## Scripts Overview

| Script | Purpose | Run On |
|--------|---------|--------|
| `setup-haven-gate.sh` | Configure gateway node (internet uplink) | First node |
| `setup-haven-point.sh` | Configure extender node | Additional nodes |
| `setup-reticulum.sh` | Install encrypted mesh overlay | Any node (optional) |
| `setup-cot-bridge.sh` | Install ATAK/CivTAK bridge | Any node (optional) |

## Quick Start

### 1. Gateway Node (with internet)

```bash
# Download and edit configuration
curl -sL https://raw.githubusercontent.com/user/haven-manet-ip-mesh-radio/main/scripts/setup-haven-gate.sh -o setup.sh
vi setup.sh    # Edit passwords, channel, etc.
sh setup.sh
reboot
```

### 2. Point Nodes (mesh extenders)

```bash
# Download and edit configuration
curl -sL https://raw.githubusercontent.com/user/haven-manet-ip-mesh-radio/main/scripts/setup-haven-point.sh -o setup.sh
vi setup.sh    # Set unique hostname, IP for each node
sh setup.sh
reboot
```

### 3. (Optional) Add Reticulum Encryption

```bash
curl -sL https://raw.githubusercontent.com/user/haven-manet-ip-mesh-radio/main/scripts/setup-reticulum.sh | sh
/etc/init.d/rnsd enable && /etc/init.d/rnsd start
```

### 4. (Optional) Add ATAK Bridge

```bash
curl -sL https://raw.githubusercontent.com/user/haven-manet-ip-mesh-radio/main/scripts/setup-cot-bridge.sh | sh
/etc/init.d/cot_bridge enable && /etc/init.d/cot_bridge start
```

## Configuration Reference

### Gate Node Defaults

| Setting | Default | Description |
|---------|---------|-------------|
| `HOSTNAME` | green | Node hostname |
| `ROOT_PASSWORD` | green | SSH password |
| `MESH_ID` | haven | Mesh network name |
| `MESH_KEY` | havenmesh | Mesh encryption key |
| `MESH_IP` | 10.41.0.1 | Node IP address |
| `HALOW_CHANNEL` | 28 | HaLow channel (916 MHz) |
| `HALOW_HTMODE` | HT20 | Channel width (2 MHz) |

### Point Node Defaults

| Setting | Default | Description |
|---------|---------|-------------|
| `HOSTNAME` | blue | Node hostname |
| `ROOT_PASSWORD` | blue | SSH password |
| `MESH_IP` | 10.41.0.2 | Node IP (unique per node) |
| `GATEWAY_IP` | 10.41.0.1 | Gate node IP |

### HaLow Channel Selection

| Region | Frequency Range | Example |
|--------|-----------------|---------|
| US/FCC | 902-928 MHz | Channel 28 = 916 MHz |
| EU/ETSI | 863-868 MHz | Region-specific |
| Japan | 920-928 MHz | Region-specific |
| Australia | 915-928 MHz | Region-specific |

### Channel Width vs Range

| Setting | Width | Speed | Range |
|---------|-------|-------|-------|
| HT10 | 1 MHz | ~1.5 Mbps | Maximum |
| HT20 | 2 MHz | ~4 Mbps | Very Long |
| HT40 | 4 MHz | ~15 Mbps | Long |
| HT80 | 8 MHz | ~32 Mbps | Medium |

## After Setup

### Verify Mesh Connectivity
```bash
iwinfo wlan0 info     # HaLow link quality
batctl n              # BATMAN-adv neighbors
ping 10.41.0.1        # Ping gateway
```

### Verify Reticulum (if installed)
```bash
rnstatus              # Shows peers and traffic
```

### Verify ATAK Bridge (if installed)
```bash
tail -f /tmp/bridge.log
```

## Adding More Nodes

For each additional point node:
1. Edit `setup-haven-point.sh` with unique `HOSTNAME` and `MESH_IP`
2. Keep `MESH_ID`, `MESH_KEY`, `HALOW_CHANNEL` the same as gate
3. Run script and reboot

## Troubleshooting

See the [Haven Guide](https://buildwithparallel.com/products/haven) for video tutorials and Discord support.

### Nodes Can't Connect
- Verify `MESH_ID`, `MESH_KEY`, `HALOW_CHANNEL` match exactly on all nodes
- Check HaLow radio: `iwinfo wlan0 info`

### No Internet on Point Nodes
- Verify gateway route: `ip route | grep default`
- Test: `ping 10.41.0.1` then `ping 8.8.8.8`

### Reticulum Issues
- Check status: `rnstatus`
- View logs: `rnsd -v`
