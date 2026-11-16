"""Build a small offline base map for SkyAware from Natural Earth (public domain).

Run on a computer with internet (Python 3, standard library only):

    python3 build-offline-map.py <center_lat> <center_lon> [out_dir]

Whole degrees are enough for the center. Downloads Natural Earth GeoJSON
(about 100 MB, cached), keeps what falls inside a box of roughly 1,200 x 1,500 km
around the center, rounds coordinates, and writes compact GeoJSON files
(usually under 1 MB) to out_dir (default: ./offline). Copy that folder to the
Haven and run install.sh. Lines are cut to the box; polygons and points are
kept when their bounding box touches it.
"""
import json
import os
import sys
import tempfile
import urllib.request

if len(sys.argv) < 3:
    sys.exit(__doc__)
lat0, lon0 = float(sys.argv[1]), float(sys.argv[2])
out = sys.argv[3] if len(sys.argv) > 3 else "offline"
BOX = (lon0 - 7, lat0 - 5.5, lon0 + 7, lat0 + 5.5)  # minlon, minlat, maxlon, maxlat
BASE = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/"
cache = os.path.join(tempfile.gettempdir(), "haven-natural-earth")
os.makedirs(cache, exist_ok=True)
os.makedirs(out, exist_ok=True)


def fetch(name):
    path = os.path.join(cache, name)
    if not os.path.exists(path):
        print("downloading", name, flush=True)
        urllib.request.urlretrieve(BASE + name, path)
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def inside(pt, pad=0.0):
    return BOX[0] - pad <= pt[0] <= BOX[2] + pad and BOX[1] - pad <= pt[1] <= BOX[3] + pad


def rnd(pt):
    return [round(pt[0], 4), round(pt[1], 4)]


def bbox_touches(coords):
    xs, ys = [], []

    def walk(c):
        if isinstance(c[0], (int, float)):
            xs.append(c[0]); ys.append(c[1])
        else:
            for x in c:
                walk(x)
    walk(coords)
    return not (max(xs) < BOX[0] or min(xs) > BOX[2] or max(ys) < BOX[1] or min(ys) > BOX[3])


def cut_line(line):
    """Split a line into the runs that are inside the box (one point of padding)."""
    runs, cur = [], []
    for pt in line:
        if inside(pt, 0.3):
            cur.append(rnd(pt))
        elif cur:
            cur.append(rnd(pt))
            if len(cur) > 1:
                runs.append(cur)
            cur = []
    if len(cur) > 1:
        runs.append(cur)
    return runs


def round_poly(rings):
    return [[rnd(p) for p in ring] for ring in rings]


def lines(src, dst, keep=lambda p: True, props=()):
    feats = []
    for f in fetch(src)["features"]:
        g = f.get("geometry")
        if not g or not keep(f["properties"]):
            continue
        parts = [g["coordinates"]] if g["type"] == "LineString" else g["coordinates"]
        runs = [r for part in parts for r in cut_line(part)]
        if runs:
            feats.append({"type": "Feature",
                          "properties": {k: f["properties"].get(k) for k in props},
                          "geometry": {"type": "MultiLineString", "coordinates": runs}})
    write(dst, feats)


def polys(src, dst, props=()):
    feats = []
    for f in fetch(src)["features"]:
        g = f.get("geometry")
        if not g or not bbox_touches(g["coordinates"]):
            continue
        if g["type"] == "Polygon":
            coords = round_poly(g["coordinates"])
        else:
            coords = [round_poly(p) for p in g["coordinates"]]
        feats.append({"type": "Feature",
                      "properties": {k: f["properties"].get(k) for k in props},
                      "geometry": {"type": g["type"], "coordinates": coords}})
    write(dst, feats)


def points(src, dst, keep=lambda p: True, name="name", extra=()):
    feats = []
    for f in fetch(src)["features"]:
        g = f.get("geometry")
        if not g or g["type"] != "Point" or not inside(g["coordinates"]) or not keep(f["properties"]):
            continue
        p = {"name": f["properties"].get(name)}
        for k in extra:
            p[k] = f["properties"].get(k)
        feats.append({"type": "Feature", "properties": p,
                      "geometry": {"type": "Point", "coordinates": rnd(g["coordinates"])}})
    write(dst, feats)


def write(dst, feats):
    path = os.path.join(out, dst)
    with open(path, "w", encoding="utf-8") as f:
        json.dump({"type": "FeatureCollection", "features": feats}, f, separators=(",", ":"))
    print("%-16s %5d features %8.0f KB" % (dst, len(feats), os.path.getsize(path) / 1024), flush=True)


polys("ne_50m_land.geojson", "land.geojson")
polys("ne_10m_lakes.geojson", "lakes.geojson")
lines("ne_10m_coastline.geojson", "coast.geojson")
lines("ne_10m_admin_1_states_provinces_lines.geojson", "states.geojson")
lines("ne_10m_roads.geojson", "roads.geojson",
      keep=lambda p: p.get("type") in ("Major Highway", "Secondary Highway"), props=("type",))
points("ne_10m_populated_places_simple.geojson", "places.geojson",
       keep=lambda p: (p.get("scalerank") or 10) <= 8, extra=("scalerank",))
points("ne_10m_airports.geojson", "airports.geojson", extra=("iata_code", "abbrev"))
