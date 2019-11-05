kaginawa-server
===============

[![Actions Status](https://github.com/kaginawa/kaginawa-server/workflows/Go/badge.svg)](https://github.com/kaginawa/kaginawa-server/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/kaginawa/kaginawa-server)](https://goreportcard.com/report/github.com/kaginawa/kaginawa-server)

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/kaginawa/kaginawa-server)

Kaginawa server program.

See [kaginawa](https://github.com/kaginawa/kaginawa) repository for more details.

## Requirements

Environment variables:

- `MONGODB_URI`: MongoDB endpoint (`mongodb://user:pass@host:port/db`)

Kaginawa Server automatically creates following collections when first touch:

- `keys` - All API keys
- `servers` - All SSH servers
- `nodes` - Newest received reports for each nodes
- `logs` - All received reports

We recommend creating `logs` collection as a [capped collection](https://docs.mongodb.com/manual/core/capped-collections/).

## Admin API

### List all nodes

- Method: `GET`
- Resource: `/nodes`
- Headers:
    - `Authorization: token <admin_api_key>`
    - `Accept: application/json`
- Response: List of all `Record` object (see [db.go](db.go) definition)

Curl example:

```
curl -H "Authorization: token admin123" -H "Accept: application/json" -X GET "http://localhost:8080/nodes/02:00:17:00:7d:b0"
```

### Get node by custom ID

- Request: `GET `
- Resource: `/nodes`
- Query Params:
    - (Optional) `custom-id`
- Headers:
    - `Authorization: token <admin_api_key>`
    - `Accept: application/json`
- Response: List of matched `Record` object (see [db.go](db.go) definition)

This API can return multiple records. 
Custom IDs are expected to be unique, but can be duplicated (such as device replacements).

Curl example:

```
curl -H "Authorization: token admin123" -H "Accept: application/json" -X GET "http://localhost:8080/nodes?custom-id=dev1"
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
