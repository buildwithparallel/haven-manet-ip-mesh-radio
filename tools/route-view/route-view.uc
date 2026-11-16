// Haven route view: how traffic from this node reaches another node, live.
// Made to be filmed or shown on a screen. Redraws every second from
// batman-adv's own tables. Start it with route-view.sh, which restores the
// terminal when you press Ctrl-C.
//
//   ucode route-view.uc <node name or mesh MAC>
//   ONCE=1 ucode route-view.uc blue      draw one frame without screen control
import { popen, readfile } from 'fs';

const SILENT_MS = 3000;     // a node not heard for this long counts as silent
const DEST_ARG = ARGV[0];
const ONCE = getenv('ONCE');

const E = '\u001b[';
const RESET = E + '0m', BOLD = E + '1m', DIM = E + '2m';
const GREEN = E + '1;32m', YELLOW = E + '1;33m', RED = E + '1;31m', CYAN = E + '1;36m';
const BG_GREEN = E + '1;30;42m', BG_YELLOW = E + '1;30;43m', BG_RED = E + '1;97;41m';

function rep(s, n) { let o = ''; for (let i = 0; i < n; i++) o += s; return o; }
function center(s, w) {
	let l = length(s);
	if (l >= w) return substr(s, 0, w);
	let left = int((w - l) / 2);
	return rep(' ', left) + s + rep(' ', w - l - left);
}
function pad(s, w) { s = '' + s; return length(s) >= w ? s : s + rep(' ', w - length(s)); }
function run(cmd) { let p = popen(cmd); if (!p) return ''; let o = p.read('all'); p.close(); return o ?? ''; }
function now_ms() { let c = clock(true); return c[0] * 1000 + int(c[1] / 1000000); }
function hhmmss() { let t = localtime(); return sprintf('%02d:%02d:%02d', t.hour, t.min, t.sec); }

// ---------- names ----------
// From alfred's /etc/bat-hosts ("green_wlh0" becomes "green"), plus
// /etc/haven-names ("<mac> <name>" per line) for nodes alfred does not name,
// such as Heltec dongles.
let names = {};
function load_names() {
	names = {};
	for (let file in ['/etc/bat-hosts', '/etc/haven-names'])
		for (let line in split(readfile(file) ?? '', '\n')) {
			let m = match(line, /^([0-9a-f:]{17})[ \t]+([^ \t]+)/);
			if (!m) continue;
			let n = file == '/etc/bat-hosts' ? match(m[2], /^(.+)_[^_]+$/) : null;
			names[m[1]] = n ? n[1] : m[2];
		}
}
function name(mac) { return names[mac] ?? mac; }

const GATE_NAME = trim(readfile('/proc/sys/kernel/hostname') ?? 'this node');
let mesh_if = match(run('batctl if'), /^([^:\s]+):/);
const GATE_MAC = trim(readfile('/sys/class/net/' + (mesh_if ? mesh_if[1] : 'wlh0') + '/address') ?? '');

// ---------- terminal size ----------
// OpenWrt has no stty or tput. BusyBox resize asks the terminal itself.
let ROWS = 40;
function read_size() {
	let r = match(run('resize 2>/dev/tty'), /LINES=([0-9]+)/);
	if (r && +r[1] > 5) ROWS = +r[1];
}

// ---------- state ----------
let seen = {};           // every originator ever in the table
let silent_since = {};   // mac -> ms when it went silent
let events = [];
let last_key = null, last_path = null;

function log(msg) {
	push(events, hhmmss() + '  ' + msg);
	if (length(events) > 12) shift(events);
}

function trace(dest) {
	let hops = [GATE_MAC];
	for (let l in split(run('batctl tr -n ' + dest + ' 2>/dev/null'), '\n')) {
		if (match(l, /^ *[0-9]+: +\*/)) return null;
		let m = match(l, /^ *[0-9]+: +([0-9a-f:]{17})/);
		if (m) push(hops, m[1]);
	}
	return length(hops) > 1 && hops[length(hops) - 1] == dest ? hops : null;
}

function describe(path) {
	if (!path) return 'no route';
	if (length(path) == 2) return 'direct';
	let relays = [];
	for (let i = 1; i < length(path) - 1; i++) push(relays, name(path[i]));
	return 'through ' + join(', ', relays);
}

// ---------- drawing ----------
const BOX = 10;   // inner width; a box is BOX + 2 wide

