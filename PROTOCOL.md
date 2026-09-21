# Kerge agent protocol

Version `kerge.v1`.

This document defines the wire protocol between a Kerge agent and a Kerge
panel. It is normative: an implementation that follows it interoperates with
the Go packages in this repository, which are the reference implementation.
"Must" and "may" carry their usual meaning.

## 1. Model

A panel monitors many hosts. Each host runs one agent, which connects to the
panel and reports what it measures. The protocol is one-directional in
substance: the agent reports, and the panel answers only with the result of
an enrollment or with the reason it is ending the connection.

**There is no command message, in either direction, and none will be added.**
A panel cannot ask an agent to run anything, read a file, or change its
configuration. An agent that receives a message it does not recognise closes
the connection rather than guessing what it means.

The agent always dials out and never listens on a port.

## 2. Transport

The endpoint is `wss://<panel>/api/agent/ws`. Plain `ws://` is allowed only
when the host is a loopback address (`localhost`, `127.0.0.1`, `::1`); for
any other host the agent must use TLS and verify the certificate.

Connections use WebSocket (RFC 6455) with the `permessage-deflate`
extension, context takeover included. Metrics are repetitive JSON, and
compression cuts the outbound traffic of a reporting agent to roughly a
quarter.

Every protocol message is one JSON object in one **text** frame. A binary
frame ends the connection. Size limits apply to the decompressed message:

| Receiver | Limit |
|---|---|
| Panel | 64 KiB |
| Agent | 4 KiB |

## 3. Handshake

### 3.1 Version negotiation

The protocol version is settled in the HTTP upgrade, before any message is
exchanged.

1. The agent lists the versions it speaks in `Sec-WebSocket-Protocol`, most
   preferred first. The only version defined today is `kerge.v1`.
2. The panel picks the first version in *its own* preference order that the
   agent also offered, and echoes that one token in the response.
3. If the header is absent or the two sides share no version, the panel
   refuses the handshake with **HTTP 400** and does not upgrade the
   connection.
4. The agent must check the echoed token. A panel that echoes nothing has
   agreed to nothing: the agent closes the connection and retries with
   backoff (§7.6), rather than reporting into it.

Version tokens are compared exactly. `KERGE.V1` is not `kerge.v1` and is
refused, so that a spelling mistake fails loudly instead of being guessed at.

### 3.2 Authentication

The credential travels in the `Authorization` header of the upgrade request,
never in the URL, where it would reach proxy and server logs. Two kinds
exist, told apart by their prefix:

| Kind | Header value |
|---|---|
| Enrollment | `Bearer enroll:<token>` |
| Established | `Bearer agent:<agent_id>.<secret>` |

An unknown prefix, an unknown or spent token, and an unknown or wrong
credential all produce **HTTP 401** before the upgrade. A panel that has not
completed its own first-run setup answers **HTTP 503**.

The panel settles the version first and authenticates second. A handshake
refused for the version therefore never consumes a one-time enrollment
token.

## 4. Enrollment

An agent is enrolled once and then uses a long-lived credential.

1. The operator adds the host in the panel, which generates a **one-time
   enrollment token**: 32 random bytes, stored only as a hash, valid for 24
   hours.
2. The agent connects with `Bearer enroll:<token>`.
3. The panel validates the token, generates `agent_id` and `secret` for the
   host, stores only the hash of the secret, marks the token as used — all
   in one atomic step, so two agents racing on the same token cannot both
   succeed — and sends `registered` as the first message on that connection.
4. The agent stores the credential durably before it reports anything, so
   that a crash right after enrollment does not strand it with a spent
   token. The reference agent writes a single line `<agent_id>.<secret>` to
   a file with mode 0600, using a temporary file, `fsync` and `rename`.
5. The connection stays open. The agent continues with `host_info` and its
   metrics on the same connection.
6. Afterwards the agent presents `Bearer agent:<agent_id>.<secret>`.

**Recovery.** If an established credential is refused — HTTP 401 at the
handshake, or an `error` with code `unauthorized` on an open connection — an
agent that still has an enrollment token configured uses it on its next
attempt. This is how a host recovers after an operator resets its access. If
the token has already been spent, the attempt fails like any other and the
agent keeps retrying with backoff.

## 5. Messages

### 5.1 Framing and strictness

Both sides decode strictly. A message is rejected when it

- is not a single JSON object,
- carries anything after that object,
- has no `type` field, or a `type` the receiver does not accept in that
  direction,
- carries a field the receiver does not know,
- has a field of the wrong JSON type, or
- violates a limit in §6.

A rejected message ends the connection. The panel first sends `error` with
code `invalid_message`; the agent reconnects with backoff.

### 5.2 `host_info` (agent → panel)

Sent once per connection, before the first `metrics`.

