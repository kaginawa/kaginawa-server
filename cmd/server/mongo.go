package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

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
	knownAPIKeys sync.Map
	sshServers   []sshServer
	t            = true
	upsert       = &options.ReplaceOptions{Upsert: &t}
)

type db struct {
	client   *mongo.Client
	instance *mongo.Database
}

type apiKey struct {
	Key   string `bson:"key"`
	Label string `bson:"label"`
}

type sshServer struct {
	Host     string `bson:"host"`
	Port     int    `bson:"port"`
	User     string `bson:"user"`
	Key      string `bson:"key"`
	Password string `bson:"password"`
}

func newDB(endpoint string) (*db, error) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(endpoint).SetRetryWrites(false))
	if err != nil {
		return nil, err
	}
	return &db{
		client:   client,
		instance: client.Database(endpoint[strings.LastIndex(endpoint, "/")+1:]),
	}, nil
}

// validateAPIKey validates API key. Results are ok, label and error.
func (db *db) validateAPIKey(key string) (bool, string, error) {
	// Check cache first
	if v, ok := knownAPIKeys.Load(key); ok {
		return ok, v.(string), nil
	}

	// Retrieve from database
	result := db.instance.Collection(keyCollection).FindOne(context.Background(), bson.M{"key": key})
	if result.Err() != nil {
		return false, "", result.Err()
	}
	var apiKey apiKey
	if err := result.Decode(&apiKey); err != nil {
		return false, "", result.Err()
	}

	// Cache and return
	knownAPIKeys.Store(apiKey.Key, apiKey.Label)
	return true, apiKey.Label, nil
}

func (db *db) listAPIKeys() ([]apiKey, error) {
	cur, err := db.instance.Collection(keyCollection).Find(context.Background(), bson.D{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cur.Close(context.Background()); err != nil {
			log.Printf("failed to close cursor: %v", err)
		}
	}()
	var apiKeys []apiKey
	for cur.Next(context.Background()) {
		var result apiKey
		if err := cur.Decode(&result); err != nil {
			return nil, err
		}
		apiKeys = append(apiKeys, result)
		knownAPIKeys.Store(result.Key, result.Label) // update cache
	}
	return apiKeys, nil
}

func (db *db) putAPIKey(apiKey apiKey) error {
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

func (db *db) listSSHServers() ([]sshServer, error) {
	cur, err := db.instance.Collection(serverCollection).Find(context.Background(), bson.D{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cur.Close(context.Background()); err != nil {
			log.Printf("failed to close cursor: %v", err)
		}
	}()
	var servers []sshServer
	for cur.Next(context.Background()) {
		var result sshServer
		if err := cur.Decode(&result); err != nil {
			return nil, err
		}
		servers = append(servers, result)
	}
	sshServers = servers // update cache
	return servers, nil
}

func (db *db) putSSHServer(server sshServer) error {
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

func (db *db) putReport(report report) error {
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

func (db *db) listReports() ([]report, error) {
	sort := &options.FindOptions{Sort: bson.M{"hostname": 1}}
	cur, err := db.instance.Collection(nodeCollection).Find(context.Background(), bson.D{}, sort)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cur.Close(context.Background()); err != nil {
			log.Printf("failed to close cursor: %v", err)
		}
	}()
	var reports []report
	for cur.Next(context.Background()) {
		var result report
		if err := cur.Decode(&result); err != nil {
			return nil, err
		}
		reports = append(reports, result)
	}
	return reports, nil
}

func (db *db) getReportByID(id string) (*report, error) {
	result := db.instance.Collection(nodeCollection).FindOne(context.Background(), bson.M{"id": id})
	if result.Err() != nil {
		return nil, result.Err()
	}
	var report report
	if err := result.Decode(&report); err != nil {
		return nil, err
	}
	return &report, nil
}

func (db *db) getReportByCustomID(customID string) (*report, error) {
	result := db.instance.Collection(nodeCollection).FindOne(context.Background(), bson.M{"custom_id": customID})
	if result.Err() != nil {
		return nil, result.Err()
	}
	var report report
	if err := result.Decode(&report); err != nil {
		return nil, err
	}
	return &report, nil
}
