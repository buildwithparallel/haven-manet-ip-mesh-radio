#!/usr/bin/env python3
"""
HaLow SNR Audible Monitor

Run on a laptop connected to any Haven mesh point (via WiFi or ethernet).
Beep rate indicates signal quality; periodically announces internet speed.

Usage:
    python3 halow_monitor.py                      # auto-detect gateway, prompt for password
    python3 halow_monitor.py -p havenblue          # point node (blue) default password
    python3 halow_monitor.py -H 10.42.0.1 -p pass  # explicit host
    python3 halow_monitor.py --no-audio             # visual only, no beeps or speech
"""

import argparse
import subprocess
import sys
import time
import wave
import struct
import tempfile
import os
import math
import re
import shutil
import threading
import platform

# SNR thresholds (dB)
SNR_UNUSABLE = 3
SNR_MIN = 3
SNR_MAX = 35

BEEP_INTERVAL_SLOW = 1.8
BEEP_INTERVAL_FAST = 0.12

PING_INTERVAL = 15


def detect_gateway():
    """Auto-detect the default gateway IP (i.e. the mesh point we're connected to)."""
    try:
        if platform.system() == 'Darwin':
            r = subprocess.run(['route', '-n', 'get', 'default'],
                               capture_output=True, text=True, timeout=5)
            m = re.search(r'gateway:\s+([0-9.]+)', r.stdout)
            if m:
                return m.group(1)
        else:
            r = subprocess.run(['ip', 'route', 'show', 'default'],
                               capture_output=True, text=True, timeout=5)
            m = re.search(r'via\s+([0-9.]+)', r.stdout)
            if m:
                return m.group(1)
    except Exception:
        pass
    return None


def find_audio_player():
    """Find available audio player command."""
    for cmd in ['afplay', 'aplay', 'paplay']:
        try:
            subprocess.run(['which', cmd], capture_output=True, timeout=3)
            if subprocess.run(['which', cmd], capture_output=True, timeout=3).returncode == 0:
                return cmd
        except Exception:
            continue
    return None


def find_tts():
    """Find available text-to-speech command."""
    for cmd, args in [('say', ['-r', '190']), ('espeak', []), ('spd-say', [])]:
        try:
            if subprocess.run(['which', cmd], capture_output=True, timeout=3).returncode == 0:
                return cmd, args
        except Exception:
            continue
    return None, []


def make_tone(path, freq=800, duration=0.06, volume=0.6):
    rate = 44100
    n = int(rate * duration)
    with wave.open(path, 'w') as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(rate)
        for i in range(n):
            env = min(1.0, min(i, n - i) / (rate * 0.005))
            s = volume * env * math.sin(2 * math.pi * freq * i / rate)
            w.writeframes(struct.pack('<h', int(s * 32767)))


def make_chime(path, volume=0.5):
    rate = 44100
    dur1 = 0.07
    dur2 = 0.09
    gap = 0.03
    n1 = int(rate * dur1)
    n2 = int(rate * dur2)
    ng = int(rate * gap)
    with wave.open(path, 'w') as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(rate)
        for i in range(n1):
            env = min(1.0, min(i, n1 - i) / (rate * 0.005))
            s = volume * env * math.sin(2 * math.pi * 523 * i / rate)
            w.writeframes(struct.pack('<h', int(s * 32767)))
        for i in range(ng):
            w.writeframes(struct.pack('<h', 0))
        for i in range(n2):
            env = min(1.0, min(i, n2 - i) / (rate * 0.005))
            s = volume * env * math.sin(2 * math.pi * 784 * i / rate)
            w.writeframes(struct.pack('<h', int(s * 32767)))


def normalize_snr(snr):
    return max(0.0, min(1.0, (snr - SNR_MIN) / (SNR_MAX - SNR_MIN)))


