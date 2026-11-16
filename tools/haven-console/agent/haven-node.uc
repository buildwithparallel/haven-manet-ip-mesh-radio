#!/usr/bin/ucode
// Haven Console node agent.
//
// Installed on every node as /www/cgi-bin/haven-node. The gate's console reads
// it over the mesh to learn this node's HaLow peers, batman-adv routes, and the
// Wi-Fi clients on its access points. Adapted from the field performance
// benchmark in field-mesh-network-design.
//
// It never prints a Wi-Fi key or the mesh key. Request text is never passed to
// a shell command. Keep the answer small: a few KB did not make it back across
// a Heltec hop in field testing.
'use strict';

import { connect } from 'ubus';
import { cursor } from 'uci';
import { open, popen } from 'fs';

const MESH = 'bat0';

function sh(cmd) {
	let p = popen(cmd, 'r');
	if (!p)
		return '';
	let out = p.read('all');
	p.close();
	return out ?? '';
}

function sh_json(cmd) {
	let out = trim(sh(cmd));
	if (!length(out))
		return null;
	try {
		return json(out);
	}
	catch (e) {
		return null;
	}
}

function lower(v) {
	return v != null ? lc(v) : null;
}

function mbps(kbps) {
	return kbps != null ? int(kbps / 100 + 0.5) / 10.0 : null;
}

// OpenMANET names the HaLow mesh radio wlh0. Heltec firmware names it wlan0.
// A node can have several access points (2.4 GHz and 5 GHz).
function ifaces() {
	let cur = null, mesh = null, aps = [], info = {};
	for (let line in split(sh('iw dev'), '\n')) {
		let named = match(line, /Interface ([A-Za-z0-9._-]+)/);
		if (named) {
			cur = named[1];
			info[cur] = { ssid: null, mhz: null, ch: null, width: null };
		}
		if (!cur)
			continue;
		if (match(line, /type mesh point/))
			mesh = cur;
		if (match(line, /type AP/))
			push(aps, cur);
		let ssid = match(line, /^\s+ssid (.+)$/);
		if (ssid)
			info[cur].ssid = ssid[1];
		let ch = match(line, /channel ([0-9]+) \(([0-9]+) MHz\)/);
		if (ch) {
			info[cur].ch = +ch[1];
			info[cur].mhz = +ch[2];
		}
		let width = match(line, /width: ([0-9]+) MHz/);
		if (width)
			info[cur].width = +width[1];
	}
	return { mesh, aps, info };
}

// The HaLow channel as the radio really runs it:
// [center MHz, operating width MHz, primary width MHz, channel number].
// iw reports Morse radios on a stand-in 5 GHz channel, so ask morse_cli,
// and take the channel number from the s1g radio in UCI.
function halow_channel(dev) {
	let out = sh('morse_cli -i ' + dev + ' channel 2>/dev/null');
	let f = match(out, /Operating Frequency: ([0-9]+) kHz/);
	if (!f)
		return null;
	let bw = match(out, /Operating BW: ([0-9]+) MHz/);
	let pbw = match(out, /Primary BW: ([0-9]+) MHz/);
	let ch = null;
	cursor().foreach('wireless', 'wifi-device', (s) => {
		if (s.band == 's1g' && s.channel != null)
			ch = +s.channel;
	});
	return [int(+f[1] / 100 + 0.5) / 10.0, bw ? +bw[1] : null, pbw ? +pbw[1] : null, ch];
}

function mesh_ip() {
	// The bridge address is the truth. OpenMANET can move a point's address
	// away from what UCI says.
	// Prefer br-ahwlan: a Pi 5 point can keep its setup LAN at 10.41.254.1/24.
	let out = trim(sh("{ ip -4 -o addr show dev br-ahwlan; ip -4 -o addr show; } 2>/dev/null | awk '$4 ~ /^10\\.41\\./ {print $4; exit}'"));
	if (length(out))
		return split(out, '/')[0];
	let uci = cursor();
	return uci.get('network', 'ahwlan', 'ipaddr') ?? uci.get('network', 'lan', 'ipaddr');
}

function txpower(dev) {
	let m = match(sh('iw dev ' + dev + ' info 2>/dev/null'), /txpower ([0-9.]+) dBm/);
	return m ? +m[1] : null;
}

function emit(obj) {
	// uhttpd drops a CGI body whose headers are not terminated, and a short
	// Content-Length makes the reader wait forever, so count the bytes.
	let body = sprintf('%J\n', obj);
	let pid = trim(sh('echo $PPID'));
	let n = null;
	if (match(pid, /^[0-9]+$/)) {
		let path = '/tmp/haven-node-out-' + pid;
		let wr = popen('cat > ' + path, 'w');
		if (wr) {
			wr.write(body);
			wr.close();
		}
		n = trim(sh('wc -c < ' + path));
		sh('rm -f ' + path);
	}
	printf('Content-Type: application/json\r\n');
	printf('Cache-Control: no-store\r\n');
	printf('Connection: close\r\n');
	if (match(n, /^[0-9]+$/))
		printf('Content-Length: %s\r\n', n);
	printf('\r\n');
	printf('%s', body);
}

function snapshot() {
	let bus = connect();
	let found = ifaces();
	let halow = found.mesh || 'wlh0';
	let board = bus.call('system', 'board') ?? {};
	let gw_mode = cursor().get('network', MESH, 'gw_mode');

	// HaLow peers: signal, both rates, 802.11s link state.
	let peers = [];
	for (let p in (bus.call('iwinfo', 'assoclist', { device: halow })?.results ?? [])) {
		push(peers, [lower(p.mac), p.signal ?? null, mbps(p.tx?.rate), mbps(p.rx?.rate), p['mesh plink'] ?? null]);
	}

	// One selected route per originator: [originator, next hop, throughput Mb/s].
	let routes = [];
	for (let o in (sh_json(`batctl meshif ${MESH} originators_json 2>/dev/null`) ?? [])) {
		if (!o.best)
			continue;
		let tp = o.throughput != null ? int(o.throughput / 100 + 0.5) / 10.0 : null;
		push(routes, [lower(o.orig_address), lower(o.neigh_address), tp ?? o.tq ?? null]);
	}

	let gateway = null;
	for (let g in (sh_json(`batctl meshif ${MESH} gateways_json 2>/dev/null`) ?? [])) {
		if (g.best)
			gateway = lower(g.orig_address);
	}

	// Access points: [ssid, MHz, channel, width MHz]. Wi-Fi clients: [mac, signal,
	// index into aps]. The console uses these to find which node and network
	// a phone joined. Only the network name is sent, never its key.
	let aplist = [], stations = [];
	for (let i = 0; i < length(found.aps); i++) {
		let ap = found.aps[i], inf = found.info[ap];
		push(aplist, [inf.ssid, inf.mhz, inf.ch, inf.width]);
		for (let s in (bus.call('iwinfo', 'assoclist', { device: ap })?.results ?? [])) {
			push(stations, [lower(s.mac), s.signal ?? null, i]);
		}
	}

	return {
		k: 'node',
		host: board.hostname ?? null,
		model: board.model ?? null,
		ip: mesh_ip(),
		role: gw_mode == 'server' ? 'gate' : 'point',
		mac: lower(trim(sh('cat /sys/class/net/' + halow + '/address 2>/dev/null'))),
		dev: halow,
		txp: txpower(halow),
		gw: gateway,
		peers: peers,
		routes: routes,
		halow: halow_channel(halow),
		aps: aplist,
		sta: stations,
	};
}

try {
	emit(snapshot());
}
catch (e) {
	emit({ k: 'error', error: sprintf('%s', e) });
}
