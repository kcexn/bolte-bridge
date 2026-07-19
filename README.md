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

## Usage

Build the bridge:

```bash
go build
```

This produces the `bolte-bridge` executable in the current directory.

Run the built executable:

```bash
./bolte-bridge
```

Alternatively, run the bridge directly with Go:

```bash
go run .
```

By default, the bridge creates and uses a SQLite database named
`bolte-bridge.db` in the current working directory.

### Configuration

The bridge can be configured through either command-line flags or
environment variables.

Command-line flags take precedence over environment variables, which in
turn take precedence over built-in defaults.

### Command-line options

| Flag | Description | Default |
| ---- | ----------- | ------- |
| `-d`, `--db-path` | Path to the SQLite database. | `bolte-bridge.db` |

### Environment variables

| Variable | Description | Default |
| ---- | ----------- | ------- |
| `BOLTE_BRIDGE_DB_PATH` | Path to the SQLite database. | `bolte-bridge.db` |

### Examples

The following examples demonstrate both supported configuration methods.

Use a custom database path:

```bash
go run . --db-path bridge.db
```

Or configure the database path through the environment:

```bash
export BOLTE_BRIDGE_DB_PATH=bridge.db
go run .
```
