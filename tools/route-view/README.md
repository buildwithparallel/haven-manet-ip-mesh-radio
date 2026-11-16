# Route view

A live terminal view of how traffic from one Haven node reaches another, made to be shown on a screen or filmed. It redraws every second from batman-adv's own tables.

```text
  HAVEN MESH  ·  LIVE ROUTE                              13:12:11
  How traffic from green reaches blue right now

    ┌──────────┐                                    ┌──────────┐
    │  GREEN   │ ═════════════════════════════════▶ │   BLUE   │
    │   gate   │                                    │receiving │
    └──────────┘                                    └──────────┘
                            ┌──────────┐
                            │ HELTEC-4 │
                            │ standby  │
                            └──────────┘

    DIRECT    green talks straight to blue over one HaLow hop

  Paths green knows to blue   batman-adv picks the fastest, every second
    ●  direct            19.4 Mb/s   ▶ in use
       via heltec-4      16.1 Mb/s     backup

  Nodes
    ●  green       gate (this screen)
    ●  heltec-4    heard 0.1 s ago
    ●  blue        heard 0.4 s ago

  What happened
    13:12:11  watching the route to blue: direct
```

## What it shows

| Part | Meaning |
|---|---|
| Boxes and arrows | The path traffic takes right now. Nodes not on the path sit below, as **standby** or **OFFLINE**. |
| Banner | **DIRECT** (one HaLow hop), **RELAYED** (another node passes the traffic along), **LINK LOST** (a node on the path has gone silent and batman-adv has not switched yet) or **NO ROUTE**. |
| Paths | Every path this node knows to the destination, with batman-adv's speed estimate. The fastest is in use; the rest are backups ready to take over. |
| Nodes | When each node was last heard. After 3 seconds without hearing a node, it shows as silent. |
| What happened | Route changes and nodes going silent or coming back, with times. A reroute says how many seconds it took after the node went silent. |

## Install and run

Copy both files to the node you want to watch from, usually the gate:

```sh
ssh root@<gate> 'cat > /root/route-view.uc' < tools/route-view/route-view.uc
ssh root@<gate> 'cat > /root/route-view.sh' < tools/route-view/route-view.sh
```

Then, on that node:

```sh
sh /root/route-view.sh blue
```

Run it with no node name to list the nodes it can reach. Press Ctrl-C to stop. The terminal goes back to normal.

## Notes

- Names come from alfred's `/etc/bat-hosts`. Nodes alfred does not name, such as Heltec dongles, show as MAC addresses. To name them, add lines like `0c:bf:74:1f:ef:b7 heltec-4` to `/etc/haven-names`.
- It needs only `ucode` and `batctl`, which OpenMANET already has. It reads the window size with BusyBox `resize`, and fits the screen by trimming the oldest log lines.
- Zoom the terminal in for filming. The full view needs about 80 columns and 30 rows.
- The path comes from `batctl tr`, so it is the real path even through several relays.
