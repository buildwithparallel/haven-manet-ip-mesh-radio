#!/bin/sh
# Serve the SkyAware aircraft map over HTTPS with a real certificate, so car
# browsers (Tesla) and phones can open it by name on the mesh, with no internet.
#
#   sh install.sh <name> [address]
#
#   <name>     the map's name, e.g. adsb.example.com (the certificate's name)
#   [address]  an extra address for the map in 100.64.0.0/10, default 100.64.41.6.
#              Use a different one on each Haven that serves a map.
#
# Put fullchain.pem and privkey.pem for <name> next to this script first (see
# README.md for getting them from Let's Encrypt with a DNS check).
#
# What it sets up:
# - /usr/bin/adsb-https.py: HTTPS on port 8443 for /usr/share/dump1090 (Python
#   and OpenSSL with TLS 1.3; uhttpd's mbedtls build stalls on some Android and
#   car browsers), sending Cache-Control: no-cache
# - <address> on the mesh interface at every boot: the Tesla browser refuses
#   private 10.x addresses, even by name, but accepts 100.64.0.0/10
# - the Haven's DNS answers <name> with <address>, so it works offline; phones
#   and cars must use this Haven's DNS (a point serving its own DHCP pool does)
set -e

NAME=$1
ADDR=${2:-100.64.41.6}
HERE=$(cd "$(dirname "$0")" && pwd)
[ -n "$NAME" ] || { echo "Usage: sh install.sh <name> [address]"; exit 1; }
for f in fullchain.pem privkey.pem adsb-https.py; do
	[ -f "$HERE/$f" ] || { echo "Missing $HERE/$f"; exit 1; }
done
case "$ADDR" in 100.6[4-9].*|100.[7-9][0-9].*|100.1[01][0-9].*|100.12[0-7].*) ;; *)
	echo "Address must be in 100.64.0.0/10 (100.64.x.x to 100.127.x.x)."; exit 1;;
esac
[ -d /usr/share/dump1090 ] || { echo "No /usr/share/dump1090: set up ADS-B first (setup-haven2.sh)."; exit 1; }

# Certificate and key, kept across a firmware upgrade that keeps settings.
cp "$HERE/fullchain.pem" /etc/adsb.crt
umask 077; cp "$HERE/privkey.pem" /etc/adsb.key; umask 022
for f in /etc/adsb.crt /etc/adsb.key; do grep -qx "$f" /etc/sysupgrade.conf || echo "$f" >> /etc/sysupgrade.conf; done

# HTTPS server as a service.
cp "$HERE/adsb-https.py" /usr/bin/adsb-https.py
chmod +x /usr/bin/adsb-https.py
cat > /etc/init.d/adsb-https <<'EOF'
#!/bin/sh /etc/rc.common
# HTTPS for the SkyAware map (python + OpenSSL, TLS 1.3). See /usr/bin/adsb-https.py.
START=96
STOP=10
USE_PROCD=1
start_service() {
	procd_open_instance
	procd_set_param command /usr/bin/python3 /usr/bin/adsb-https.py
	procd_set_param respawn 3600 5 0
	procd_set_param stderr 1
	procd_close_instance
}
EOF
chmod +x /etc/init.d/adsb-https

# If uhttpd was also set up for HTTPS on this port, give the port back.
if uci -q get uhttpd.dump1090.listen_https >/dev/null; then
	uci -q delete uhttpd.dump1090.listen_https
	uci -q delete uhttpd.dump1090.cert
	uci -q delete uhttpd.dump1090.key
fi
# uhttpd's DNS-rebinding check answers Forbidden to a 10.x client asking for a
# 100.64 address; on your own mesh it isn't needed.
uci -q get uhttpd.dump1090 >/dev/null && uci set uhttpd.dump1090.rfc1918_filter=0
uci commit uhttpd
/etc/init.d/uhttpd restart

# The extra address, now and whenever the mesh interface comes up.
cat > /etc/hotplug.d/iface/90-adsb-address <<EOF
#!/bin/sh
# Extra address for the ADS-B map (car browsers refuse private 10.x addresses).
[ "\$INTERFACE" = ahwlan ] && [ "\$ACTION" = ifup ] && ip addr add $ADDR/32 dev br-ahwlan 2>/dev/null
exit 0
EOF
chmod +x /etc/hotplug.d/iface/90-adsb-address
rm -f /etc/hotplug.d/iface/90-adsb-tesla-address
ip addr add "$ADDR/32" dev br-ahwlan 2>/dev/null || true

# The Haven's DNS answers the name with that address.
for a in $(uci -q get dhcp.@dnsmasq[0].address); do
	case "$a" in "/$NAME/"*) uci del_list dhcp.@dnsmasq[0].address="$a";; esac
done
uci add_list dhcp.@dnsmasq[0].address="/$NAME/$ADDR"
uci commit dhcp
/etc/init.d/dnsmasq restart

/etc/init.d/adsb-https enable
/etc/init.d/adsb-https restart
sleep 3
echo "HTTPS map: https://$NAME:8443  ($NAME -> $ADDR on this Haven's DNS)"
netstat -tln | grep -q ":8443 " && echo "Listening on 8443." || echo "Not listening on 8443: check logread."
