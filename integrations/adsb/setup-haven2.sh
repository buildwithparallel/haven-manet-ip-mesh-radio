#!/bin/sh
# ADS-B to ATAK on a Haven 2 (Raspberry Pi 5, openwrt-morse-rpi5 image).
#
# Run on the node that has the RTL-SDR plugged in (antenna attached):
#   sh setup-haven2.sh
#
# The image already includes rtl-sdr, dump1090 and Python 3 with cryptography.
# This script adds adsbcot, turns dump1090 on, and runs adsbcot as a service.
# dump1090 decodes the RTL-SDR and writes aircraft.json; adsbcot turns that
# file into Cursor on Target and sends it to ATAK's SA multicast group
# (239.2.3.1:6969). The mesh carries it to every node's Wi-Fi, so any ATAK
# phone on the mesh sees the aircraft with no setup.
#
# The node needs internet (through the gate) for the adsbcot download.
set -e

ADSBCOT_URL=https://github.com/snstac/adsbcot/archive/refs/tags/v9.2.2.tar.gz

if ! lsusb 2>/dev/null | grep -q -i -E 'rtl28|0bda:2838'; then
	echo "No RTL-SDR found on USB. Plug it in (antenna first) and run this again."
	exit 1
fi

# adsbcot is no longer published on PyPI, so "pip3 install adsbcot" fails.
# Install the tagged release from the upstream GitHub repo instead.
if ! command -v adsbcot >/dev/null 2>&1; then
	pip3 install --no-cache-dir "$ADSBCOT_URL"
fi

cat > /etc/adsbcot.ini <<'EOF'
[adsbcot]
# Aircraft list written by dump1090 (write_json in /etc/config/dump1090).
FEED_URL = file:///var/run/dump1090/aircraft.json
POLL_INTERVAL = 2
# ATAK's default SA multicast group. Every ATAK phone on the mesh listens here.
COT_URL = udp+wo://239.2.3.1:6969
EOF

cat > /etc/init.d/adsbcot <<'EOF'
#!/bin/sh /etc/rc.common
# ADS-B to ATAK: reads dump1090's aircraft.json, sends CoT to the mesh.
START=95
STOP=10
USE_PROCD=1

start_service() {
	procd_open_instance
	procd_set_param command /usr/bin/adsbcot -c /etc/adsbcot.ini
	procd_set_param respawn 3600 10 0
	procd_set_param stdout 1
	procd_set_param stderr 1
	procd_close_instance
}
EOF
chmod +x /etc/init.d/adsbcot

# The image's default dump1090 config passes --html-dir, which this dump1090
# (FlightAware 4.0) rejects, so the service exits at once. Clear it.
uci set dump1090.main.disabled='0'
uci set dump1090.main.html_dir=''
uci set dump1090.main.net='1'
uci set dump1090.main.write_json='/var/run/dump1090'
uci set dump1090.main.write_json_every='1'
uci commit dump1090

/etc/init.d/dump1090 enable
/etc/init.d/dump1090 restart
/etc/init.d/adsbcot enable
/etc/init.d/adsbcot restart

sleep 15
echo "dump1090: $(pidof dump1090 >/dev/null && echo running || echo NOT running)"
python3 - <<'EOF'
import json
try:
    a = json.load(open("/var/run/dump1090/aircraft.json"))["aircraft"]
    print("aircraft seen: %d, with a position: %d" % (len(a), len([x for x in a if "lat" in x])))
except Exception as e:
    print("no aircraft file yet:", e)
EOF
echo "Check the feed on the mesh: tcpdump -n -i br-ahwlan -c 3 udp port 6969"
