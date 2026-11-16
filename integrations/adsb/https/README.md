# The aircraft map on a Tesla screen (HTTPS by name)

Open the Haven's live aircraft map (SkyAware) in a Tesla's browser, or any browser, over the Haven's own Wi-Fi with no internet: `https://adsb.example.com:8443`.

Tested on Nov 16, 2026: a Haven 2 point (Pi 5 + MM8108) serving the map to a Tesla over its Wi-Fi.

## Why it takes more than the plain map

- **The Tesla browser only loads `https://` pages,** so the map needs a real certificate for a name you own. A self-signed certificate won't do.
- **The Tesla browser refuses private addresses** (`10.x`, `192.168.x`), by IP or by a name that points to one, with `ERR_CONNECTION_REFUSED`. It does accept the `100.64.0.0/10` range. So the Haven also answers on an address like `100.64.41.6`, and its own DNS points the name there.
- **uhttpd's TLS (mbedtls, TLS 1.2 only) stalls on some Android Chrome handshakes,** so HTTPS is served by a small Python server using OpenSSL with TLS 1.3. Plain HTTP stays on uhttpd at `:8080`.

The car (or phone) has to use the Haven's DNS. A Haven point that serves its own DHCP pool hands out itself as DNS (see `scripts/node-setup/setup-haven-point.sh`).

## Before you start

- ADS-B and the SkyAware map running on the Haven ([`setup-haven2.sh`](../setup-haven2.sh)).
- A domain you control, where you can add a DNS TXT record.
- [certbot](https://certbot.eff.org/) on a computer (`brew install certbot` on a Mac).

## 1. Get a certificate (on your computer)

Let's Encrypt checks a DNS record instead of reaching the Haven, so the Haven never has to be on the internet:

```bash
sudo certbot certonly --manual --preferred-challenges dns -d adsb.example.com --agree-tos --register-unsafely-without-email
```

certbot prints a TXT record. Add it at your DNS provider (name `_acme-challenge.adsb`, value as printed), wait a minute, then press Enter. Copy the files somewhere readable:

```bash
mkdir -p ~/adsb-cert && sudo cp -L /etc/letsencrypt/live/adsb.example.com/{fullchain,privkey}.pem ~/adsb-cert/ && sudo chown $USER ~/adsb-cert/*.pem
```

The certificate lasts 90 days. Renew by running the same command again and repeating step 2.

## 2. Install on the Haven

Put `fullchain.pem` and `privkey.pem` next to `install.sh` and `adsb-https.py`, copy the folder to the Haven, and run:

```bash
scp -O -r https root@<haven-ip>:/tmp/
ssh root@<haven-ip> sh /tmp/https/install.sh adsb.example.com 100.64.41.6
```

Use a different `100.64.x.x` address on each Haven that serves a map. The installer sets up the HTTPS server (port 8443, starts at boot), adds the address at every boot, points the name at it in the Haven's DNS, and keeps the certificate across a firmware upgrade that keeps settings.

## 3. Open it in the car

Join the Haven's Wi-Fi from the car (Controls → Wi-Fi), then open `https://adsb.example.com:8443`, typed with `https://` and the port. Tesla's browser only works in Park.

You don't need a public DNS record for the name. If a device ever looks the name up somewhere else, you can add a public A record pointing to the same `100.64.x.x` address; it only works on the mesh either way.
