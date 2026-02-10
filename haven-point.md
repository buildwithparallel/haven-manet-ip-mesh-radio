# Haven Point (Blue) - Mesh Node

The **Haven Point** is a mesh extender node that connects to the Haven Gate over the HaLow mesh backhaul and provides local WiFi access to clients. It relies on the Gate node for internet connectivity and DHCP.

## Overview

| Property | Value |
|----------|-------|
| Hostname | blue |
| Role | Mesh Extender / Access Point |
| Mesh IP | 10.41.73.196 |
| Gateway | 10.41.0.1 (Haven Gate) |
| SSH | root / blue |

## Network Architecture

```
                    Internet
                        │
                        ▼
                [Haven Gate (GREEN)]
                   10.41.0.1
                        │
                        │ HaLow 916 MHz Mesh
                        ▼
┌───────────────────────────────────────┐
│           Haven Point (BLUE)          │
│                                       │
│  br-ahwlan: 10.41.73.196/16          │
│    ├── bat0 (BATMAN-adv)             │
│    ├── wlan0 (HaLow 916MHz mesh)     │
│    └── phy1-ap0 (5GHz AP)            │
└───────────────────────────────────────┘
                        │
                        ▼ 5GHz WiFi
                   [Clients]
```

## Radio Configuration

### HaLow Mesh Radio (802.11ah)
The primary backhaul radio connecting to other Haven nodes.

| Property | Value |
|----------|-------|
| Interface | wlan0 |
| Driver | morse (Morse Micro) |
| Hardware | Morse Micro SPI-MM601X |
| Frequency | 916 MHz (Channel 28) |
| Mode | Mesh Point |
| Mesh ID | haven |
| Encryption | WPA3 SAE (CCMP) |
| Key | havenmesh |
| Beacon Interval | 1000ms |

```bash
# Check HaLow link quality
iwinfo wlan0 info

# Expected output:
# Signal: -18 dBm (excellent)
# Link Quality: 70/70
# Bit Rate: 32.5 MBit/s
```

### 5GHz Access Point
Client access point for local devices.

| Property | Value |
|----------|-------|
| Interface | phy1-ap0 |
| Hardware | Cypress CYW43455 |
| Frequency | 5.180 GHz (Channel 36) |
| Mode | Access Point |
| SSID | blue-5ghz |
| Encryption | WPA2 PSK |
| Key | blue-5ghz |
| HT Mode | VHT80 |

## Network Configuration

### Bridge Interface (br-ahwlan)
All interfaces bridged for Layer 2 connectivity.

```
network.ahwlan=interface
network.ahwlan.proto='static'
network.ahwlan.device='br-ahwlan'
network.ahwlan.ipaddr='10.41.73.196'
network.ahwlan.netmask='255.255.0.0'
network.ahwlan.gateway='10.41.0.1'
network.ahwlan.dns='8.8.8.8 8.8.4.4'
```

### Important: Default Gateway
Point nodes must have their gateway set to the Gate node for internet access:

```bash
uci set network.ahwlan.gateway="10.41.0.1"
uci commit network
/etc/init.d/network reload
```

Traffic flow: `Clients → Blue → HaLow Mesh → Green → Internet`

## Services

### Reticulum
Reticulum network stack for encrypted mesh communication.

- Config: `~/.reticulum/config`
- Service: `/etc/init.d/rnsd`

Configuration:
```ini
[reticulum]
  share_instance = Yes
  enable_transport = Yes
  instance_control_port = 37428

[interfaces]
  [[HaLow Mesh Bridge]]
    type = AutoInterface
    enabled = Yes
    devices = br-ahwlan
    group_id = reticulum

  [[UDP Broadcast]]
    type = UDPInterface
    enabled = Yes
    listen_ip = 10.41.73.196
    listen_port = 4242
    forward_ip = 10.41.255.255
    forward_port = 4242
```

### ATAK Bridge
CoT bridge for ATAK integration.

- Script: `/root/cot_bridge_multicast.py`
- Listens: Multicast 239.2.3.1:6969

## Management

### SSH Access
Point nodes don't have external IPs. Access via the Gate node:

```bash
# From your computer (via Gate as jump host)
ssh -o ProxyCommand="ssh -W %h:%p root@192.168.0.21" root@10.41.73.196
# Password: blue

# Or from the Gate node directly
ssh root@10.41.73.196
```

### Useful Commands
```bash
# Check mesh connectivity
ping 10.41.0.1              # Ping gate
iwinfo wlan0 info           # HaLow link quality

# Check Reticulum
rnstatus

# Check routing
ip route

# View bridge log
tail -f /tmp/bridge.log
```

## Troubleshooting

### No Internet Connectivity
1. Check gateway is set:
   ```bash
   ip route | grep default
   # Should show: default via 10.41.0.1
   ```
2. If missing, add it:
   ```bash
   uci set network.ahwlan.gateway="10.41.0.1"
   uci commit network
   /etc/init.d/network reload
   ```
3. Verify DNS:
   ```bash
   uci set network.ahwlan.dns="8.8.8.8 8.8.4.4"
   uci commit network
   ```

### Cannot Reach Gate Node
1. Check HaLow mesh is connected:
   ```bash
   iwinfo wlan0 info | grep -E "Signal|Quality"
   ```
2. Check BATMAN neighbors:
   ```bash
   batctl n
   ```
3. Verify mesh credentials match Gate node

### Clients Get "Connected, No Internet"
1. Android/iOS check connectivity via captive portal detection
2. Usually a DNS issue - ensure DNS is configured:
   ```bash
   uci show network.ahwlan.dns
   ```
