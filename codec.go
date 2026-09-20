package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrTooLarge means the message exceeds the limit for its direction.
var ErrTooLarge = errors.New("protocol: message too large")

// DecodeAgentMessage decodes one message received from an agent. The result
// is normalized and validated, so the caller may use it directly; any error
// means the message must be dropped and the connection closed.
func DecodeAgentMessage(data []byte) (AgentMessage, error) {
	if len(data) > MaxPanelRead {
		return nil, ErrTooLarge
	}
	name, err := peekType(data)
	if err != nil {
		return nil, err
	}
	switch name {
	case TypeHostInfo:
		m := new(HostInfo)
		if err := decodeInto(data, m); err != nil {
			return nil, err
		}
		m.Normalize()
		if err := m.Validate(); err != nil {
			return nil, err
		}
		return m, nil
	case TypeMetrics:
		m := new(Metrics)
		if err := decodeInto(data, m); err != nil {
			return nil, err
		}
		if err := m.Validate(); err != nil {
			return nil, err
		}
		return m, nil
	default:
		return nil, unknownType(name)
	}
}

// DecodePanelMessage decodes one message received from the panel. The agent
// accepts only the two whitelisted types; anything else is an error and the
// agent reconnects (REQUIREMENTS 5.1).
func DecodePanelMessage(data []byte) (PanelMessage, error) {
	if len(data) > MaxAgentRead {
		return nil, ErrTooLarge
	}
	name, err := peekType(data)
	if err != nil {
		return nil, err
	}
	switch name {
	case TypeRegistered:
		m := new(Registered)
		if err := decodeInto(data, m); err != nil {
			return nil, err
		}
		if err := m.Validate(); err != nil {
			return nil, err
		}
		return m, nil
	case TypeError:
		m := new(ErrorMessage)
		if err := decodeInto(data, m); err != nil {
			return nil, err
		}
		m.Normalize()
		if err := m.Validate(); err != nil {
			return nil, err
		}
		return m, nil
	default:
		return nil, unknownType(name)
	}
}

// Marshal encodes a message, setting its type field.
func Marshal(m any) ([]byte, error) {
	switch v := m.(type) {
	case *HostInfo:
		v.Type = TypeHostInfo
	case *Metrics:
		v.Type = TypeMetrics
	case *Registered:
		v.Type = TypeRegistered
	case *ErrorMessage:
		v.Type = TypeError
	default:
		return nil, fmt.Errorf("protocol: cannot marshal %T", m)
	}
	return json.Marshal(m)
}

// peekType reads the type field without rejecting the other fields.
func peekType(data []byte) (string, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return "", fmt.Errorf("protocol: %w", err)
	}
	return head.Type, nil
}

// decodeInto decodes exactly one JSON object into v, rejecting unknown
// fields and anything following the object.
func decodeInto(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("protocol: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("protocol: trailing data after message")
	}
	return nil
}

// unknownType reports a type the receiver does not accept. The name is
// quoted with %q so that control characters cannot reach a log line raw.
func unknownType(name string) error {
	if len(name) > 32 {
		name = name[:32]
	}
	return fmt.Errorf("protocol: unknown message type %q", name)
}

// ParseAuthorization splits an Authorization header value into its kind,
// KindEnroll or KindAgent, and the credential that follows it.
func ParseAuthorization(header string) (kind, value string, err error) {
	rest, ok := strings.CutPrefix(header, authScheme)
	if !ok {
		return "", "", errors.New("protocol: authorization must use the Bearer scheme")
	}
	for _, kind := range []string{KindEnroll, KindAgent} {
		if v, ok := strings.CutPrefix(rest, kind); ok {
			if v == "" {
				return "", "", errors.New("protocol: empty credential")
			}
			return kind, v, nil
		}
	}
	return "", "", errors.New("protocol: unknown credential kind")
}
