# ChainLoop Bedrock

## Projects

- `app/controlplane`
- `app/artifact-cas`
- `app/cli`

See makefiles in those directories for more information

## Development

### Using Docker Compose

You can run the core services (controlplane and CAS) and associated dependencies (postgresql) by running

```
docker compose up
```

Then, the CLI can be run by doing

```
docker compose run --rm cli
```

Note that changes made in the source code are not reflected automatically in the running services, for that you'll need to perform a restart.

```
docker compose restart -t0 controlplane
# or
docker compose restart -t0 cas
```

### Locally

Prerequisites

- postgresql

Note: You can run the prerequisites by leveraging the provided docker-compose file i.e `docker compose up postgresql`

Then each project has a `make run` target that can be used
