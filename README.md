# Kerge protocol

The wire protocol between a [Kerge](https://kerge.io) agent and a Kerge
panel, and the reference implementation both sides build on.

```
go get kerge.io/protocol
```

## Packages

| Package | Contents |
|---|---|
| `kerge.io/protocol` | Message types, strict decoding, and validation of untrusted agent input |
| `kerge.io/protocol/ifacefilter` | Network interface name matching and the default exclusion list |
| `kerge.io/protocol/metering` | Turning the reported counters into network rates |

The module has no dependencies outside the standard library.

## Status

Pre-release. The protocol is not yet versioned and may change without
notice; a normative `PROTOCOL.md` and an explicit version handshake are
being written. Until then the Go packages are the reference.

## License

Apache License 2.0. See `LICENSE` and `NOTICE`.
