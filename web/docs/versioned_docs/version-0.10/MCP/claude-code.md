# Claude Code

Anthropic's CLI for Claude AI with native MCP support.

## Configuration

### Using an API Key

Create or edit `~/.claude.json` (user-level) or `.mcp.json` (project root):

```json
{
  "mcpServers": {
    "marmot": {
      "type": "http",
      "url": "https://<your-marmot-server>/api/v1/mcp",
      "headers": {
        "X-API-Key": "<your-api-key>"
      }
    }
  }
}
```

### Using a Bearer Token

If you authenticate with `marmot login`, you can use the cached token instead of an API key:

```json
{
  "mcpServers": {
    "marmot": {
      "type": "http",
      "url": "https://<your-marmot-server>/api/v1/mcp",
      "headers": {
        "Authorization": "Bearer <your-token>"
      }
    }
  }
}
```

Project-scoped servers require approval on first use.
