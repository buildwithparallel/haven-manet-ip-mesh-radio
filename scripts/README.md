# Haven Setup Scripts

Automated setup scripts for configuring Haven mesh nodes from a fresh OpenMANET install.

## Quick Start

### Option 1: Download and Run (Recommended)

On the **Gateway** node (first node with internet uplink):
```bash
curl -sL https://raw.githubusercontent.com/YOUR_REPO/haven-manet-ip-mesh-radio/main/scripts/setup-haven-gate.sh | sh
```

On each **Point** node (mesh extenders):
```bash
curl -sL https://raw.githubusercontent.com/YOUR_REPO/haven-manet-ip-mesh-radio/main/scripts/setup-haven-point.sh | sh
```

### Option 2: Copy and Run Manually

1. Copy the appropriate script to the node
2. Edit the configuration section at the top if needed
3. Run: `sh setup-haven-gate.sh` or `sh setup-haven-point.sh`

## What the Scripts Configure

### Haven Gate (Gateway Node)

| Component | Configuration |
|-----------|---------------|
| Hostname | `green` |
| Mesh IP | `10.41.0.1/16` |
| HaLow Radio | Mesh mode, WPA3-SAE |
| 5GHz WiFi | Access point for clients |
| DHCP Server | Serves 10.41.0.100-250 |
| NAT/Firewall | Internet sharing enabled |
| Reticulum | Transport daemon + config |
| CoT Bridge | ATAK multicast relay |

### Haven Point (Extender Node)

| Component | Configuration |
|-----------|---------------|
| Hostname | `blue` |
| Mesh IP | `10.41.0.2/16` |
| Gateway | `10.41.0.1` (Gate node) |
| HaLow Radio | Mesh mode (same settings as Gate) |
| 5GHz WiFi | Access point for clients |
| DHCP | Disabled (Gate handles this) |
| Reticulum | Transport daemon + config |
| CoT Bridge | ATAK multicast relay |

## Configuration Options

Edit the variables at the top of each script before running:

```bash
# Node identity
HOSTNAME="green"          # Change for each node
ROOT_PASSWORD="green"     # Set a secure password

# Mesh network - MUST MATCH ALL NODES
MESH_ID="haven"
MESH_KEY="havenmesh"      # Change this!

# HaLow frequency - MUST MATCH ALL NODES
HALOW_CHANNEL="28"        # See frequency table below
HALOW_HTMODE="HT20"       # HT20=2MHz (longer range)
```

### HaLow Channel Selection

| Region | Frequency Range | Recommended Channels |
|--------|-----------------|----------------------|
| US/FCC | 902-928 MHz | 1-51 (channel 28 = 916 MHz) |
| EU/ETSI | 863-868 MHz | Region-specific |
| Japan | 920-928 MHz | Region-specific |
| Australia | 915-928 MHz | Region-specific |

### Channel Width vs Range

| Width | Speed | Range | Setting |
|-------|-------|-------|---------|
| 1 MHz | ~1.5 Mbps | Maximum | `HALOW_HTMODE="HT10"` |
| 2 MHz | ~4 Mbps | Very Long | `HALOW_HTMODE="HT20"` |
| 4 MHz | ~15 Mbps | Long | `HALOW_HTMODE="HT40"` |
| 8 MHz | ~32 Mbps | Medium | `HALOW_HTMODE="HT80"` |

## After Running the Script

1. **Reboot the node:**
   ```bash
   reboot
   ```

2. **After reboot, start services:**
   ```bash
   /etc/init.d/rnsd enable && /etc/init.d/rnsd start
   /etc/init.d/cot_bridge enable && /etc/init.d/cot_bridge start
   ```

3. **Verify mesh connectivity:**
   ```bash
   # Check HaLow link
   iwinfo wlan0 info

   # Check BATMAN neighbors
   batctl n

   # Check Reticulum
   rnstatus
   ```

## Adding More Point Nodes

For each additional point node:

1. Edit `setup-haven-point.sh`:
   - Change `HOSTNAME` (e.g., "node3", "node4")
   - Change `MESH_IP` (e.g., "10.41.0.3", "10.41.0.4")
   - Change `WIFI_5GHZ_SSID` to be unique

2. Run the script and reboot

All nodes will automatically discover each other via BATMAN-adv mesh routing.

## Troubleshooting

### Nodes Can't See Each Other
- Verify `MESH_ID`, `MESH_KEY`, `HALOW_CHANNEL`, and `HALOW_HTMODE` match exactly
- Check HaLow radio is enabled: `iwinfo wlan0 info`
- Verify BATMAN neighbors: `batctl n`

### No Internet on Point Nodes
- Verify gateway is set: `ip route | grep default`
- Check NAT on Gate: `nft list ruleset | grep masquerade`
- Test connectivity: `ping 10.41.0.1` then `ping 8.8.8.8`

### Reticulum Not Connecting
- Check status: `rnstatus`
- Verify bridge interface: `ip link show br-ahwlan`
- Check logs: `rnsd -v`

### ATAK Not Working
- Verify bridge is running: `ps | grep cot_bridge`
- Check logs: `tail -f /tmp/bridge.log`
- Ensure ATAK is using default multicast (no custom outputs)
