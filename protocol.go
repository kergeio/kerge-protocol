// Package protocol defines the JSON messages exchanged between the agent and
// the panel (REQUIREMENTS 5.3), together with strict decoding (5.1) and the
// validation the panel applies to untrusted agent input (5.4).
//
// The agent may only send host_info and metrics; the panel may only reply
// with registered and error. There is no command message in either direction.
package protocol

// Message type names.
const (
	TypeHostInfo   = "host_info"
	TypeMetrics    = "metrics"
	TypeRegistered = "registered"
	TypeError      = "error"
)

// Message size limits. The panel accepts larger messages than the agent
// because metrics carry per-interface counters.
const (
	// MaxAgentRead is the largest panel message the agent accepts.
	MaxAgentRead = 4 << 10
	// MaxPanelRead is the largest agent message the panel accepts.
	MaxPanelRead = 64 << 10
)

// MaxInterfaces is the largest number of interfaces one metrics message may
// carry. The agent caps its own report at this many.
const MaxInterfaces = 64

// Error codes the panel may send in an ErrorMessage.
const (
	CodeUnauthorized   = "unauthorized"
	CodeInvalidMessage = "invalid_message"
	CodeRateLimited    = "rate_limited"
	CodeReplaced       = "replaced"
	CodeServerError    = "server_error"
)

// AgentMessage is a message the agent sends to the panel.
type AgentMessage interface{ isAgentMessage() }

// PanelMessage is a message the panel sends to the agent.
type PanelMessage interface{ isPanelMessage() }

// HostInfo describes the monitored host. The agent sends it once after the
// connection is established and again whenever a field changes.
type HostInfo struct {
	Type            string `json:"type"`
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	Kernel          string `json:"kernel"`
	Arch            string `json:"arch"`
	CPUModel        string `json:"cpu_model"`
	CPUCores        int    `json:"cpu_cores"`
	AgentVersion    string `json:"agent_version"`
	// IntervalMS is the reporting interval the agent is configured with.
	IntervalMS int `json:"interval_ms"`
}

func (*HostInfo) isAgentMessage() {}

// NetCounters holds the cumulative byte counters of one interface.
type NetCounters struct {
	RX uint64 `json:"rx"`
	TX uint64 `json:"tx"`
}

// Metrics is one sample. Every optional field is omitted when its collector
// timed out or the platform does not support it; the panel treats an omitted
// field as "no data this cycle", never as zero.
type Metrics struct {
	Type string `json:"type"`
	// TS is the agent's wall clock in Unix seconds. It is advisory only:
	// samples are stored with the panel's receive time.
	TS int64 `json:"ts"`
	// MonoMS is the agent's monotonic clock in milliseconds since start,
	// used for rate calculation (REQUIREMENTS 3.1.4).
	MonoMS     int64                  `json:"mono_ms"`
	CPUPercent *float64               `json:"cpu_percent,omitempty"`
	MemTotal   *uint64                `json:"mem_total,omitempty"`
	MemUsed    *uint64                `json:"mem_used,omitempty"`
	SwapTotal  *uint64                `json:"swap_total,omitempty"`
	SwapUsed   *uint64                `json:"swap_used,omitempty"`
	Load1      *float64               `json:"load1,omitempty"`
	Load5      *float64               `json:"load5,omitempty"`
	Load15     *float64               `json:"load15,omitempty"`
	DiskTotal  *uint64                `json:"disk_total,omitempty"`
	DiskUsed   *uint64                `json:"disk_used,omitempty"`
	DiskFree   *uint64                `json:"disk_free,omitempty"`
	Net        map[string]NetCounters `json:"net,omitempty"`
	Uptime     *uint64                `json:"uptime,omitempty"`
}

func (*Metrics) isAgentMessage() {}

// Registered carries the long-lived credential created when an agent
// connects with a one-time enrollment token (REQUIREMENTS 5.2).
type Registered struct {
	Type    string `json:"type"`
	AgentID string `json:"agent_id"`
	Secret  string `json:"secret"`
}

func (*Registered) isPanelMessage() {}

// ErrorMessage tells the agent why the panel rejects it or closes the
// connection. Message is English text for the agent's log only; the agent
// never acts on its content.
type ErrorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (*ErrorMessage) isPanelMessage() {}

// Authorization header values (REQUIREMENTS 5.2). The two kinds of
// credential carry a prefix so that the panel dispatches on it instead of
// guessing from the shape of the value.
const (
	authScheme = "Bearer "
	// KindEnroll marks a one-time enrollment token.
	KindEnroll = "enroll:"
	// KindAgent marks a long-lived "<agent_id>.<secret>" credential.
	KindAgent = "agent:"
)

// EnrollAuthorization returns the header value for a one-time token.
func EnrollAuthorization(token string) string { return authScheme + KindEnroll + token }

// AgentAuthorization returns the header value for a long-lived credential.
func AgentAuthorization(credential string) string { return authScheme + KindAgent + credential }
