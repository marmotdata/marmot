---
sidebar_position: 4
---

This guide covers deploying Marmot using the command-line interface.

## Installation

### Automatic Installation

You can install Marmot with the automatic installation script, it's strongly recommended you inspect the contents of any script before piping it into bash.

```bash
curl -fsSL get.marmotdata.io | sh
```

### Manual Installation

If you prefer to install manually:

1. Download the latest Marmot binary for your platform from [GitHub Releases](https://github.com/marmotdata/marmot/releases)
2. Make the binary executable:
   ```bash
   chmod +x marmot
   ```
3. Move the binary to a location in your PATH:
   ```bash
   sudo mv marmot /usr/local/bin/
   ```

## Running Marmot

```bash
marmot run --config /path/to/config.yaml
```

> __The default username and password is admin:admin__

## Configuration

Create a `config.yaml` file with your database connection details. You can read more about [available configuration options here.](/docs/configure)

```yaml
database:
  host: localhost
  port: 5432
  user: postgres
  password: password
  name: marmot
```
