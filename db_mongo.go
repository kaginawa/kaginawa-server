package kaginawa

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	keyCollection    = "keys"
	serverCollection = "servers"
	nodeCollection   = "nodes"
	logCollection    = "logs"
)

var (
	t      = true
	upsert = &options.ReplaceOptions{Upsert: &t}
)

// MongoDB implements DB interface.
type MongoDB struct {
	client   *mongo.Client
	instance *mongo.Database
}

// NewMongoDB will creates MongoDB instance that implements DB interface.
func NewMongoDB(endpoint string) (*MongoDB, error) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(endpoint).SetRetryWrites(false))
	if err != nil {
		return &MongoDB{}, err
	}
	return &MongoDB{
		client:   client,
		instance: client.Database(endpoint[strings.LastIndex(endpoint, "/")+1:]),
	}, nil
}

// ValidateAPIKey implements same signature of the DB interface.
func (db *MongoDB) ValidateAPIKey(key string) (bool, string, error) {
	// Check cache first
	if v, ok := KnownAPIKeys.Load(key); ok {
		return ok, v.(string), nil
	}

	// Retrieve from database
	result := db.instance.Collection(keyCollection).FindOne(context.Background(), bson.M{"key": key})
	if result.Err() != nil {
		if result.Err() == mongo.ErrNoDocuments {
			return false, "", nil
		}
		return false, "", result.Err()
	}
	var apiKey APIKey
	if err := result.Decode(&apiKey); err != nil {
		return false, "", result.Err()
	}

	// Cache and return
	KnownAPIKeys.Store(apiKey.Key, apiKey.Label)
	return true, apiKey.Label, nil
}

// ValidateAdminAPIKey implements same signature of the DB interface.
func (db *MongoDB) ValidateAdminAPIKey(key string) (bool, string, error) {
	// Check cache first
	if v, ok := KnownAdminAPIKeys.Load(key); ok {
		return ok, v.(string), nil
	}

	// Retrieve from database
	result := db.instance.Collection(keyCollection).FindOne(context.Background(), bson.M{"key": key, "admin": true})
	if result.Err() != nil {
		if result.Err() == mongo.ErrNoDocuments {
			return false, "", nil
		}
		return false, "", result.Err()
	}
	var apiKey APIKey
	if err := result.Decode(&apiKey); err != nil {
		return false, "", result.Err()
	}

	// Cache and return
	KnownAdminAPIKeys.Store(apiKey.Key, apiKey.Label)
	return true, apiKey.Label, nil
}

// ListAPIKeys implements same signature of the DB interface.
func (db *MongoDB) ListAPIKeys() ([]APIKey, error) {
	cur, err := db.instance.Collection(keyCollection).Find(context.Background(), bson.D{})
	if err != nil {
		return nil, err
	}
	defer db.safeClose(cur)
	var apiKeys []APIKey
	for cur.Next(context.Background()) {
		var result APIKey
		if err := cur.Decode(&result); err != nil {
			return nil, err
		}
		apiKeys = append(apiKeys, result)
		KnownAPIKeys.Store(result.Key, result.Label) // update cache
		if result.Admin {
			KnownAdminAPIKeys.Store(result.Key, result.Label) // update cache
		}
	}
	return apiKeys, nil
}

// PutAPIKey implements same signature of the DB interface.
func (db *MongoDB) PutAPIKey(apiKey APIKey) error {
	raw, err := bson.Marshal(apiKey)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}
	key := bson.M{"key": apiKey.Key}
	if _, err := db.instance.Collection(keyCollection).ReplaceOne(context.Background(), key, raw, upsert); err != nil {
		return err
	}
	return nil
}

// ListSSHServers implements same signature of the DB interface.
func (db *MongoDB) ListSSHServers() ([]SSHServer, error) {
	cur, err := db.instance.Collection(serverCollection).Find(context.Background(), bson.D{})
	if err != nil {
		return nil, err
	}
	defer db.safeClose(cur)
	var servers []SSHServer
	for cur.Next(context.Background()) {
		var result SSHServer
		if err := cur.Decode(&result); err != nil {
			return nil, err
		}
		servers = append(servers, result)
	}
	SSHServers = servers // update cache
	return servers, nil
}

// GetSSHServerByHost implements same signature of the DB interface.
func (db *MongoDB) GetSSHServerByHost(host string) (*SSHServer, error) {
	result := db.instance.Collection(serverCollection).FindOne(context.Background(), bson.M{"host": host})
	if result.Err() != nil {
		if result.Err() == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, result.Err()
	}
	var server SSHServer
	if err := result.Decode(&server); err != nil {
		return nil, err
	}
	return &server, nil
}

// PutSSHServer implements same signature of the DB interface.
func (db *MongoDB) PutSSHServer(server SSHServer) error {
	raw, err := bson.Marshal(server)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}
	key := bson.M{"host": server.Host, "port": server.Port}
	if _, err := db.instance.Collection(serverCollection).ReplaceOne(context.Background(), key, raw, upsert); err != nil {
		return err
	}
	return nil
}

