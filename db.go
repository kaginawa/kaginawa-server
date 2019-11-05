package kaginawa

import (
	"fmt"
	"sync"
)

var (
	// KnownAPIKeys caches known api keys on memory.
	KnownAPIKeys sync.Map
	// KnownAdminAPIKeys caches known admin api keys on memory.
	KnownAdminAPIKeys sync.Map
	// SSHServers caches list of ssh servers on memory.
	SSHServers []SSHServer
)

// DB implements database operations.
type DB interface {
	// ValidateAPIKey validates API key. Results are ok, label and error.
	ValidateAPIKey(key string) (bool, string, error)
	// ValidateAdminAPIKey validates API key for admin privilege only. Results are ok, label and error.
	ValidateAdminAPIKey(key string) (bool, string, error)
	// ListAPIKeys scans all api keys.
	ListAPIKeys() ([]APIKey, error)
	// PutAPIKey puts an api key.
	PutAPIKey(apiKey APIKey) error
	// ListSSHServers scans all ssh servers.
	ListSSHServers() ([]SSHServer, error)
	// PutSSHServer puts a ssh server entry.
	PutSSHServer(server SSHServer) error
	// PutReport puts a report.
	PutReport(report Report) error
	// ListReports scans all reports.
	ListReports() ([]Report, error)
	// GetReportByID queries a report by id.
	GetReportByID(id string) (*Report, error)
	// GetReportByCustomID queries a report by custom id.
	GetReportByCustomID(customID string) ([]Report, error)
}

// APIKey defines database item of an api key.
type APIKey struct {
	Key   string `bson:"key"`
	Label string `bson:"label"`
	Admin bool   `bson:"admin"`
}

// SSHServer defines database item of a ssh server.
type SSHServer struct {
	Host     string `bson:"host"`
	Port     int    `bson:"port"`
	User     string `bson:"user"`
	Key      string `bson:"key"`
	Password string `bson:"password"`
}

// Report defines all of Report attributes
type Report struct {
	// Kagiwana shared fields
	ID             string   `json:"id" bson:"id"`                                       // MAC address
	Runtime        string   `json:"runtime" bson:"runtime"`                             // OS and arch
	Success        bool     `json:"success" bson:"success"`                             // Equals len(Errors) == 0
	Sequence       int      `json:"seq" bson:"seq"`                                     // Report sequence number
	DeviceTime     int64    `json:"device_time" bson:"device_time"`                     // Device time (UTC)
	BootTime       int64    `json:"boot_time" bson:"boot_time"`                         // Device boot time (UTC)
	GenMillis      int64    `json:"gen_ms" bson:"gen_ms"`                               // Generation time milliseconds
	AgentVersion   string   `json:"agent_version" bson:"agent_version"`                 // Agent version
	CustomID       string   `json:"custom_id,omitempty" bson:"custom_id"`               // User specified ID
	SSHServerHost  string   `json:"ssh_server_host,omitempty" bson:"ssh_server_host"`   // Connected SSH server host
	SSHRemotePort  int      `json:"ssh_remote_port,omitempty" bson:"ssh_remote_port"`   // Connected SSH remote port
	SSHConnectTime int64    `json:"ssh_connect_time,omitempty" bson:"ssh_connect_time"` // Connected time of the SSH
	Adapter        string   `json:"adapter,omitempty" bson:"adapter"`                   // Name of network adapter
	LocalIPv4      string   `json:"ip4_local,omitempty" bson:"ip4_local"`               // Local IPv6 address
	LocalIPv6      string   `json:"ip6_local,omitempty" bson:"ip6_local"`               // Local IPv6 address
	Hostname       string   `json:"hostname,omitempty" bson:"hostname"`                 // OS Hostname
	RTTMills       int64    `json:"rtt_ms,omitempty" bson:"rtt_ms"`                     // Round trip time milliseconds
	UploadKBPS     int64    `json:"upload_bps,omitempty" bson:"upload_bps"`             // Upload throughput bps
	DownloadKBPS   int64    `json:"download_bps,omitempty" bson:"download_bps"`         // Download throughput bps
	Errors         []string `json:"errors,omitempty" bson:"errors"`                     // List of errors
	Payload        string   `json:"payload,omitempty" bson:"payload"`                   // Custom content
	PayloadCmd     string   `json:"payload_cmd,omitempty" bson:"payload_cmd"`           // Executed payload command

	// Server-side injected fields
	GlobalIP   string `json:"ip_global" bson:"ip_global"`     // Global IP address
	GlobalHost string `json:"host_global" bson:"host_global"` // Reverse lookup result for global IP address
	ServerTime int64  `json:"server_time" bson:"server_time"` // Server-side consumed UTC time
}

// DownloadMBPS formats download throughput as Mbps.
func (r Report) DownloadMBPS() string {
	return fmt.Sprintf("%.1f", float64(r.DownloadKBPS)/1024)
}

// UploadMBPS formats upload throughput as Mbps.
func (r Report) UploadMBPS() string {
	return fmt.Sprintf("%.1f", float64(r.UploadKBPS)/1024)
}