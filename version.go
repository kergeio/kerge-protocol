package protocol

import "strings"

// Version is the protocol version this implementation speaks. The agent and
// the panel agree on it during the WebSocket handshake, before any message
// is exchanged (REQUIREMENTS 5.1).
const Version = "kerge.v1"

// SubprotocolHeader carries the offered versions in the handshake. The
// agent lists what it supports and the panel echoes the one it picked.
const SubprotocolHeader = "Sec-WebSocket-Protocol"

// Supported lists the versions this implementation speaks, most preferred
// first. The agent offers all of them; the panel picks the first one it
// shares with the agent.
func Supported() []string { return []string{Version} }

// Supports reports whether version is one this implementation speaks. The
// agent checks the version the panel echoed, because a panel that echoes
// nothing has not agreed to anything.
func Supports(version string) bool {
	for _, v := range Supported() {
		if v == version {
			return true
		}
	}
	return false
}

// SelectVersion picks the first supported version the agent offered. The
// argument holds the values of the agent's Sec-WebSocket-Protocol header,
// each of which may list several versions separated by commas.
//
// Version tokens are compared exactly: an agent that spells one differently
// is refused rather than guessed at. No common version means the panel must
// refuse the handshake with 400 instead of upgrading the connection.
func SelectVersion(offered []string) (string, bool) {
	for _, want := range Supported() {
		for _, value := range offered {
			for _, token := range strings.Split(value, ",") {
				if strings.TrimSpace(token) == want {
					return want, true
				}
			}
		}
	}
	return "", false
}
