package kaginawa

import (
	"sync"
	"time"
)

// MemDB implements in-memory store for testing.
type MemDB struct {
	keys         map[string]APIKey
	servers      map[string]SSHServer
	nodes        map[string]Report
	logs         []Report
	keysMutex    sync.RWMutex
	serversMutex sync.RWMutex
	nodesMutex   sync.RWMutex
	logsMutex    sync.RWMutex
}

// NewMemDB will creates in-memory DB instance that implements DB interface.
func NewMemDB() *MemDB {
	return &MemDB{
		keys:    make(map[string]APIKey),
		servers: make(map[string]SSHServer),
		nodes:   make(map[string]Report),
		logs:    make([]Report, 0),
	}
}

// ValidateAPIKey implements same signature of the DB interface.
func (db *MemDB) ValidateAPIKey(key string) (bool, string, error) {
	db.keysMutex.RLock()
	defer db.keysMutex.RUnlock()
	for _, v := range db.keys {
		if v.Key == key {
			return true, v.Label, nil
		}
	}
	return false, "", nil
}

// ValidateAdminAPIKey implements same signature of the DB interface.
func (db *MemDB) ValidateAdminAPIKey(key string) (bool, string, error) {
	db.keysMutex.RLock()
	defer db.keysMutex.RUnlock()
	for _, v := range db.keys {
		if !v.Admin {
			continue
		}
		if v.Key == key {
			return true, v.Label, nil
		}
	}
	return false, "", nil
}

// ListAPIKeys implements same signature of the DB interface.
func (db *MemDB) ListAPIKeys() ([]APIKey, error) {
	db.keysMutex.RLock()
	defer db.keysMutex.RUnlock()
	slice := make([]APIKey, 0, len(db.keys))
	for _, v := range db.keys {
		slice = append(slice, v)
	}
	return slice, nil
}

// PutAPIKey implements same signature of the DB interface.
func (db *MemDB) PutAPIKey(apiKey APIKey) error {
	db.keysMutex.Lock()
	defer db.keysMutex.Unlock()
	db.keys[apiKey.Key] = apiKey
	return nil
}

// ListSSHServers implements same signature of the DB interface.
func (db *MemDB) ListSSHServers() ([]SSHServer, error) {
	db.serversMutex.RLock()
	defer db.serversMutex.RUnlock()
	slice := make([]SSHServer, 0, len(db.servers))
	for _, v := range db.servers {
		slice = append(slice, v)
	}
	return slice, nil
}

// GetSSHServerByHost implements same signature of the DB interface.
func (db *MemDB) GetSSHServerByHost(host string) (*SSHServer, error) {
	db.serversMutex.RLock()
	defer db.serversMutex.RUnlock()
	for k, v := range db.servers {
		if k == host {
			return &v, nil
		}
	}
	return nil, nil
}

// PutSSHServer implements same signature of the DB interface.
func (db *MemDB) PutSSHServer(server SSHServer) error {
	db.serversMutex.Lock()
	defer db.serversMutex.Unlock()
	db.servers[server.Host] = server
	return nil
}

// PutReport implements same signature of the DB interface.
func (db *MemDB) PutReport(report Report) error {
	db.nodesMutex.Lock()
	db.logsMutex.Lock()
	defer db.nodesMutex.Unlock()
	defer db.logsMutex.Unlock()
	db.nodes[report.ID] = report
	db.logs = append(db.logs, report)
	return nil
}

// CountReports implements same signature of the DB interface.
func (db *MemDB) CountReports() (int, error) {
	db.nodesMutex.RLock()
	defer db.nodesMutex.RUnlock()
	return len(db.nodes), nil
}

// ListReports implements same signature of the DB interface.
func (db *MemDB) ListReports(skip, limit, _ int, _ Projection) ([]Report, error) {
	db.nodesMutex.RLock()
	defer db.nodesMutex.RUnlock()
	var slice []Report
	var count int
	for _, v := range db.nodes {
		count++
		if count <= skip {
			continue
		}
		if limit > 0 && len(slice) >= limit {
			break
		}
		slice = append(slice, v)
	}
	return slice, nil
}

// CountAndListReports implements same signature of the DB interface.
func (db *MemDB) CountAndListReports(skip, limit, _ int, _ Projection) ([]Report, int, error) {
	db.nodesMutex.RLock()
	defer db.nodesMutex.RUnlock()
	var slice []Report
	var count int
	for _, v := range db.nodes {
		count++
		if count <= skip {
			continue
		}
		if limit > 0 && len(slice) >= limit {
			break
		}
		slice = append(slice, v)
	}
	return slice, len(db.nodes), nil
}

// GetReportByID implements same signature of the DB interface.
func (db *MemDB) GetReportByID(id string) (*Report, error) {
	db.nodesMutex.RLock()
	defer db.nodesMutex.RUnlock()
	report, ok := db.nodes[id]
	if ok {
		return &report, nil
	}
	return nil, nil
}

// ListReportsByCustomID implements same signature of the DB interface.
func (db *MemDB) ListReportsByCustomID(customID string, _ int, _ Projection) ([]Report, error) {
	db.nodesMutex.RLock()
	defer db.nodesMutex.RUnlock()
	var slice []Report
	for _, v := range db.nodes {
		if v.CustomID == customID {
			slice = append(slice, v)
		}
	}
	return slice, nil
}

// ListHistory implements same signature of the DB interface.
func (db *MemDB) ListHistory(id string, begin time.Time, end time.Time, _ Projection) ([]Report, error) {
	db.nodesMutex.RLock()
	defer db.nodesMutex.RUnlock()
	var slice []Report
	for _, l := range db.logs {
		t := time.Unix(l.ServerTime, 0)
		if l.ID == id && t.Before(begin) && t.After(end) {
			slice = append(slice, l)
		}
	}
	return slice, nil
}
