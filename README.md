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
BOLTE_BRIDGE_EMAIL_PASSWORD=<app-password> ./bolte-bridge --email bridge@example.com
```

Alternatively, run the bridge directly with Go:

```bash
BOLTE_BRIDGE_EMAIL_PASSWORD=<app-password> go run . --email bridge@example.com
```

The bridge signs in to a single mail account, which it uses to read from and
post to the mailing list. The account address and password are **required**;
the bridge exits at startup if either is missing. Everything else has a
default.

By default, the bridge creates and uses a SQLite database named
`bolte-bridge.db` in the current working directory, and talks to Gmail's
IMAP and SMTP endpoints.

### Configuration

The bridge can be configured through either command-line flags or
environment variables.

Command-line flags take precedence over environment variables, which in
turn take precedence over built-in defaults.

The account password is the one exception: it can only be supplied through the
environment. It has no flag, so that it never appears in `argv`, where it would
be visible in the process table and in shell history.

### Command-line options

| Flag | Description | Default |
| ---- | ----------- | ------- |
| `-d`, `--db-path` | Path to the SQLite database. | `bolte-bridge.db` |
| `-e`, `--email` | Account name for IMAP/SMTP (the full email address). **Required.** | — |
| `--email-imap-addr` | `host:port` of the IMAP endpoint (implicit TLS). | `imap.gmail.com:993` |
| `--email-smtp-addr` | `host:port` of the SMTP submission endpoint (STARTTLS). | `smtp.gmail.com:587` |
| `--email-mailbox` | IMAP mailbox to fetch from. | `INBOX` |

### Environment variables

Every setting is also readable from the environment, under the
`BOLTE_BRIDGE_` prefix.

| Variable | Description | Default |
| ---- | ----------- | ------- |
| `BOLTE_BRIDGE_DB_PATH` | Path to the SQLite database. | `bolte-bridge.db` |
| `BOLTE_BRIDGE_EMAIL_ACCOUNT` | Account name for IMAP/SMTP (the full email address). **Required.** | — |
| `BOLTE_BRIDGE_EMAIL_PASSWORD` | Account password. **Required**, and settable only here. | — |
| `BOLTE_BRIDGE_EMAIL_IMAP_ADDR` | `host:port` of the IMAP endpoint (implicit TLS). | `imap.gmail.com:993` |
| `BOLTE_BRIDGE_EMAIL_SMTP_ADDR` | `host:port` of the SMTP submission endpoint (STARTTLS). | `smtp.gmail.com:587` |
| `BOLTE_BRIDGE_EMAIL_MAILBOX` | IMAP mailbox to fetch from. | `INBOX` |

The IMAP endpoint is contacted over implicit TLS and the SMTP endpoint over
STARTTLS, so the port you choose should be one the server offers for that
scheme.

If you point the bridge at a Gmail account, the password is an
[app password](https://support.google.com/accounts/answer/185833), not the
account's own password; Gmail rejects IMAP and SMTP logins that use the latter.

### Examples

The following examples demonstrate both supported configuration methods.

Run against Gmail with the default endpoints and database:

```bash
export BOLTE_BRIDGE_EMAIL_PASSWORD=<app-password>
go run . --email bridge@example.com
```

Use a custom database path:

```bash
export BOLTE_BRIDGE_EMAIL_PASSWORD=<app-password>
go run . --email bridge@example.com --db-path bridge.db
```

Point the bridge at a non-Gmail provider, and read from a mailbox other than
`INBOX`:

```bash
export BOLTE_BRIDGE_EMAIL_PASSWORD=<app-password>
go run . \
  --email bridge@example.com \
  --email-imap-addr imap.example.com:993 \
  --email-smtp-addr smtp.example.com:587 \
  --email-mailbox Lists/mlug
```

Or configure everything through the environment:

```bash
export BOLTE_BRIDGE_DB_PATH=bridge.db
export BOLTE_BRIDGE_EMAIL_ACCOUNT=bridge@example.com
export BOLTE_BRIDGE_EMAIL_PASSWORD=<app-password>
export BOLTE_BRIDGE_EMAIL_IMAP_ADDR=imap.example.com:993
export BOLTE_BRIDGE_EMAIL_SMTP_ADDR=smtp.example.com:587
export BOLTE_BRIDGE_EMAIL_MAILBOX=Lists/mlug
go run .
```
