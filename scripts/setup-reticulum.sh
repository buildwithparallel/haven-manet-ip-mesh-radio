#!/bin/sh
#
# Reticulum Setup Script
# Installs and configures Reticulum on a Haven node
#
# Prerequisites: Run setup-haven-gate.sh or setup-haven-point.sh first
# Usage: sh setup-reticulum.sh
#

set -e

echo "═══════════════════════════════════════════════════════════════════"
echo "  Reticulum Setup"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

if [ ! -f /etc/openwrt_release ]; then
    echo "ERROR: This script must be run on OpenWrt/OpenMANET"
    exit 1
fi

# Check if bridge exists
if ! ip link show br-ahwlan >/dev/null 2>&1; then
    echo "WARNING: br-ahwlan not found. Run setup-haven-gate.sh or setup-haven-point.sh first."
    echo "Continuing anyway..."
fi

echo "[1/3] Installing Python and Reticulum..."
opkg update
opkg install python3 python3-pip 2>/dev/null || echo "Python may already be installed"

# Try different pip install methods
if pip3 install rns 2>/dev/null; then
    echo "  Reticulum installed via pip3"
elif pip3 install --break-system-packages rns 2>/dev/null; then
    echo "  Reticulum installed via pip3 (break-system-packages)"
else
    echo "  Reticulum may already be installed"
fi

echo "[2/3] Creating Reticulum configuration..."
mkdir -p /root/.reticulum

cat > /root/.reticulum/config << 'EOF'
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
    listen_ip = 0.0.0.0
    listen_port = 4242
    forward_ip = 10.41.255.255
    forward_port = 4242
EOF

echo "[3/3] Creating Reticulum service..."
cat > /etc/init.d/rnsd << 'EOF'
#!/bin/sh /etc/rc.common
START=99
STOP=10
USE_PROCD=1

start_service() {
    procd_open_instance
    procd_set_param command /usr/bin/rnsd
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
EOF
chmod +x /etc/init.d/rnsd

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  Reticulum Setup Complete!"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "  Config:  /root/.reticulum/config"
echo "  Service: /etc/init.d/rnsd"
echo ""
echo "  Start Reticulum:"
echo "    /etc/init.d/rnsd enable"
echo "    /etc/init.d/rnsd start"
echo ""
echo "  Check status:"
echo "    rnstatus"
echo ""
echo "  Optional: Install ATAK bridge"
echo "    sh setup-cot-bridge.sh"
echo ""
echo "═══════════════════════════════════════════════════════════════════"
