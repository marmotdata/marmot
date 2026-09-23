# Cline

VS Code extension for autonomous AI assistance with native MCP support.

## Configuration

1. Open Cline in VS Code (click the Cline icon in the sidebar)
2. Click the MCP Servers icon in Cline's top navigation
3. Select the "Configure" tab
4. Click "Configure MCP Servers"

Add the Marmot server to `cline_mcp_settings.json`:

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