// PutReport implements same signature of the DB interface.
func (db *MongoDB) PutReport(report Report) error {
	raw, err := bson.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}
	key := bson.M{"id": report.ID}
	if _, err = db.instance.Collection(nodeCollection).ReplaceOne(context.Background(), key, raw, upsert); err != nil {
		return err
	}
	if _, err = db.instance.Collection(logCollection).InsertOne(context.Background(), raw); err != nil {
		return err
	}
	return nil
}

// CountReports counts number of records in node table.
func (db *MongoDB) CountReports() (int, error) {
	n, err := db.instance.Collection(nodeCollection).CountDocuments(context.Background(), bson.D{})
	return int(n), err
}

// ListReports implements same signature of the DB interface.
func (db *MongoDB) ListReports(skip, limit, minutes int, projection Projection) ([]Report, error) {
	opts := &options.FindOptions{Sort: bson.M{"custom_id": 1}, Skip: int64p(skip), Limit: int64p(limit)}
	opts = db.applyProjection(opts, projection)
	filter := bson.M{}
	if minutes > 0 {
		timestamp := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
		filter = bson.M{"server_time": bson.M{"$gte": timestamp.Unix()}}
	}
	cur, err := db.instance.Collection(nodeCollection).Find(context.Background(), filter, opts)
	if err != nil {
		return nil, err
	}
	defer db.safeClose(cur)
	return db.decodeReports(cur)
}

// GetReportByID implements same signature of the DB interface.
func (db *MongoDB) GetReportByID(id string) (*Report, error) {
	result := db.instance.Collection(nodeCollection).FindOne(context.Background(), bson.M{"id": id})
	if result.Err() != nil {
		if result.Err() == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, result.Err()
	}
	var report Report
	if err := result.Decode(&report); err != nil {
		return nil, err
	}
	return &report, nil
}

// ListReportsByCustomID implements same signature of the DB interface.
func (db *MongoDB) ListReportsByCustomID(customID string, minutes int, projection Projection) ([]Report, error) {
	opts := &options.FindOptions{Sort: bson.M{"hostname": 1}}
	filter := bson.M{"custom_id": customID}
	if minutes > 0 {
		timestamp := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
		filter["server_time"] = bson.M{"$gte": timestamp.Unix()}
	}
	opts = db.applyProjection(opts, projection)
	cur, err := db.instance.Collection(nodeCollection).Find(context.Background(), filter, opts)
	if err != nil {
		return nil, err
	}
	defer db.safeClose(cur)
	return db.decodeReports(cur)
}

// ListHistory implements same signature of the DB interface.
func (db *MongoDB) ListHistory(id string, begin time.Time, end time.Time, projection Projection) ([]Report, error) {
	opts := db.applyProjection(&options.FindOptions{Sort: bson.M{"server_time": 1}}, projection)
	filter := bson.M{"id": id, "server_time": bson.M{"$gte": begin.Unix(), "$lte": end.Unix()}}
	cur, err := db.instance.Collection(logCollection).Find(context.Background(), filter, opts)
	if err != nil {
		return nil, err
	}
	defer db.safeClose(cur)
	return db.decodeReports(cur)
}

func (db *MongoDB) applyProjection(opts *options.FindOptions, projection Projection) *options.FindOptions {
	switch projection {
	case IDAttributes:
		opts.Projection = bson.D{
			{"id", 1},
			{"custom_id", 1},
			{"server_time", 1},
			{"success", 1},
		}
	case ListViewAttributes:
		opts.Projection = bson.D{
			{"id", 1},
			{"custom_id", 1},
			{"hostname", 1},
			{"server_time", 1},
			{"ssh_server_host", 1},
			{"ssh_remote_port", 1},
			{"ip_global", 1},
			{"host_global", 1},
			{"ip4_local", 1},
			{"seq", 1},
			{"agent_version", 1},
			{"success", 1},
			{"errors", 1},
		}
	case MeasurementAttributes:
		opts.Projection = bson.D{
			{"id", 1},
			{"custom_id", 1},
			{"hostname", 1},
			{"server_time", 1},
			{"seq", 1},
			{"rtt_ms", 1},
			{"upload_bps", 1},
			{"download_bps", 1},
			{"success", 1},
		}
	}
	return opts
}

func (db *MongoDB) safeClose(cur *mongo.Cursor) {
	if err := cur.Close(context.Background()); err != nil {
		log.Printf("failed to close cursor: %v", err)
	}
}

func (db *MongoDB) decodeReports(cur *mongo.Cursor) ([]Report, error) {
	reports := make([]Report, 0)
	for cur.Next(context.Background()) {
		var result Report
		if err := cur.Decode(&result); err != nil {
			return nil, err
		}
		reports = append(reports, result)
	}
	return reports, nil
}

func int64p(n int) *int64 {
	n64 := int64(n)
	return &n64
}
