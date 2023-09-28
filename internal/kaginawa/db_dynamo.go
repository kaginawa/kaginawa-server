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
	instance        *dynamodb.DynamoDB
	encoder         *dynamodbattribute.Encoder
	decoder         *dynamodbattribute.Decoder
	keysTable       string
	serversTable    string
	nodesTable      string
	logsTable       string
	sessionsTable   string
	customIDIndex   string
	logsTTLDays     int
	sessionsTTLDays int
}

// NewDynamoDB will create AWS DynamoDB instance that implements DB interface.
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
	db.sessionsTable = os.Getenv("DYNAMO_SESSIONS")
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
	if len(db.sessionsTable) == 0 {
		return nil, errors.New("missing env var: DYNAMO_SESSIONS")
	}
	if len(db.customIDIndex) == 0 {
		return nil, errors.New("missing env var: DYNAMO_CUSTOM_IDS")
	}
	if ttlStr := os.Getenv("DYNAMO_LOGS_TTL_DAYS"); len(ttlStr) > 0 {
		ttl, err := strconv.Atoi(ttlStr)
		if err != nil || ttl < 0 {
			return nil, fmt.Errorf("invalid env var: DYNAMO_LOGS_TTL_DAYS = %s", ttlStr)
		}
		db.logsTTLDays = ttl
	}
	if ttlStr := os.Getenv("DYNAMO_SESSIONS_TTL_DAYS"); len(ttlStr) > 0 {
		ttl, err := strconv.Atoi(ttlStr)
		if err != nil || ttl < 0 {
			return nil, fmt.Errorf("invalid env var: DYNAMO_SESSIONS_TTL_DAYS =%s", ttlStr)
		}
		db.sessionsTTLDays = ttl
	}
	return db, nil
}

// ValidateAPIKey implements same signature of the DB interface.
func (db *DynamoDB) ValidateAPIKey(key string) (bool, string, error) {
	if v, ok := KnownAPIKeys.Load(key); ok {
		return ok, v.(string), nil
	}
	apiKey, err := db.findAPIKey(key)
	if err != nil {
		return false, "", err
	}
	KnownAPIKeys.Store(apiKey.Key, apiKey.Label)
	return true, apiKey.Label, nil
}

// ValidateAdminAPIKey implements same signature of the DB interface.
func (db *DynamoDB) ValidateAdminAPIKey(key string) (bool, string, error) {
	if v, ok := KnownAdminAPIKeys.Load(key); ok {
		return ok, v.(string), nil
	}
	apiKey, err := db.findAPIKey(key)
	if err != nil {
		return false, "", err
	}
	if !apiKey.Admin {
		return false, "", nil
	}
	KnownAdminAPIKeys.Store(apiKey.Key, apiKey.Label)
	return true, apiKey.Label, nil
}

