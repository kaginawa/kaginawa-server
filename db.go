package kaginawa

import (
	"fmt"
	"sync"
	"time"
)

var (
	// KnownAPIKeys caches known api keys on memory.
	KnownAPIKeys sync.Map
	// KnownAdminAPIKeys caches known admin api keys on memory.
	KnownAdminAPIKeys sync.Map
	// SSHServers caches list of ssh servers on memory.
	SSHServers []SSHServer
)

// Projection defines parameter patterns of projection attributes
type Projection int

const (
	// AllAttributes defines projection pattern of all attributes
	AllAttributes Projection = iota
	// IDAttributes defines projection pattern of ID attributes
	IDAttributes
	// ListViewAttributes defines projection pattern of list page attributes
	ListViewAttributes
	// MeasurementAttributes defines projection pattern of measurement attributes
	MeasurementAttributes
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
	// GetSSHServerByHost queries a server by host.
	GetSSHServerByHost(host string) (*SSHServer, error)
	// PutSSHServer puts a ssh server entry.
	PutSSHServer(server SSHServer) error
	// PutReport puts a report.
	PutReport(report Report) error
	// CountReports counts number of reports.
	CountReports() (int, error)
	// ListReports scans list of reports.
	ListReports(skip, limit, minutes int, projection Projection) ([]Report, error)
	// CountAndListReports scans list of reports with total count.
	CountAndListReports(skip, limit, minutes int, projection Projection) ([]Report, int, error)
	// GetReportByID queries a report by id. Returns (nil, nil) if not found.
	GetReportByID(id string) (*Report, error)
	// ListReportsByCustomID queries list of reports by custom id.
	ListReportsByCustomID(customID string, minutes int, projection Projection) ([]Report, error)
	// DeleteReport deletes a report. Histories are preserved.
	DeleteReport(id string) error
	// ListHistory queries list of history.
	ListHistory(id string, begin time.Time, end time.Time, projection Projection) ([]Report, error)
	// GetUserSession gets a user session.
	GetUserSession(id string) (*UserSession, error)
	// PutUserSession puts a user session.
	PutUserSession(session UserSession) error
	// DeleteUserSession deletes a user session.
	DeleteUserSession(id string) error
}

// APIKey defines database item of an api key.
type APIKey struct {
	Key   string `bson:"key"`
	Label string `bson:"label"`
	Admin bool   `bson:"admin"`
}

// SSHServer defines database item of a ssh server.
type SSHServer struct {
	Host     string `json:"host" bson:"host"`
	Port     int    `json:"port" bson:"port"`
	User     string `json:"user" bson:"user"`
	Key      string `json:"key,omitempty" bson:"key"`
	Password string `json:"password,omitempty" bson:"password"`
}

// Addr formats address by host:port.
func (s SSHServer) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}
