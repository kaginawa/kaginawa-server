kaginawa-server
===============

[![Actions Status](https://github.com/kaginawa/kaginawa-server/workflows/Go/badge.svg)](https://github.com/kaginawa/kaginawa-server/actions)
[![Actions Status](https://github.com/kaginawa/kaginawa-server/workflows/Docker/badge.svg)](https://github.com/kaginawa/kaginawa-server/actions)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=kaginawa_kaginawa-server&metric=alert_status)](https://sonarcloud.io/dashboard?id=kaginawa_kaginawa-server)
[![Go Report Card](https://goreportcard.com/badge/github.com/kaginawa/kaginawa-server)](https://goreportcard.com/report/github.com/kaginawa/kaginawa-server)

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/kaginawa/kaginawa-server)

Kaginawa server program.

See [kaginawa](https://github.com/kaginawa/kaginawa) repository for more details.

Docker image is available at [Docker Hub](https://hub.docker.com/r/kaginawa/kaginawa-server).

## Requirements

### OAuth 2.0 Provider

Administration users must be authorized by OAuth 2.0 provider.

You can choose Auth0, Google and more as an identity provider.

#### Using Auth0

Required environment variables:

- `AUTH0_DOMAIN` - OAuth 2.0 provider domain name (e.g. `xxx.auth0.com`)
- `AUTH0_CLIENT_ID` - OAuth 2.0 provider client ID
- `AUTH0_CLIENT_SECRET` - OAuth 2.0 provider client secret
- `SELF_URL` - Self URL using OAuth 2.0 callback process (e.g. `http://localhost:8080`)

If you use the [Deploy to Heroku] button, they will be set automatically by the add-on except `SELF_URL`.

#### Using Google OAuth 2.0 API

Required environment variables:

- `OAUTH_TYPE` - Set to `google`
- `OAUTH_CLIENT_ID` - OAuth 2.0 provider client ID
- `OAUTH_CLIENT_SECRET` - OAuth 2.0 provider client secret
- `SELF_URL` - Self URL using OAuth 2.0 callback process (e.g. `http://localhost:8080`)

See [developer.google.com](https://developers.google.com/identity/protocols/oauth2/openid-connect) for more information.

#### Using other identity providers

Required environment variables:

- `OAUTH_TYPE` - Set to `custom`
- `OAUTH_CLIENT_ID` - OAuth 2.0 provider client ID
- `OAUTH_CLIENT_SECRET` - OAuth 2.0 provider client secret
- `SELF_URL` - Self URL using OAuth 2.0 callback process (e.g. `http://localhost:8080`)
- `OAUTH_AUTH_URL` - OAuth 2.0 authorization URL
- `OAUTH_TOKEN_URL` - OAuth 2.0 token URL
- `OAUTH_AUDIENCE` - OAuth 2.0 audience string
- `OAUTH_USERINFO_URL` - OpenID Connect user info URL

### Database

You can choose MongoDB or DynamoDB as a database.

#### Using MongoDB

Required environment variable:

- `MONGODB_URI`: MongoDB endpoint (`mongodb://user:pass@host:port/db`)

Note that MongoDB Atlas may fail to connect with long database name, so exclude all parameters from the connection string (e.g. `mongodb+srv://user:pass@cluster/db`).

Kaginawa Server automatically creates following collections when first touch:

- `keys` - All API keys
- `servers` - All SSH servers
- `nodes` - Newest received reports for each node
- `logs` - All received reports (*1)
- `sessions` - Web UI sessions (*2)

*1) We recommend creating `logs` collection as a [capped collection](https://docs.mongodb.com/manual/core/capped-collections/).
Example mongo shell (set to 256 MB):

```
db.runCommand({"convertToCapped": "logs", size: 268435456})
```

*2) Session expiration is configurable with [TTL indexes](https://docs.mongodb.com/manual/core/index-ttl/) feature.
Example mongo shell (set to 6 months):

```
db.sessions.createIndex({"time": 1}, {expireAfterSeconds: 15552000})
```

#### Using DynamoDB

Kaginawa server uses AWS default credentials.
See the comment of [AWS SDK for Go API Reference](https://docs.aws.amazon.com/sdk-for-go/api/aws/session/#NewSession) for more details.

Required environment variables:

- `DYNAMO_KEYS` - Table of keys (e.g. `KaginawaKeys`)
- `DYNAMO_SERVERS` - Table of servers (e.g. `KaginawaServers`)
- `DYNAMO_NODES` - Table of nodes (e.g. `KaginawaNodes`)
- `DYNAMO_LOGS` - Table of logs (e.g. `KaginawaLogs`)
- `DYNAMO_SESSIONS` = Table of sessions (e.g. `KaginawaSessions`)
- `DYNAMO_CUSTOM_IDS` - Index of custom id (e.g. `CustomID-index`)
- `DYNAMO_LOGS_TTL_DAYS` - (Optional) TTL for the table of logs 
- `DYNAMO_SESSIONS_TTL_DAYS` - (Optional) TTL for the table of sessions
- `DYNAMO_ENDPOINT` - (Optional) Custom endpoint (i.e. using DynamoDB Local)

Create a table of keys using aws-cli:

```
aws dynamodb create-table \
    --table-name KaginawaKeys \
    --attribute-definitions AttributeName=Key,AttributeType=S \
    --key-schema AttributeName=Key,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1
```

Create a table of servers using aws-cli:

```
aws dynamodb create-table \
    --table-name KaginawaServers \
    --attribute-definitions AttributeName=Host,AttributeType=S \
    --key-schema AttributeName=Host,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1
```

Create a table of nodes using aws-cli:

```
aws dynamodb create-table \
    --table-name KaginawaNodes \
    --attribute-definitions AttributeName=ID,AttributeType=S \
    --key-schema AttributeName=ID,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1
```

Create a table of logs using aws-cli:

```
aws dynamodb create-table \
    --table-name KaginawaLogs \
    --attribute-definitions \
        AttributeName=ID,AttributeType=S \
        AttributeName=ServerTime,AttributeType=N \
    --key-schema AttributeName=ID,KeyType=HASH AttributeName=ServerTime,KeyType=RANGE \
    --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1
aws dynamodb update-time-to-live \
    --table-name KaginawaLogs \
    --time-to-live-specification \
        Enabled=true,AttributeName=TTL
```

Create a table of sessions using aws-cli:

```
aws dynamodb create-table \
    --table-name KaginawaSessions \
    --attribute-definitions AttributeName=ID,AttributeType=S \
    --key-schema AttributeName=ID,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1
aws dynamodb update-time-to-live \
    --table-name KaginawaSessions \
    --time-to-live-specification \
        Enabled=true,AttributeName=TTL
```

Create an index of custom ID for a table of nodes using aws-cli:

```
aws dynamodb update-table \
    --table-name KaginawaNodes \
    --attribute-definitions AttributeName=CustomID,AttributeType=S \
    --global-secondary-index-updates \
    "[{\"Create\":{\"IndexName\": \"CustomID-index\",\"KeySchema\":[{\"AttributeName\":\"CustomID\",\"KeyType\":\"HASH\"}], \
    \"ProvisionedThroughput\": {\"ReadCapacityUnits\": 1, \"WriteCapacityUnits\": 1},\"Projection\":{\"ProjectionType\":\"ALL\"}}}]" 
```

## Admin API

### `/nodes` List nodes

- Method: `GET`
- Resource: `/nodes`
- Query Params:
    - (Optional) `custom-id` - filter by custom-id
    - (Optional) `minutes` - filter by minutes ago
    - (Optional) `projection` - pattern of projection attributes (`all`, `id`, `list-view` or `measurement`)
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

NOTE: Custom IDs are expect to unique, but can be duplicated (such as device replacements).

Curl example with `custom-id` and `minutes`:

```
curl -H "Authorization: token admin123" -H "Accept: application/json" -X GET "http://localhost:8080/nodes?custom-id=dev1&minutes=5"
```

Curl example with `minutes` and `projection`:

```
curl -H "Authorization: token admin123" -H "Accept: application/json" -X GET "http://localhost:8080/nodes?minutes=5&projection=id"
```

### `/nodes/:id` Get node by ID

- Method: `GET`
- Resource: `/nodes/:id`
- Headers:
    - `Authorization: token <admin_api_key>`
    - `Accept: application/json`
- Response: A `Record` object (see [db.go](db.go) definition)

Curl example:

```
curl -H "Authorization: token admin123" -H "Accept: application/json" -X GET "http://localhost:8080/nodes/02:00:17:00:7d:b0"
```

### `/nodes/:id/command` Send command via ssh

- Method: `POST`
- Resource: `/nodes/:id/command`
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

### `/nodes/:id/histories` List report histories

- Method: `GET`
- Resource: `/nodes/:id/history`
- Header:
    - `Authorization: token <admin_api_key>`
- Form params:
    - (Optional) `begin` - begin time as UTC unix timestamp (default: 24 hours ago)
    - (Optional) `end` - end time as UTC unix timestamp (default: now)
    - (Optional) `projection` - pattern of projection attributes (`all`, `id`, `list-view` or `measurement`)
- Response: List of all `Record` object (see [db.go](db.go) definition)

Curl example:

```
curl -H "Authorization: token admin123" -H "Accept: application/json" -X GET "http://localhost:8080/nodes/02:00:17:00:7d:b0/history&begin=1581900000&end=1582000000"
```

## License

Kaginawa Server licensed under the [BSD 3-clause license](LICENSE).

## Author

- [mikan](https://github.com/mikan)
