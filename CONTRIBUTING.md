# Contributing

Thanks for your interest in Kerge.

## Licensing

This repository is licensed under the Apache License 2.0 (see `LICENSE`).
Contributions are accepted under the same license.

## Developer Certificate of Origin

Every commit must carry a `Signed-off-by` line certifying the Developer
Certificate of Origin (see the `DCO` file):

```
git commit -s
```

The line must match the author of the commit:

```
Signed-off-by: Jane Doe <jane@example.com>
```

There is no CLA to sign.

## What belongs here

This repository holds the wire protocol between a Kerge agent and a Kerge
panel, and the reference implementation both sides use:

- message types, strict decoding and validation of untrusted input;
- the interface name matching rules;
- how the reported counters are turned into rates and increments.

It deliberately contains nothing product-specific. Agent collection lives
in the agent repository, and the user interface, storage and alerting in
the panel repository.

Anything that a second panel implementation would have to reimplement in
order to stay compatible belongs here. Anything that is a product
decision does not.

## Before opening a pull request

```
make check
```

That runs formatting, `go vet`, the tests with the race detector, a
vulnerability scan, and the checks described below.

## House rules

- Everything in this repository is written in English: code, comments,
  commit messages, branch names and documentation.
- Commit messages follow Conventional Commits, for example
  `feat(protocol): add the host_info interval field`.
- Changes to the wire format need a matching change to the protocol
  version; see `PROTOCOL.md` once it lands.
