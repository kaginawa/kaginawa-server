package kaginawa

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
)

const customIDPlaceholder = "-"

// DynamoDB implements DB interface.
type DynamoDB struct {
	instance      *dynamodb.DynamoDB
	encoder       *dynamodbattribute.Encoder
	decoder       *dynamodbattribute.Decoder
	keysTable     string
	serversTable  string
	nodesTable    string
	logsTable     string
	customIDIndex string
	logsTTLDays   int
}

// NewDynamoDB will creates AWS DynamoDB instance that implements DB interface.
func NewDynamoDB() (*DynamoDB, error) {
	s := session.Must(session.NewSession(&aws.Config{Region: aws.String(os.Getenv("AWS_DEFAULT_REGION"))}))
	d := dynamodb.New(s)
	ep := os.Getenv("DYNAMO_ENDPOINT")
	if len(ep) > 0 {
		d.Endpoint = ep
	}
	db := &DynamoDB{
		instance: d,
		encoder: dynamodbattribute.NewEncoder(func(encoder *dynamodbattribute.Encoder) {
			encoder.SupportJSONTags = false // disable json tag (default is true)
		}),
		decoder: dynamodbattribute.NewDecoder(func(decoder *dynamodbattribute.Decoder) {
			decoder.SupportJSONTags = false // disable json tag (default is true)
		}),
	}
	db.keysTable = os.Getenv("DYNAMO_KEYS")
	db.serversTable = os.Getenv("DYNAMO_SERVERS")
	db.nodesTable = os.Getenv("DYNAMO_NODES")
	db.logsTable = os.Getenv("DYNAMO_LOGS")
	db.customIDIndex = os.Getenv("DYNAMO_CUSTOM_IDS")
	if len(db.keysTable) == 0 {
		return nil, errors.New("missing env var: DYNAMO_KEYS")
	}
	if len(db.serversTable) == 0 {
		return nil, errors.New("missing env var: DYNAMO_SERVERS")
	}
	if len(db.nodesTable) == 0 {
		return nil, errors.New("missing env var: DYNAMO_NODES")
	}
	if len(db.logsTable) == 0 {
		return nil, errors.New("missing env var: DYNAMO_LOGS")
	}
	if len(db.customIDIndex) == 0 {
		return nil, errors.New("missing env var: DYNAMO_CUSTOM_IDS")
	}
	ttlStr := os.Getenv("DYNAMO_TTL_DAYS")
	if len(ttlStr) > 0 {
		ttl, err := strconv.Atoi(ttlStr)
		if err != nil || ttl < 0 {
			return nil, fmt.Errorf("invalid env var: DYNAMO_TTL_DAYS = %s", ttlStr)
		}
		db.logsTTLDays = ttl
	}
	return db, nil
}

// ValidateAPIKey validates API key. Results are ok, label and error.
func (db DynamoDB) ValidateAPIKey(key string) (bool, string, error) {
	// Check cache first
	if v, ok := KnownAPIKeys.Load(key); ok {
		return ok, v.(string), nil
	}

	// Retrieve from database
	hash, err := db.encoder.Encode(struct{ Key string }{key})
	if err != nil {
		return false, "", fmt.Errorf("invalid key: %v", err)
	}
	item, err := db.instance.GetItem(&dynamodb.GetItemInput{TableName: aws.String(db.keysTable), Key: hash.M})
	if err != nil || item == nil {
		return false, "", err
	}
	var apiKey APIKey
	if err := db.decoder.Decode(&dynamodb.AttributeValue{M: item.Item}, &apiKey); err != nil {
		return false, "", fmt.Errorf("failed to unmarshal record: %w", err)
	}

	// Cache and return
	KnownAPIKeys.Store(apiKey.Key, apiKey.Label)
	return true, apiKey.Label, nil
}

// ValidateAdminAPIKey validates API key for admin privilege only. Results are ok, label and error.
func (db DynamoDB) ValidateAdminAPIKey(key string) (bool, string, error) {
	// Check cache first
	if v, ok := KnownAdminAPIKeys.Load(key); ok {
		return ok, v.(string), nil
	}

	// Retrieve from database
	hash, err := db.encoder.Encode(struct{ Key string }{key})
	if err != nil {
		return false, "", fmt.Errorf("invalid key: %v", err)
	}
	item, err := db.instance.GetItem(&dynamodb.GetItemInput{TableName: aws.String(db.keysTable), Key: hash.M})
	if err != nil || item == nil {
		return false, "", err
	}
	var apiKey APIKey
	if err := db.decoder.Decode(&dynamodb.AttributeValue{M: item.Item}, &apiKey); err != nil {
		return false, "", fmt.Errorf("failed to unmarshal record: %w", err)
	}
	if !apiKey.Admin {
		return false, "", nil
	}

	// Cache and return
	KnownAdminAPIKeys.Store(apiKey.Key, apiKey.Label)
	return true, apiKey.Label, nil
}

// ListAPIKeys scans all api keys.
func (db DynamoDB) ListAPIKeys() ([]APIKey, error) {
	var records []APIKey
	if err := db.instance.ScanPages(&dynamodb.ScanInput{
		TableName: aws.String(db.keysTable),
	}, func(output *dynamodb.ScanOutput, lastPage bool) bool {
		for _, item := range output.Items {
			var record APIKey
			if err := db.decoder.Decode(&dynamodb.AttributeValue{M: item}, &record); err != nil {
				log.Printf("skipping error item: %v", err)
				continue
			}
			records = append(records, record)
			KnownAPIKeys.Store(record.Key, record.Label) // update cache
			if record.Admin {
				KnownAdminAPIKeys.Store(record.Key, record.Label) // update cache
			}
		}
		return !lastPage
	}); err != nil {
		return nil, err
	}
	return records, nil
}

