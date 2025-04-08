# Configure

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
auth:
  anonymous:
    enabled: true
    role: user
```

## Configuration Options

| Configuration Key        | Description                            | Default     | Environment Variable            |
| ------------------------ | -------------------------------------- | ----------- | ------------------------------- |
| `database.host`          | PostgreSQL host                        | `localhost` | `MARMOT_DATABASE_HOST`          |
| `database.port`          | PostgreSQL port                        | `5432`      | `MARMOT_DATABASE_PORT`          |
| `database.user`          | PostgreSQL username                    | `postgres`  | `MARMOT_DATABASE_USER`          |
| `database.password`      | PostgreSQL password                    | -           | `MARMOT_DATABASE_PASSWORD`      |
| `database.name`          | PostgreSQL database name               | `marmot`    | `MARMOT_DATABASE_NAME`          |
| `database.sslmode`       | PostgreSQL SSL mode                    | `disable`   | `MARMOT_DATABASE_SSLMODE`       |
| `database.maxConns`      | Max database connections               | `10`        | `MARMOT_DATABASE_MAX_CONNS`     |
| `database.idleConns`     | Min idle connections                   | `5`         | `MARMOT_DATABASE_IDLE_CONNS`    |
| `database.connLifetime`  | Connection lifetime (minutes)          | `30`        | `MARMOT_DATABASE_CONN_LIFETIME` |
| `server.host`            | Server host                            | `0.0.0.0`   | `MARMOT_SERVER_HOST`            |
| `server.port`            | Server port                            | `8080`      | `MARMOT_SERVER_PORT`            |
| `logging.level`          | Log level (debug, info, warn, error)   | `info`      | `MARMOT_LOGGING_LEVEL`          |
| `logging.format`         | Log format (json, console)             | `json`      | `MARMOT_LOGGING_FORMAT`         |
| `auth.anonymous.enabled` | Enable anonymous authentication        | `false`     | `MARMOT_AUTH_ANONYMOUS_ENABLED` |
| `auth.anonymous.role`    | Role to be assigned to anonymous users | `user`      | `MARMOT_AUTH_ANONYMOUS_ROLE`    |

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
