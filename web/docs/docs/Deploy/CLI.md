---
sidebar_position: 4
---

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

## Generate Encryption Key

Generate a secure encryption key for pipeline credentials:

```bash
marmot generate-encryption-key
```

## Running Marmot

```bash
# Set the encryption key
export MARMOT_SERVER_ENCRYPTION_KEY="your-generated-key"

marmot server --config /path/to/config.yaml
```

> **The default username and password is admin:admin**

## Configuration

Create a `config.yaml` file with your database and server settings:

```yaml
server:
  encryption_key: "your-generated-key"  # Or use MARMOT_SERVER_ENCRYPTION_KEY env var

database:
  host: localhost
  port: 5432
  user: postgres
  password: password
  name: marmot
```

### Development Without Encryption

For development only (credentials stored in plaintext):

```yaml
server:
  allow_unencrypted: true
```

Read more about [available configuration options here.](/docs/configure)
