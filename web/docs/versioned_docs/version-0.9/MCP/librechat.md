# LibreChat

Universal AI chat interface supporting multiple providers with native MCP support.

## Configuration

Add the Marmot MCP server to your `librechat.yaml`:

```yaml
mcpServers:
  marmot:
    type: streamable-http
    url: https://<your-marmot-server>/api/v1/mcp
    headers:
      X-API-Key: <your-api-key>
    timeout: 30000
```

You can use environment variables with `${VAR_NAME}` syntax and user context with `{{LIBRECHAT_USER_ID}}` placeholders.

