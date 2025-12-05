# Claude Desktop

Anthropic's official desktop application with native MCP support.

## Configuration

Edit the configuration file for your platform:

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

Add the Marmot MCP server:

```json
{
  "mcpServers": {
    "marmot": {
      "command": "npx",
      "args": [
        "-y",
        "mcp-remote",
        "https://<your-marmot-server>/api/v1/mcp",
        "--header",
        "X-API-Key:<your-api-key>"
      ]
    }
  }
}
```

For HTTP connections (development), add `--allow-http` to the args array.

## Activation

1. Save the configuration file
2. Restart Claude Desktop
