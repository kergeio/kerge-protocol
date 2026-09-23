# Kerge protocol

The wire protocol between a [Kerge](https://kerge.io) agent and a Kerge
panel, and the reference implementation both sides build on.

`PROTOCOL.md` defines the protocol; `testdata/vectors.json` is a
language-independent conformance suite for a decoder.

```
go get github.com/kergeio/kerge-protocol
```

## Packages

| Package | Contents |
|---|---|
| `github.com/kergeio/kerge-protocol` | Message types, strict decoding, and validation of untrusted agent input |
| `github.com/kergeio/kerge-protocol/ifacefilter` | Network interface name matching and the default exclusion list |
| `github.com/kergeio/kerge-protocol/metering` | Turning the reported counters into network rates and traffic |

The module has no dependencies outside the standard library.

## Status

Pre-release. The current protocol version is `kerge.v1`, negotiated in the
WebSocket handshake. Until the first release it may still change without a
new version token.

## License

Apache License 2.0. See `LICENSE` and `NOTICE`.