```json
{ "type": "host_info", "hostname": "web-1", "os": "linux",
  "platform": "debian", "platform_version": "12", "kernel": "6.1.0",
  "arch": "amd64", "cpu_model": "Xeon", "cpu_cores": 2,
  "agent_version": "0.1.0", "interval_ms": 5000 }
```

| Field | Type | Meaning |
|---|---|---|
| `hostname` | string | The host's own name, which the panel shows beside the operator's label |
| `os` | string | `linux`, `darwin`, … |
| `platform` | string | Distribution, for example `debian` |
| `platform_version` | string | Distribution version |
| `kernel` | string | Kernel release |
| `arch` | string | `amd64`, `arm64`, … |
| `cpu_model` | string | May be empty: some platforms do not expose a model name |
| `cpu_cores` | integer | Logical cores |
| `agent_version` | string | Agent version |
| `interval_ms` | integer | The reporting interval the agent is configured with |

All fields are required; a string field with nothing to report is sent
empty. The panel replaces what it holds for the host every time it receives
this message.

### 5.3 `metrics` (agent → panel)

```json
{ "type": "metrics", "ts": 1726560000, "mono_ms": 86400123,
  "cpu_percent": 12.5,
  "mem_total": 2048000000, "mem_used": 812000000,
  "swap_total": 1073741824, "swap_used": 0,
  "load1": 0.42, "load5": 0.35, "load15": 0.3,
  "disk_total": 40000000000, "disk_used": 12000000000,
  "disk_free": 26000000000,
  "net": { "eth0": { "rx": 123456789, "tx": 987654321 } },
  "uptime": 864000 }
```

| Field | Type | Required | Meaning |
|---|---|---|---|
| `ts` | integer | yes | The agent's wall clock, Unix seconds. Advisory only (§7.3) |
| `mono_ms` | integer | yes | Milliseconds on the agent's monotonic clock since the agent started (§7.4) |
| `cpu_percent` | number | no | Busy time since the previous sample, 0 to 100 |
| `mem_total`, `mem_used` | integer | no | Bytes; `used` is `total - available` |
| `swap_total`, `swap_used` | integer | no | Bytes |
| `load1`, `load5`, `load15` | number | no | Load averages, omitted entirely on platforms without them |
| `disk_total`, `disk_used`, `disk_free` | integer | no | Bytes for the root filesystem. `total` includes reserved blocks, so `total` may exceed `used + free` |
| `net` | object | no | Cumulative byte counters per interface, keyed by interface name |
| `uptime` | integer | no | Seconds since boot |

An optional field is **omitted** when the agent has no value for it this
cycle — a collector that timed out, a first sample with no baseline, a
platform that does not provide it. Omitted is not zero: a receiver must
record "no data" and must not substitute `0`.

Fields that are measured together are reported together. The pairs and
groups are `mem_total`/`mem_used`, `swap_total`/`swap_used`,
`disk_total`/`disk_used`/`disk_free` and `load1`/`load5`/`load15`; a partial
group rejects the message.

Interfaces are filtered by the agent before sending, and the panel may
exclude more by name (§7.5). The counters are the kernel's cumulative
values, not deltas: rates are derived by the panel (§7.4).

### 5.4 `registered` (panel → agent)

```json
{ "type": "registered", "agent_id": "kRr8Qw", "secret": "Yx7pLq" }
```

Sent only on a connection authenticated with an enrollment token, as the
first message on that connection. Neither part may contain a dot, because
the two are joined with one in the `Authorization` header.

### 5.5 `error` (panel → agent)

```json
{ "type": "error", "code": "unauthorized", "message": "unknown credential" }
```

The panel sends this as the last message before it closes a connection. The
agent logs it, closes, and reconnects with backoff. `message` is English
text for a log line; **an agent must never branch on its content.**

| Code | Meaning |
|---|---|
| `unauthorized` | The credential was refused. Triggers the recovery of §4 |
| `invalid_message` | A message was rejected (§5.1) |
| `rate_limited` | Too many messages (§7.5) |
| `replaced` | Another connection took over this `agent_id` (§7.5) |
| `server_error` | The panel could not carry on |

An agent must accept a code it does not know and treat it like any other
error: log it and reconnect. This is the protocol's one forward-compatible
extension point (§8).

## 6. Limits

Everything a peer sends is untrusted input. Violating a limit rejects the
message, except where the table says the value is repaired.

| Subject | Rule |
|---|---|
| String fields of `host_info`, and `error.message` | Control characters are removed and the value is cut to 128 characters (`error.message`: 256). The message is kept |
| `cpu_cores` | 0 to 4096 |
| `interval_ms` | 3000 to 60000 |
| `ts`, `mono_ms` | Not negative |
| `cpu_percent` | 0 to 100 |
| `mem_used`, `swap_used` | Not above the matching total |
| `disk_used`, `disk_free` | Not above `disk_total` |
| Byte counts (`mem_*`, `swap_*`, `disk_*`) | At most 1 PiB (1125899906842624) |
| `uptime` | At most 3153600000 (a hundred years) |
| `load1`, `load5`, `load15` | Not negative, not NaN, not infinite |
| `net` | At most 64 interfaces |
| Interface name | 1 to 32 characters, printable ASCII, no space |
| `agent_id`, `secret` | 1 to 128 characters, printable ASCII, no space, no dot |
| `error.code` | 1 to 32 characters of `a`–`z`, `0`–`9` and `_` |