def play(player, path):
    if player:
        subprocess.Popen([player, path],
                         stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def speak(tts_cmd, tts_args, text):
    if tts_cmd:
        subprocess.Popen([tts_cmd] + tts_args + [text],
                         stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def main():
    parser = argparse.ArgumentParser(
        description='HaLow SNR Audible Monitor - connect to a Haven mesh point '
                    'and monitor signal quality with audio feedback.')
    parser.add_argument('-H', '--host',
                        help='Mesh point IP (default: auto-detect gateway)')
    parser.add_argument('-u', '--user', default='root',
                        help='SSH username (default: root)')
    parser.add_argument('-p', '--password',
                        help='SSH password (e.g. havenblue, havengreen)')
    parser.add_argument('--no-audio', action='store_true',
                        help='Disable all audio (beeps and speech)')
    args = parser.parse_args()

    # Resolve host
    host = args.host
    if not host:
        host = detect_gateway()
        if not host:
            print("ERROR: Could not detect gateway. Specify with -H <ip>")
            sys.exit(1)
        print(f"  Auto-detected gateway: {host}")

    # Resolve password
    password = args.password
    if not password:
        import getpass
        password = getpass.getpass(f"SSH password for root@{host}: ")

    user = args.user

    # Check sshpass
    if subprocess.run(['which', 'sshpass'], capture_output=True).returncode != 0:
        print("ERROR: sshpass is required. Install with: brew install sshpass / apt install sshpass")
        sys.exit(1)

    # Audio setup
    audio_player = None if args.no_audio else find_audio_player()
    tts_cmd, tts_args = (None, []) if args.no_audio else find_tts()

    tmpdir = tempfile.mkdtemp()
    beep_path = os.path.join(tmpdir, 'beep.wav')
    make_tone(beep_path, freq=800)
    chime_path = os.path.join(tmpdir, 'chime.wav')
    make_chime(chime_path)

    print("\nHaLow SNR Audible Monitor")
    print(f"  Target:         {user}@{host}")
    print(f"  SNR > {SNR_MAX} dB    = rapid beeps")
    print(f"  SNR < {SNR_UNUSABLE} dB     = silence (unusable)")
    print(f"  Rising chime   = internet reachable")
    print(f"  Audio player:  {audio_player or 'none'}")
    print(f"  TTS:           {tts_cmd or 'none'}")
    print("  Ctrl+C to stop\n")
    print(f"  Connecting to {host}...")

    current_snr = [None]
    internet_ok = [None]
    throughput = [None]
    running = [True]

    ssh_base = ['sshpass', '-p', password, 'ssh',
                '-o', 'StrictHostKeyChecking=no',
                '-o', 'ConnectTimeout=5']

    def ssh_poller():
        while running[0]:
            try:
                ssh = subprocess.Popen(
                    ssh_base + [
                        '-o', 'ServerAliveInterval=3',
                        '-o', 'ServerAliveCountMax=2',
                        f'{user}@{host}',
                        'while true; do'
                        '  INFO=$(iwinfo wlan0 assoclist 2>/dev/null | head -1);'
                        '  SIG=$(echo "$INFO" | grep -oE "\\-?[0-9]+ dBm" | head -1 | grep -oE "\\-?[0-9]+");'
                        '  NOISE=$(echo "$INFO" | grep -oE "/ \\-?[0-9]+ dBm" | grep -oE "\\-?[0-9]+");'
                        '  SNR=$(echo "$INFO" | grep -oE "SNR [0-9]+" | grep -oE "[0-9]+");'
                        '  [ -n "$SNR" ] && [ -n "$SIG" ] && [ -n "$NOISE" ]'
                        '    && echo "SNR:${SNR}:S:${SIG}:N:${NOISE}"'
                        '    || echo "SNR:none";'
                        '  sleep 0.5;'
                        'done'],
                    stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                    text=True, bufsize=1
                )
                for line in ssh.stdout:
                    if not running[0]:
                        break
                    m = re.match(r'SNR:(-?\d+):S:(-?\d+):N:(-?\d+)', line.strip())
                    if m:
                        current_snr[0] = (
                            int(m.group(1)),
                            int(m.group(2)),
                            int(m.group(3)),
                        )
                    elif 'none' in line:
                        current_snr[0] = None
                ssh.terminate()
            except Exception:
                pass
            if running[0]:
                current_snr[0] = None
                time.sleep(2)

    def ping_checker():
        while running[0]:
            try:
                r = subprocess.run(
                    ssh_base + [
                        f'{user}@{host}',
                        'RTT=$(ping -c 1 -W 3 8.8.8.8 2>/dev/null'
                        '  | grep -oE "time=[0-9.]+" | grep -oE "[0-9.]+");'
                        'T1=$(cut -d" " -f1 /proc/uptime);'
                        'wget -q -O /dev/null "http://speed.cloudflare.com/__down?bytes=100000" 2>/dev/null;'
                        'T2=$(cut -d" " -f1 /proc/uptime);'
                        'DT=$(awk "BEGIN{printf \\"%.2f\\", $T2-$T1}");'
                        'SPEED=$(awk "BEGIN{if($DT>0) printf \\"%.0f\\", 100000*8/$DT; else print 0}");'
                        '[ -n "$RTT" ] && echo "OK:RTT:${RTT}:DL:${SPEED}"'
                        '  || echo "FAIL"'],
                    capture_output=True, text=True, timeout=30
                )
                if 'OK' in r.stdout:
                    internet_ok[0] = True
                    m = re.search(r'OK:RTT:([0-9.]+):DL:([0-9.]+)', r.stdout)
                    if m:
                        rtt = float(m.group(1))
                        dl_bps = float(m.group(2))
                        dl_kbps = dl_bps / 1000
                        throughput[0] = (dl_kbps, rtt)

                        speed_say = f"{dl_kbps/1000:.2f} megabits per second"
                        speak(tts_cmd, tts_args,
                              f"Internet connected. {speed_say}, "
                              f"{rtt:.0f} milliseconds ping.")
                    play(audio_player, chime_path)
                else:
                    internet_ok[0] = False
                    throughput[0] = None
                    speak(tts_cmd, tts_args, 'No internet connection.')
            except Exception:
                internet_ok[0] = False
            time.sleep(PING_INTERVAL)

    threading.Thread(target=ssh_poller, daemon=True).start()
    threading.Thread(target=ping_checker, daemon=True).start()

    try:
        while True:
            data = current_snr[0]
            if data is not None:
                snr, sig, noise = data

                tp = throughput[0]
                if tp:
                    dl, rtt = tp
                    if dl >= 1000:
                        tp_str = f"DL:{dl/1000:.1f}Mbps  RTT:{rtt:.0f}ms"
                    else:
                        tp_str = f"DL:{dl:.0f}kbps  RTT:{rtt:.0f}ms"
                else:
                    tp_str = "DL:--  RTT:--"
                inet = "INET:yes" if internet_ok[0] else (
                    "INET:no" if internet_ok[0] is False else "INET:?")

                if snr < SNR_UNUSABLE:
                    print(f'\r  SNR: {snr:>3} dB  (sig:{sig} noise:{noise})'
                          f'  [--------------------]  unusable  {inet}  {tp_str}  ',
                          end='', flush=True)
                    time.sleep(1)
                    continue

                norm = normalize_snr(snr)
                interval = BEEP_INTERVAL_SLOW - norm * (BEEP_INTERVAL_SLOW - BEEP_INTERVAL_FAST)

                bar_len = int(norm * 20)
                bars = '#' * bar_len + '-' * (20 - bar_len)
                print(f'\r  SNR: {snr:>3} dB  (sig:{sig} noise:{noise})'
                      f'  [{bars}]  {inet}  {tp_str}  ',
                      end='', flush=True)

                play(audio_player, beep_path)
                time.sleep(interval)
            else:
                print('\r  SNR: --- dB  '
                      '[--------------------]  connecting...          ',
                      end='', flush=True)
                time.sleep(1)
    except KeyboardInterrupt:
        running[0] = False
        print("\n\nStopped.")
    finally:
        running[0] = False
        shutil.rmtree(tmpdir, ignore_errors=True)


if __name__ == '__main__':
    main()