// ListAPIKeys implements same signature of the DB interface.
func (db *DynamoDB) ListAPIKeys() ([]APIKey, error) {
	var records []APIKey
	if err := db.instance.ScanPages(&dynamodb.ScanInput{
		TableName: &db.keysTable,
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

// PutAPIKey implements same signature of the DB interface.
func (db *DynamoDB) PutAPIKey(apiKey APIKey) error {
	item, err := db.encoder.Encode(apiKey)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}
	_, err = db.instance.PutItem(&dynamodb.PutItemInput{TableName: &db.keysTable, Item: item.M})
	return err
}

// ListSSHServers implements same signature of the DB interface.
func (db *DynamoDB) ListSSHServers() ([]SSHServer, error) {
	var records []SSHServer
	if err := db.instance.ScanPages(&dynamodb.ScanInput{
		TableName: &db.serversTable,
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

// GetSSHServerByHost implements same signature of the DB interface.
func (db *DynamoDB) GetSSHServerByHost(host string) (*SSHServer, error) {
	hash, err := db.encoder.Encode(struct{ Host string }{host})
	if err != nil {
		return nil, fmt.Errorf("invalid host: %v", err)
	}
	item, err := db.instance.GetItem(&dynamodb.GetItemInput{TableName: &db.serversTable, Key: hash.M})
	if err != nil {
		return nil, err
	}
	if item.Item == nil {
		return nil, nil
	}
	var server SSHServer
	if err := db.decoder.Decode(&dynamodb.AttributeValue{M: item.Item}, &server); err != nil {
		return nil, err
	}
	return &server, nil
}

// PutSSHServer implements same signature of the DB interface.
func (db *DynamoDB) PutSSHServer(server SSHServer) error {
	item, err := db.encoder.Encode(server)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}
	_, err = db.instance.PutItem(&dynamodb.PutItemInput{TableName: &db.serversTable, Item: item.M})
	return err
}

// PutReport implements same signature of the DB interface.
func (db *DynamoDB) PutReport(report Report) error {
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

// CountReports implements same signature of the DB interface.
func (db *DynamoDB) CountReports() (int, error) {
	var count int
	err := db.instance.ScanPages(&dynamodb.ScanInput{
		TableName: &db.nodesTable,
		Select:    aws.String(dynamodb.SelectCount),
	}, func(output *dynamodb.ScanOutput, lastPage bool) bool {
		count += int(*output.Count)
		return !lastPage
	})
	return count, err
}

// ListReports implements same signature of the DB interface.
// Set limit <= 0 to enable unlimited scans.
func (db *DynamoDB) ListReports(skip, limit, minutes int, projection Projection) ([]Report, error) {
	builder := expression.NewBuilder()
	if minutes > 0 {
		timestamp := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
		builder = builder.WithFilter(expression.Name("ServerTime").GreaterThanEqual(expression.Value(timestamp.Unix())))
	}
	expr, err := db.applyProjection(builder, projection).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}
	var records []Report
	var count int
	if err := db.instance.ScanPages(&dynamodb.ScanInput{
		TableName:                 &db.nodesTable,
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}, func(output *dynamodb.ScanOutput, lastPage bool) bool {
		for _, item := range output.Items {
			count++
			if count <= skip {
				continue
			}
			if limit > 0 && len(records) >= limit {
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

// CountAndListReports implements same signature of the DB interface.
func (db *DynamoDB) CountAndListReports(skip, limit, minutes int, projection Projection) ([]Report, int, error) {
	all, err := db.ListReports(0, 0, minutes, projection)
	if err != nil {
		return nil, -1, err
	}
	var reports []Report
	var pos int
	for _, v := range all {
		pos++
		if pos <= skip {
			continue
		}
		if limit > 0 && len(reports) >= limit {
			break
		}
		reports = append(reports, v)
	}
	return reports, len(all), nil
}

// GetReportByID implements same signature of the DB interface.
func (db *DynamoDB) GetReportByID(id string) (*Report, error) {
	hash, err := db.encoder.Encode(struct{ ID string }{id})
	if err != nil {
		return nil, fmt.Errorf("invalid report ID: %v", err)
	}
	item, err := db.instance.GetItem(&dynamodb.GetItemInput{TableName: &db.nodesTable, Key: hash.M})
	if err != nil {
		return nil, err
	}
	if item.Item == nil {
		return nil, nil
	}
	var report Report
	if err := db.decoder.Decode(&dynamodb.AttributeValue{M: item.Item}, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

// ListReportsByCustomID implements same signature of the DB interface.
func (db *DynamoDB) ListReportsByCustomID(customID string, minutes int, projection Projection) ([]Report, error) {
	if len(customID) == 0 {
		customID = customIDPlaceholder
	}
	builder := expression.NewBuilder().WithKeyCondition(expression.Key("CustomID").Equal(expression.Value(customID)))
	if minutes > 0 {
		timestamp := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
		builder = builder.WithFilter(expression.Name("ServerTime").GreaterThanEqual(expression.Value(timestamp.Unix())))
	}
	expr, err := db.applyProjection(builder, projection).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}
	return db.queryReports(&dynamodb.QueryInput{
		TableName:                 &db.nodesTable,
		IndexName:                 &db.customIDIndex,
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
}

// DeleteReport implements same signature of the DB interface.
func (db *DynamoDB) DeleteReport(id string) error {
	hash, err := db.encoder.Encode(struct{ ID string }{id})
	if err != nil {
		return fmt.Errorf("invalid report ID: %v", err)
	}
	_, err = db.instance.DeleteItem(&dynamodb.DeleteItemInput{TableName: &db.nodesTable, Key: hash.M})
	return err
}

// ListHistory implements same signature of the DB interface.
func (db *DynamoDB) ListHistory(id string, begin time.Time, end time.Time, projection Projection) ([]Report, error) {
	keyCond := expression.Key("ID").Equal(expression.Value(id)).And(
		expression.Key("ServerTime").Between(expression.Value(begin.Unix()), expression.Value(end.Unix())))
	builder := db.applyProjection(expression.NewBuilder().WithKeyCondition(keyCond), projection)
	expr, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}
	return db.queryReports(&dynamodb.QueryInput{
		TableName:                 &db.logsTable,
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
}

// GetUserSession implements same signature of the DB interface.
func (db *DynamoDB) GetUserSession(id string) (*UserSession, error) {
	hash, err := db.encoder.Encode(struct{ ID string }{id})
	if err != nil {
		return nil, fmt.Errorf("invalid session ID: %w", err)
	}
	item, err := db.instance.GetItem(&dynamodb.GetItemInput{TableName: &db.sessionsTable, Key: hash.M})
	if err != nil {
		return nil, err
	}
	if len(item.Item) == 0 {
		return nil, nil
	}
	var us UserSession
	if err := db.decoder.Decode(&dynamodb.AttributeValue{M: item.Item}, &us); err != nil {
		return nil, err
	}
	return &us, nil
}

// PutUserSession implements same signature of the DB interface.
func (db *DynamoDB) PutUserSession(session UserSession) error {
	item, err := db.encoder.Encode(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	_, err = db.instance.PutItem(&dynamodb.PutItemInput{TableName: &db.sessionsTable, Item: item.M})
	return err
}

// DeleteUserSession implements same signature of the DB interface.
func (db *DynamoDB) DeleteUserSession(id string) error {
	hash, err := db.encoder.Encode(struct{ ID string }{id})
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}
	_, err = db.instance.DeleteItem(&dynamodb.DeleteItemInput{TableName: &db.sessionsTable, Key: hash.M})
	return err
}

// SessionTTLSeconds calculates session TTL seconds.
func (db *DynamoDB) SessionTTLSeconds() int {
	return db.sessionsTTLDays * 24 * 60 * 60
}

func (db *DynamoDB) applyProjection(builder expression.Builder, projection Projection) expression.Builder {
	switch projection {
	case IDAttributes:
		return builder.WithProjection(expression.NamesList(
			expression.Name("ID"),
			expression.Name("CustomID"),
			expression.Name("ServerTime"),
			expression.Name("Success"),
		))
	case ListViewAttributes:
		return builder.WithProjection(expression.NamesList(
			expression.Name("ID"),
			expression.Name("CustomID"),
			expression.Name("Hostname"),
			expression.Name("ServerTime"),
			expression.Name("SSHServerHost"),
			expression.Name("SSHRemotePort"),
			expression.Name("GlobalIP"),
			expression.Name("GlobalHost"),
			expression.Name("LocalIPv4"),
			expression.Name("LocalIPv6"),
			expression.Name("Sequence"),
			expression.Name("AgentVersion"),
			expression.Name("Success"),
			expression.Name("Errors"),
		))
	case MeasurementAttributes:
		return builder.WithProjection(expression.NamesList(
			expression.Name("ID"),
			expression.Name("CustomID"),
			expression.Name("Hostname"),
			expression.Name("ServerTime"),
			expression.Name("Sequence"),
			expression.Name("RTTMills"),
			expression.Name("UploadKBPS"),
			expression.Name("DownloadKBPS"),
			expression.Name("Success"),
		))
	}
	return builder
}

func (db *DynamoDB) findAPIKey(key string) (APIKey, error) {
	hash, err := db.encoder.Encode(struct{ Key string }{key})
	if err != nil {
		return APIKey{}, fmt.Errorf("invalid key: %v", err)
	}
	item, err := db.instance.GetItem(&dynamodb.GetItemInput{TableName: &db.keysTable, Key: hash.M})
	if err != nil || item == nil {
		return APIKey{}, err
	}
	var apiKey APIKey
	if err := db.decoder.Decode(&dynamodb.AttributeValue{M: item.Item}, &apiKey); err != nil {
		return APIKey{}, err
	}
	return apiKey, nil
}

func (db *DynamoDB) queryReports(query *dynamodb.QueryInput) ([]Report, error) {
	var records []Report
	if err := db.instance.QueryPages(query, func(output *dynamodb.QueryOutput, lastPage bool) bool {
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
