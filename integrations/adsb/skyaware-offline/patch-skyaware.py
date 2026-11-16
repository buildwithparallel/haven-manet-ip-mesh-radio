"""Patch the dump1090 SkyAware map on a Haven for the offline base map.

Run on the node (install.sh calls it): python3 patch-skyaware.py <lat> <lon>

- layers.js: add "Offline (Natural Earth)" as the first base map, so it is the
  default for a browser that has not picked one.
- config.js: center the map on <lat> <lon> at zoom 8 (it ships centered on Italy).
- script.js: remember the chosen map under a new key, so browsers that already
  picked an online map start on the offline one once.
- index.html: bump the script versions so browsers load the new files instead
  of a cached copy.

Safe to run again. Originals are saved once in /etc/haven-backup/skyaware/.
"""
import os
import re
import shutil
import sys

lat, lon = float(sys.argv[1]), float(sys.argv[2])
root = "/usr/share/dump1090/"
here = os.path.dirname(os.path.abspath(__file__))
backup = "/etc/haven-backup/skyaware/"
os.makedirs(backup, exist_ok=True)
for name in ("layers.js", "config.js", "script.js", "index.html"):
    if not os.path.exists(backup + name):
        shutil.copy2(root + name, backup + name)


def edit(name, fn):
    path = root + name
    old = open(path).read()
    new = fn(old)
    if new != old:
        open(path, "w").write(new)
        print("patched", name)
    else:
        print("unchanged", name)


def layers(s):
    if "Offline (Natural Earth)" in s:
        return s
    anchor = "        var europe = [];\n"
    if s.count(anchor) != 1:
        sys.exit("layers.js: anchor not found; this SkyAware version is not supported")
    return s.replace(anchor, anchor + "\n" + open(os.path.join(here, "offline-layer.js")).read())


def config(s):
    s = re.sub(r"^DefaultCenterLat\s*=.*$", "DefaultCenterLat = %s;" % lat, s, flags=re.M)
    s = re.sub(r"^DefaultCenterLon\s*=.*$", "DefaultCenterLon = %s;" % lon, s, flags=re.M)
    return re.sub(r"^DefaultZoomLvl\s*=.*$", "DefaultZoomLvl   = 8;", s, flags=re.M)


def script(s):
    return s.replace("localStorage['MapType']", "localStorage['MapTypeHaven']")


def index(s):
    return re.sub(r'((?:config|layers|script)\.js)\?v=[^"]*', r"\1?v=4.0-haven-offline", s)


edit("layers.js", layers)
edit("config.js", config)
edit("script.js", script)
edit("index.html", index)