Numbers are JSON numbers. A byte count is unsigned: a negative value rejects
the message rather than wrapping.

## 7. Session behaviour

### 7.1 Order

On every connection the agent sends `host_info` first, then `metrics` every
`interval_ms` until the connection ends. Enrollment inserts one step: the
agent waits for `registered` before it sends anything.

### 7.2 No backfill

An agent does not buffer samples across a disconnection. When a connection
drops, the samples taken meanwhile are lost on purpose; the agent resumes
from the present moment. The panel is a monitor, not a log shipper, and a
reconnecting fleet must not flood it. For the same reason an agent keeps at
most one unsent sample: a newer sample replaces an older one still queued.

### 7.3 Time

The panel timestamps every sample with its **receive time** and stores that.
`ts` is the agent's own clock, kept for reference; an agent with a wrong
clock therefore skews nothing.

### 7.4 Rates

Network rates are derived by the panel from two consecutive messages of one
connection: the difference of a counter divided by the difference of
`mono_ms`, summed over the interfaces that count. `mono_ms` is monotonic, so
a clock step on the host cannot produce a nonsensical rate.

- If `mono_ms` did not advance, or the elapsed `mono_ms` differs from the
  elapsed receive time by more than a factor of two, the panel records no
  rate for that sample and only refreshes its baseline.
- If an individual counter went backwards — a restarted interface, a
  counter reset — that interface contributes nothing to that sample.

An agent restarts its monotonic clock at zero. That is a decrease, so the
first sample after an agent restart yields no rate, which is correct.

### 7.5 What the panel enforces

- **Liveness.** The panel sends a WebSocket ping every 30 seconds and
  expects a pong within 10 seconds. Answering pings is mandatory; a library
  that answers them only while a read is in progress requires the agent to
  keep a read outstanding at all times.
- **Rate limit.** By default 2 messages per second sustained, bursts of 20.
  Exceeding it ends the connection with `rate_limited`. A well-behaved agent
  reports every 3 to 60 seconds and stays far below.
- **One connection per agent.** A new connection for an `agent_id` replaces
  the previous one, which is closed with `replaced`.
- **Interface exclusion.** The panel may drop interfaces from what it
  stores, by name pattern, independently of the agent's own filtering. The
  agent is not told; it keeps reporting what it sees.
- **Offline hosts.** A connection belonging to a host the panel considers
  offline is closed.

### 7.6 Reconnection

After any failure — refused handshake, closed connection, rejected message —
the agent waits `random(0, min(60s, 2^n seconds))` before attempt *n*, with
the full jitter shown here, and resets *n* once a connection is established
and `host_info` has been sent. Jitter matters: without it, every agent of a
restarted panel comes back in lockstep.

## 8. Versioning

The version token is the whole of the compatibility story. Both sides decode
strictly, so a receiver cannot ignore anything it does not know:

- **A new or renamed field, a new message type, a field that changes type or
  widens its range, a rule that loosens — all require a new version token**
  (`kerge.v2`). An implementation that speaks both offers both, and the
  panel picks.
- **Within a version, one thing may change: the set of `error.code`
  values.** Codes are validated by shape, not by membership, and an agent
  treats an unknown one as a generic error. A newer panel can therefore say
  something more precise to an older agent without breaking it.

Because the agent offers every version it speaks and the panel picks, a
fleet can be upgraded in either order.

## 9. Conformance vectors

`testdata/vectors.json` is a language-independent test suite for a decoder.
Each case holds:

| Field | Meaning |
|---|---|
| `name` | Unique name of the case |
| `direction` | `agent_to_panel` (a panel decodes it) or `panel_to_agent` (an agent decodes it) |
| `input` | The message, as the exact string received |
| `accept` | Whether the receiver keeps the message |
| `canonical` | Optional. The decoded message re-encoded, which pins the normalization of §6 |
| `note` | The rule the case is about |

`input` is a string, not an object, so that malformed JSON can be expressed.
An implementation passes when it accepts exactly the cases marked `accept`
and reproduces every `canonical` form. The reference implementation runs
this suite in `TestVectors`.

## 10. Reference implementation

| Package | Contents |
|---|---|
| `github.com/kergeio/kerge-protocol` | Messages, strict decoding, validation, version negotiation |
| `github.com/kergeio/kerge-protocol/ifacefilter` | Interface name matching and the default exclusion list |
| `github.com/kergeio/kerge-protocol/metering` | Counters to rates (§7.4) |

The module depends on nothing outside the standard library.
