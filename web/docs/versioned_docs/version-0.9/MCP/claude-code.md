# Claude Code

Anthropic's CLI for Claude AI with native MCP support.

## Configuration

### User-Level Configuration

Create or edit `~/.claude.json`:

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

### Project-Level Configuration

Create `.mcp.json` in your project root:

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

Project-scoped servers require approval on first use.