// PutAPIKey puts an api key.
func (db DynamoDB) PutAPIKey(apiKey APIKey) error {
	item, err := db.encoder.Encode(apiKey)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}
	_, err = db.instance.PutItem(&dynamodb.PutItemInput{TableName: aws.String(db.keysTable), Item: item.M})
	return err
}

// ListSSHServers scans all ssh servers.
func (db DynamoDB) ListSSHServers() ([]SSHServer, error) {
	var records []SSHServer
	if err := db.instance.ScanPages(&dynamodb.ScanInput{
		TableName: aws.String(db.serversTable),
	}, func(output *dynamodb.ScanOutput, lastPage bool) bool {
		for _, item := range output.Items {
			var record SSHServer
			if err := db.decoder.Decode(&dynamodb.AttributeValue{M: item}, &record); err != nil {
				log.Printf("skipping error item: %v", err)
				continue
			}
			records = append(records, record)
		}
		return !lastPage
	}); err != nil {
		return nil, err
	}
	SSHServers = records // update cache
	return records, nil
}

// PutSSHServer puts a ssh server entry.
func (db DynamoDB) PutSSHServer(server SSHServer) error {
	item, err := db.encoder.Encode(server)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}
	_, err = db.instance.PutItem(&dynamodb.PutItemInput{TableName: aws.String(db.serversTable), Item: item.M})
	return err
}

// PutReport puts a report.
func (db DynamoDB) PutReport(report Report) error {
	if len(report.CustomID) == 0 {
		report.CustomID = customIDPlaceholder
	}
	if db.logsTTLDays > 0 {
		report.TTL = time.Now().UTC().AddDate(0, 0, db.logsTTLDays)
	}
	item, err := db.encoder.Encode(report)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}
	_, err = db.instance.BatchWriteItem(&dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]*dynamodb.WriteRequest{
			db.nodesTable: {&dynamodb.WriteRequest{PutRequest: &dynamodb.PutRequest{Item: item.M}}},
			db.logsTable:  {&dynamodb.WriteRequest{PutRequest: &dynamodb.PutRequest{Item: item.M}}},
		},
	})
	return err
}

// CountReports counts number of reports.
func (db DynamoDB) CountReports() (int, error) {
	projection := expression.NamesList(expression.Name("ID"))
	expr, err := expression.NewBuilder().WithProjection(projection).Build()
	if err != nil {
		return 0, fmt.Errorf("failed to build projection expression: %w", err)
	}
	var count int
	err = db.instance.ScanPages(&dynamodb.ScanInput{
		TableName:                aws.String(db.nodesTable),
		ProjectionExpression:     expr.Projection(),
		ExpressionAttributeNames: expr.Names(),
	}, func(output *dynamodb.ScanOutput, lastPage bool) bool {
		count += len(output.Items)
		return !lastPage
	})
	return count, err
}

// ListReports scans all reports.
func (db DynamoDB) ListReports(skip, limit int) ([]Report, error) {
	var records []Report
	var count int
	if err := db.instance.ScanPages(&dynamodb.ScanInput{
		TableName: aws.String(db.nodesTable),
	}, func(output *dynamodb.ScanOutput, lastPage bool) bool {
		for _, item := range output.Items {
			count++
			if count <= skip {
				continue
			}
			if len(records) >= limit {
				return false
			}
			var record Report
			if err := db.decoder.Decode(&dynamodb.AttributeValue{M: item}, &record); err != nil {
				log.Printf("skipping error item: %v", err)
				continue
			}
			records = append(records, record)
		}
		return !lastPage
	}); err != nil {
		return nil, err
	}
	return records, nil
}

// GetReportByID queries a report by id.
func (db DynamoDB) GetReportByID(id string) (*Report, error) {
	hash, err := dynamodbattribute.MarshalMap(struct{ ID string }{id})
	if err != nil {
		return nil, fmt.Errorf("invalid ID: %v", err)
	}
	item, err := db.instance.GetItem(&dynamodb.GetItemInput{TableName: aws.String(db.nodesTable), Key: hash})
	if err != nil || item == nil {
		return nil, err
	}
	var report Report
	if err := db.decoder.Decode(&dynamodb.AttributeValue{M: item.Item}, &report); err != nil {
		return nil, fmt.Errorf("failed to unmarshal record: %w", err)
	}
	return &report, nil
}

// GetReportByCustomID queries a report by custom id.
func (db DynamoDB) GetReportByCustomID(customID string) ([]Report, error) {
	if len(customID) == 0 {
		customID = customIDPlaceholder
	}
	keyCondition := expression.Key("CustomID").Equal(expression.Value(customID))
	expr, err := expression.NewBuilder().WithKeyCondition(keyCondition).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build key condition expression: %w", err)
	}
	var records []Report
	if err := db.instance.QueryPages(&dynamodb.QueryInput{
		TableName:              aws.String(db.nodesTable),
		IndexName:              aws.String(db.customIDIndex),
		KeyConditionExpression: expr.KeyCondition(),
	}, func(output *dynamodb.QueryOutput, lastPage bool) bool {
		for _, item := range output.Items {
			var record Report
			if err := db.decoder.Decode(&dynamodb.AttributeValue{M: item}, &record); err != nil {
				log.Printf("skipping error item: %v", err)
				continue
			}
			records = append(records, record)
		}
		return !lastPage
	}); err != nil {
		return nil, err
	}
	return records, nil
}
