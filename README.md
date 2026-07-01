# Bolte Bridge -- email to matrix message bridge

[![CI](https://github.com/kcexn/bolte-bridge/actions/workflows/ci.yml/badge.svg)](https://github.com/kcexn/bolte-bridge/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/kcexn/bolte-bridge/graph/badge.svg?token=XVS6GG084Y)](https://codecov.io/gh/kcexn/bolte-bridge)

Bolte Bridge is a bidirectional bridge between traditional mailing lists and a Matrix room.
Messages posted to the list appear in the room, and messages sent in the room are delivered
to the list. Members can follow and join the conversation from whichever medium they
prefer.

It aims for a seamless experience across both sides:

- **Bidirectional relay** between one mailing list and one Matrix room.
- **Sender fidelity** — list senders appear as themselves in Matrix, and Matrix
  users get a stable, attributable address on the list.
- **Threading fidelity** — email reply chains map to Matrix replies/threads and
  vice versa.
- **No message loops** — the bridge never re-bridges its own traffic.

Bolte Bridge is designed to support the [Melbourne Linux User Group](https://groups.google.com/g/mlug-au)
mailing list.
