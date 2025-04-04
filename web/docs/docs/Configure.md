---
sidebar_position: 4
---

Marmot can be configured using a YAML configuration file or environment variables.

## Configuration File

Default location: `./config.yaml`

Example configuration:

```yaml
database:
  host: localhost
  port: 5432
  user: postgres
  password: password
  name: marmot
  sslmode: disable
  maxConns: 10
  idleConns: 5
  connLifetime: 30
server:
  host: 0.0.0.0
  port: 8080
logging:
  level: info
  format: console
```

## Configuration Options

| Configuration Key       | Description                          | Default     | Environment Variable     |
| ----------------------- | ------------------------------------ | ----------- | ------------------------ |
| `database.host`         | PostgreSQL host                      | `localhost` | `DATABASE_HOST`          |
| `database.port`         | PostgreSQL port                      | `5432`      | `DATABASE_PORT`          |
| `database.user`         | PostgreSQL username                  | `postgres`  | `DATABASE_USER`          |
| `database.password`     | PostgreSQL password                  | -           | `DATABASE_PASSWORD`      |
| `database.name`         | PostgreSQL database name             | `marmot`    | `DATABASE_NAME`          |
| `database.sslmode`      | PostgreSQL SSL mode                  | `disable`   | `DATABASE_SSLMODE`       |
| `database.maxConns`     | Max database connections             | `10`        | `DATABASE_MAX_CONNS`     |
| `database.idleConns`    | Min idle connections                 | `5`         | `DATABASE_IDLE_CONNS`    |
| `database.connLifetime` | Connection lifetime (minutes)        | `30`        | `DATABASE_CONN_LIFETIME` |
| `server.host`           | Server host                          | `0.0.0.0`   | `SERVER_HOST`            |
| `server.port`           | Server port                          | `8080`      | `SERVER_PORT`            |
| `logging.level`         | Log level (debug, info, warn, error) | `info`      | `LOGGING_LEVEL`          |
| `logging.format`        | Log format (json, console)           | `json`      | `LOGGING_FORMAT`         |

## Configuration Loading

Marmot loads configuration in the following order:

1. Command-line flags (e.g., `--config` to specify a configuration file)
2. Configuration sources (exact precedence between environment variables and configuration file may depend on your installation)

## Database Configuration

Marmot requires a PostgreSQL database. The connection string is built using the database configuration parameters. Make sure the PostgreSQL user has sufficient privileges to create tables and indexes.

## Server Configuration

The server configuration determines the host and port that Marmot listens on. By default, Marmot binds to all interfaces (`0.0.0.0`) on port `8080`.

## Logging Configuration

Marmot uses structured logging with configurable levels and formats:

- **Levels**: debug, info, warn, error
- **Formats**: json, console
