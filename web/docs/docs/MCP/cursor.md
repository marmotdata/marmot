# Cursor

AI-first code editor with native MCP support.

## Configuration

### Global Configuration

Create or edit `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "marmot": {
      "url": "https://<your-marmot-server>/api/v1/mcp",
      "headers": {
        "X-API-Key": "<your-api-key>"
      }
    }
  }
}
```

### Project-Level Configuration

Create `.cursor/mcp.json` in your project root:

```json
{
  "mcpServers": {
    "marmot": {
      "url": "https://<your-marmot-server>/api/v1/mcp",
      "headers": {
        "X-API-Key": "<your-api-key>"
      }
    }
  }
}
```

You can use environment variables with `${env:VAR_NAME}` syntax for sensitive credentials.

## Activation

1. Save the configuration file
2. Restart Cursor
3. Open the Cursor AI chat panel (Cmd/Ctrl+L)
4. Marmot tools will be available
