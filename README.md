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

## Author

- [mikan](https://github.com/mikan)
