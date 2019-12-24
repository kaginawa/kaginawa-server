kaginawa-server
===============

[![Actions Status](https://github.com/kaginawa/kaginawa-server/workflows/Go/badge.svg)](https://github.com/kaginawa/kaginawa-server/actions)
[![Actions Status](https://github.com/kaginawa/kaginawa-server/workflows/Docker/badge.svg)](https://github.com/kaginawa/kaginawa-server/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/kaginawa/kaginawa-server)](https://goreportcard.com/report/github.com/kaginawa/kaginawa-server)

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/kaginawa/kaginawa-server)

Kaginawa server program.

See [kaginawa](https://github.com/kaginawa/kaginawa) repository for more details.

## Requirements

### General

Environment variables:

- `LOGIN_USER`: Web interface username
- `LOGIN_PASSWORD`: Web interface password

### Using MongoDB

Environment variables:

- `MONGODB_URI`: MongoDB endpoint (`mongodb://user:pass@host:port/db`)

Kaginawa Server automatically creates following collections when first touch:

- `keys` - All API keys
- `servers` - All SSH servers
- `nodes` - Newest received reports for each nodes
- `logs` - All received reports

We recommend creating `logs` collection as a [capped collection](https://docs.mongodb.com/manual/core/capped-collections/).

### Using DynamoDB

Kaginawa server uses AWS default credentials.
See the comment of [AWS SDK for Go API Reference](https://docs.aws.amazon.com/sdk-for-go/api/aws/session/#NewSession) for more details.

Environment variables:

- `DYNAMO_KEYS`: Name of table of keys (e.g. `KaginawaKeys`)
- `DYNAMO_SERVERS`: Name of table of servers (e.g. `KaginawaServers`)
- `DYNAMO_NODES`: Name of table of nodes (e.g. `KaginawaNodes`)
- `DYNAMO_LOGS`: Name of table of logs (e.g. `KaginawaLogs`)
- `DYNAMO_CUSTOM_IDS`: Name of index of custom id (e.g. `CustomID-index`)
- `DYNAMO_TTL_DAYS`: (Optional) TTL for table of logs 
- `DYNAMO_ENDPOINT`: (Optional) Custom endpoint (i.e. using DynamoDB Local)

Create table of keys using aws-cli:

```
aws dynamodb create-table \
    --table-name KaginawaKeys \
    --attribute-definitions AttributeName=Key,AttributeType=S \
    --key-schema AttributeName=Key,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1
```

Create table of servers using aws-cli:

```
aws dynamodb create-table \
    --table-name KaginawaServers \
    --attribute-definitions AttributeName=Host,AttributeType=S \
    --key-schema AttributeName=Host,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1
```

Create table of nodes using aws-cli:

```
aws dynamodb create-table \
    --table-name KaginawaNodes \
    --attribute-definitions AttributeName=ID,AttributeType=S \
    --key-schema AttributeName=ID,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1
```

Create table of logs using aws-cli:

```
aws dynamodb create-table \
    --table-name KaginawaLogs \
    --attribute-definitions \
        AttributeName=ID,AttributeType=S \
        AttributeName=ServerTime,AttributeType=N \
    --key-schema AttributeName=ID,KeyType=HASH AttributeName=ServerTime,KeyType=RANGE \
    --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1
```

Create index of custom ID for table of nodes using aws-cli:

```
aws dynamodb update-table \
    --table-name KaginawaNodes \
    --attribute-definitions AttributeName=CustomID,AttributeType=S \
    --global-secondary-index-updates \
    "[{\"Create\":{\"IndexName\": \"CustomID-index\",\"KeySchema\":[{\"AttributeName\":\"CustomID\",\"KeyType\":\"HASH\"}], \
    \"ProvisionedThroughput\": {\"ReadCapacityUnits\": 1, \"WriteCapacityUnits\": 1},\"Projection\":{\"ProjectionType\":\"ALL\"}}}]" 
```

## Admin API

### List nodes

- Method: `GET`
- Resource: `/nodes`
- Query Params:
    - (Optional) `custom-id` - filter by custom-id
    - (Optional) `minutes` - filter by minutes ago
    - (Optional) `projection` - pattern of projection attributes (`all`, `id` or `list-view`)
- Headers:
    - `Authorization: token <admin_api_key>`
    - `Accept: application/json`
- Response: List of all `Record` object (see [db.go](db.go) definition)

Curl example with no query params:

```
curl -H "Authorization: token admin123" -H "Accept: application/json" -X GET "http://localhost:8080/nodes"
```

Curl example with `custom-id`:

```
curl -H "Authorization: token admin123" -H "Accept: application/json" -X GET "http://localhost:8080/nodes?custom-id=dev1"
```

NOTE: Custom IDs are expected to be unique, but can be duplicated (such as device replacements).

Curl example with `custom-id` and `minutes`:

```
curl -H "Authorization: token admin123" -H "Accept: application/json" -X GET "http://localhost:8080/nodes?custom-id=dev1&minutes=5"
```

Curl example with `minutes` and `projection`:

```
curl -H "Authorization: token admin123" -H "Accept: application/json" -X GET "http://localhost:8080/nodes?minutes=5&projection=id"
```

### Get node by ID

- Method: `GET`
- Resource: `/nodes/<ID>`
- Headers:
    - `Authorization: token <admin_api_key>`
    - `Accept: application/json`
- Response: A `Record` object (see [db.go](db.go) definition)

Curl example:

```
curl -H "Authorization: token admin123" -H "Accept: application/json" -X GET "http://localhost:8080/nodes/02:00:17:00:7d:b0"
```

### Send command via ssh

- Method: `POST`
- Resource: `/nodes/<ID>/command`
- Header:
    - `Authorization: token <admin_api_key>`
- Form params:
    - `command` - command
    - `user` - ssh user name
    - (Optional) `key` - ssh private key
    - (Optional) `password` - ssh password
    - (Optional) `timeout` - timeout seconds (default: 30)
- Response: Command result (MIME: `text/plain`)

Curl example:

```
curl -H "Authorization: token admin123" -X POST -d user=pi -d password=raspberry -d timeout=10 -d command="ls -alh" "http://localhost:8080/nodes/02:00:17:00:7d:b0/command"
```

## License

Kaginawa Server licensed under the [BSD 3-clause license](LICENSE).

## Author

- [mikan](https://github.com/mikan)