function draw_boxes(items, gap, arrows, indent) {
	let rows = [indent, indent, indent, indent];
	for (let i = 0; i < length(items); i++) {
		let it = items[i], between = i < length(items) - 1;
		rows[0] += it.color + '┌' + rep('─', BOX) + '┐' + RESET + (between ? rep(' ', gap) : '');
		rows[1] += it.color + '│' + center(uc(it.label), BOX) + '│' + RESET + (between ? arrows[i] : '');
		rows[2] += it.color + '│' + center(it.sub, BOX) + '│' + RESET + (between ? rep(' ', gap) : '');
		rows[3] += it.color + '└' + rep('─', BOX) + '┘' + RESET + (between ? rep(' ', gap) : '');
	}
	return rows;
}

function frame() {
	let now = now_ms();
	let origs = [];
	try { origs = json(run('batctl oj') || '[]'); } catch (e) { origs = []; }

	let by = {};
	for (let o in origs) {
		let m = o.orig_address;
		if (!by[m]) by[m] = { best: null, all: [], last: null };
		push(by[m].all, o);
		if (o.best) by[m].best = o;
		if (by[m].last == null || o.last_seen_msecs < by[m].last) by[m].last = o.last_seen_msecs;
		seen[m] = true;
	}

	let dest = null;
	if (match(DEST_ARG, /^[0-9a-f:]{17}$/)) dest = DEST_ARG;
	else for (let m in keys(seen)) if (name(m) == DEST_ARG) dest = m;

	function is_silent(m) { return m != GATE_MAC && (!by[m] || by[m].last > SILENT_MS); }

	for (let m in keys(seen)) {
		let s = is_silent(m);
		if (s && silent_since[m] == null) {
			silent_since[m] = now - (by[m] ? by[m].last : SILENT_MS);
			log(RED + name(m) + ' went silent' + RESET);
		} else if (!s && silent_since[m] != null) {
			delete silent_since[m];
			log(GREEN + name(m) + ' is back' + RESET);
		}
	}

	let path = null, best = dest && by[dest] ? by[dest].best : null;
	if (best) {
		let nh = best.neigh_address;
		if (nh == dest) path = [GATE_MAC, dest];
		else path = (!is_silent(nh) ? trace(dest) : null) ?? [GATE_MAC, nh, dest];
	}
	let key = path ? join('>', path) : 'none';
	if (key != last_key) {
		if (last_key == null) log('watching the route to ' + DEST_ARG + ': ' + describe(path));
		else {
			let why = '';
			for (let m in (last_path ?? [])) if (silent_since[m] != null) {
				why = sprintf('  (%.1f s after %s went silent)', (now - silent_since[m]) / 1000.0, name(m));
				break;
			}
			log(YELLOW + 'rerouted: ' + describe(path) + RESET + why);
		}
		last_key = key; last_path = path;
	}

	// ----- screen -----
	let L = [];
	push(L, '  ' + BOLD + 'HAVEN MESH  ·  LIVE ROUTE' + RESET + rep(' ', 30) + DIM + hhmmss() + RESET);
	push(L, '  ' + DIM + 'How traffic from ' + DEST_ARG + "'s clients reaches the uplink on " + GATE_NAME + ' right now' + RESET);
	push(L, '');

	let lost = null;
	if (path) for (let i = 1; i < length(path); i++) if (is_silent(path[i])) { lost = path[i]; break; }

	if (path) {
		// Drawn from the destination (where clients connect) toward the gate.
		let draw = reverse(path);
		let k = length(draw);
		let gap = k == 2 ? 2 * (BOX + 2) + 12 : 12;
		let items = [], arrows = [];
		for (let i = 0; i < k; i++) {
			let m = draw[i], s = is_silent(m);
			push(items, {
				label: i == k - 1 ? GATE_NAME : name(m),
				sub: s ? 'SILENT' : (i == k - 1 ? 'uplink' : (i == 0 ? 'clients' : 'relaying')),
				color: s ? RED : (i == k - 1 ? CYAN : GREEN)
			});
			if (i < k - 1) {
				let broken = false;
				for (let j = 0; j <= i + 1; j++) if (is_silent(draw[j])) broken = true;
				push(arrows, broken
					? RED + ' ' + rep('╌', gap - 3) + 'x ' + RESET
					: GREEN + ' ' + rep('═', gap - 3) + '▶ ' + RESET);
			}
		}
		for (let r in draw_boxes(items, gap, arrows, '    ')) push(L, r);

		// nodes not on the route, under the middle
		let others = [];
		for (let m in sort(keys(seen))) {
			if (m == GATE_MAC || index(path, m) >= 0) continue;
			let s = is_silent(m);
			push(others, { label: name(m), sub: s ? 'OFFLINE' : 'standby', color: s ? RED : DIM });
		}
		if (length(others)) {
			let sp = [];
			for (let i = 0; i < length(others) - 1; i++) push(sp, rep(' ', 4));
			for (let r in draw_boxes(others, 4, sp, '    ' + rep(' ', BOX + 2 + 12))) push(L, r);
		} else push(L, '', '', '', '');
	} else {
		push(L, '', '    ' + RED + DEST_ARG + ' is not in ' + GATE_NAME + "'s routing table." + RESET, '', '', '', '', '', '');
	}
	push(L, '');

	if (!path)
		push(L, '  ' + BG_RED + '  NO ROUTE  ' + RESET + '  ' + DEST_ARG + ' cannot be reached');
	else if (lost)
		push(L, '  ' + BG_RED + '  LINK LOST  ' + RESET + '  ' + name(lost) + ' has gone silent. batman-adv is finding another path.');
	else if (length(path) == 2)
		push(L, '  ' + BG_GREEN + '  DIRECT  ' + RESET + '  ' + name(dest) + ' talks straight to ' + GATE_NAME + ' over one HaLow hop');
	else
		push(L, '  ' + BG_YELLOW + '  RELAYED  ' + RESET + '  ' + describe(path) + ', ' + (length(path) - 1) + ' HaLow hops');
	push(L, '');

	push(L, '  ' + BOLD + 'Paths ' + GATE_NAME + ' knows to ' + DEST_ARG + RESET + DIM + '   batman-adv picks the fastest, every second' + RESET);
	let entries = dest && by[dest] ? sort(by[dest].all, (a, b) => b.throughput - a.throughput) : [];
	if (!length(entries)) push(L, '    ' + DIM + 'none' + RESET);
	for (let e in entries) {
		let via = e.neigh_address == dest ? 'direct' : 'via ' + name(e.neigh_address);
		let stale = e.last_seen_msecs > SILENT_MS || is_silent(e.neigh_address);
		let tag = e.best ? (stale ? RED + '▶ in use, but silent' + RESET : GREEN + '▶ in use' + RESET)
			: (stale ? RED + '  silent' + RESET : DIM + '  backup' + RESET);
		push(L, sprintf('    %s  %s %5.1f Mb/s   %s', e.best ? GREEN + '●' + RESET : ' ', pad(via, 16), e.throughput / 1000.0, tag));
	}
	push(L, '');

	push(L, '  ' + BOLD + 'Nodes' + RESET);
	push(L, '    ' + CYAN + '●' + RESET + '  ' + pad(GATE_NAME, 12) + DIM + 'gate (this screen)' + RESET);
	for (let m in sort(keys(seen))) {
		if (m == GATE_MAC) continue;
		let line = '    ';
		if (!by[m]) line += RED + '●' + RESET + '  ' + pad(name(m), 12) + RED + 'OFFLINE, dropped from the table' + RESET;
		else if (is_silent(m)) line += RED + '●' + RESET + '  ' + pad(name(m), 12) + RED + sprintf('SILENT for %d s', int(by[m].last / 1000)) + RESET;
		else line += GREEN + '●' + RESET + '  ' + pad(name(m), 12) + sprintf('heard %.1f s ago', by[m].last / 1000.0);
		push(L, line);
	}

	// the log gets whatever room is left; oldest lines go first
	let room = ROWS - length(L) - 2;
	if (room > 0) {
		push(L, '', '  ' + BOLD + 'What happened' + RESET);
		let start = length(events) - room;
		for (let i = start < 0 ? 0 : start; i < length(events); i++) push(L, '    ' + events[i]);
	}
	if (length(L) > ROWS) L = slice(L, 0, ROWS);

	if (ONCE) { print(join('\n', L) + '\n'); return; }
	// home, draw, clear the rest; no newline after the last line so it never scrolls
	print(E + 'H' + join(E + 'K\n', L) + E + 'K' + E + 'J');
}

if (!DEST_ARG) {
	load_names();
	let list = [];
	for (let o in (json(run('batctl oj') || '[]') ?? [])) if (o.best) push(list, name(o.orig_address));
	print('Usage: sh route-view.sh <node>\nNodes this one can reach: ' + (length(list) ? join(', ', list) : 'none') + '\n');
	exit(1);
}

load_names();
if (!ONCE) {
	print(E + '?1049h' + E + '?7l' + E + '?25l' + E + '2J');   // own screen, no line wrap, no cursor
	read_size();
}
let n = 0;
while (true) {
	let t0 = now_ms();
	frame();
	if (ONCE) break;
	n++;
	if (n % 3 == 0) read_size();
	if (n % 60 == 0) load_names();
	let wait = 1000 - (now_ms() - t0);
	if (wait > 50) sleep(wait);
}
