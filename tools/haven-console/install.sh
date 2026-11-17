#!/usr/bin/env bash
# Install Haven Console on a Haven mesh, from a computer that can SSH to the gate.
#
#   ./install.sh root@<gate> [<point mesh IP> ...]          install
#   ./install.sh --remove root@<gate> [<point mesh IP> ...] remove
#
# The gate gets the console (/usr/bin/haven-console, port 8443) and the node
# agent. Every point gets the node agent only (/www/cgi-bin/haven-node). Any
# node with an RTL-SDR plugged in also gets the SDR agent (/usr/bin/haven-sdr,
# port 8090) for the SDR tab; plug the SDR in before running this.
# Points are reached by jumping through the gate, so this computer does not
# need to be on the mesh. Each node asks for its root password unless you use
# SSH keys.
#
# Nothing is installed with opkg, and no UCI, radio, firewall or key setting
# is changed.
set -euo pipefail

cd "$(dirname "$0")"
REMOVE=0
if [[ "${1:-}" == "--remove" ]]; then REMOVE=1; shift; fi
if [[ $# -lt 1 ]]; then
	sed -n '2,13p' "$0" | sed 's/^# \{0,1\}//'
	exit 1
fi
GATE="$1"; shift
POINTS=("$@")
BIN=build/haven-console-linux-arm64
SDR_BIN=build/haven-sdr-linux-arm64
SSH_OPTS=(-o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new)

gate() { ssh "${SSH_OPTS[@]}" "$GATE" "$@"; }
point() { local ip="$1"; shift; ssh "${SSH_OPTS[@]}" -o ProxyCommand="ssh ${SSH_OPTS[*]} -W %h:%p $GATE" "root@$ip" "$@"; }

if [[ $REMOVE == 1 ]]; then
	echo "Removing from the gate ($GATE)"
	RM_SDR='/etc/init.d/haven-sdr stop 2>/dev/null; /etc/init.d/haven-sdr disable 2>/dev/null; rm -f /etc/init.d/haven-sdr /usr/bin/haven-sdr'
	gate "/etc/init.d/haven-console stop 2>/dev/null; /etc/init.d/haven-console disable 2>/dev/null; rm -f /etc/init.d/haven-console /usr/bin/haven-console /www/cgi-bin/haven-node; rm -rf /etc/haven-console; $RM_SDR"
	for ip in "${POINTS[@]}"; do
		echo "Removing from $ip"
		point "$ip" "rm -f /www/cgi-bin/haven-node; $RM_SDR"
	done
	echo "Removed."
	exit 0
fi

if [[ ! -f "$BIN" ]]; then
	command -v go >/dev/null || { echo "Missing $BIN and Go is not installed to build it." >&2; exit 1; }
	echo "Building $BIN"
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$BIN" .
fi
if [[ ! -f "$SDR_BIN" ]]; then
	command -v go >/dev/null || { echo "Missing $SDR_BIN and Go is not installed to build it." >&2; exit 1; }
	echo "Building $SDR_BIN"
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$SDR_BIN" ./sdr-agent
fi

# Adds the SDR agent to a node that has an RTL-SDR and rtl_tcp. $1 runs a
# command on that node (gate or point <ip>).
HAS_SDR='lsusb 2>/dev/null | grep -q -i -E "0bda:283[28]|rtl28" && command -v rtl_tcp >/dev/null'
sdr_agent() {
	if "$@" "$HAS_SDR"; then
		echo "  RTL-SDR found: SDR agent"
		"$@" 'cat > /usr/bin/haven-sdr.new && chmod 755 /usr/bin/haven-sdr.new && mv /usr/bin/haven-sdr.new /usr/bin/haven-sdr && sync' < "$SDR_BIN"
		"$@" 'cat > /etc/init.d/haven-sdr && chmod 755 /etc/init.d/haven-sdr && /etc/init.d/haven-sdr enable && /etc/init.d/haven-sdr restart && sync' < init.d-haven-sdr
	fi
}

arch=$(gate 'uname -m')
if [[ "$arch" != "aarch64" ]]; then
	echo "The gate is $arch. The console is built for aarch64 (Raspberry Pi 4 or 5)." >&2
	exit 1
fi

echo "Gate ($GATE): agent and console"
gate 'cat > /www/cgi-bin/haven-node && chmod 755 /www/cgi-bin/haven-node && sync' < agent/haven-node.uc
gate 'cat > /usr/bin/haven-console.new && chmod 755 /usr/bin/haven-console.new && mv /usr/bin/haven-console.new /usr/bin/haven-console && sync' < "$BIN"
gate 'cat > /etc/init.d/haven-console && chmod 755 /etc/init.d/haven-console && /etc/init.d/haven-console enable && sync' < init.d-haven-console
sdr_agent gate

for ip in "${POINTS[@]}"; do
	echo "Point $ip: agent"
	point "$ip" 'cat > /www/cgi-bin/haven-node && chmod 755 /www/cgi-bin/haven-node && sync' < agent/haven-node.uc
	sdr_agent point "$ip"
done

# Start the console last: an address that does not answer as a node is
# skipped for 10 minutes, so it must not probe points before their agents exist.
echo "Starting the console"
gate '/etc/init.d/haven-console restart'

echo "Checking"
sleep 4
gate 'wget -q -T 3 -O - http://127.0.0.1/cgi-bin/haven-node | grep -q "\"k\": \"node\"" && echo "  gate agent: ok" || echo "  gate agent: NO ANSWER"'
for ip in "${POINTS[@]}"; do
	gate "wget -q -T 3 -O - http://$ip/cgi-bin/haven-node | grep -q '\"k\": \"node\"' && echo '  $ip agent: ok' || echo '  $ip agent: NO ANSWER'"
done
gate 'wget -q --no-check-certificate -T 3 -O - https://127.0.0.1:8443/api/mesh >/dev/null && echo "  console: ok" || echo "  console: NO ANSWER (logread -e haven-console)"'
mesh_ip=$(gate "ip -4 -o addr show br-ahwlan 2>/dev/null | awk '{print \$4}' | cut -d/ -f1")
echo
echo "Open https://${mesh_ip:-<gate mesh IP>}:8443 from a phone on the mesh Wi-Fi."
