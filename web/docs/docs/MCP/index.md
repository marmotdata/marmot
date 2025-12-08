---
sidebar_position: 6
---

# Model Context Protocol (MCP)

Marmot includes a built-in **Model Context Protocol (MCP)** server that enables AI assistants like Claude, ChatGPT and other LLM-powered tools to interact with your data catalog using natural language.

## What is MCP?

The Model Context Protocol is a standardised way for AI assistants to connect with external data sources and systems. Think of it as a universal translator between AI models and your data. It exposes your catalog's capabilities, metadata and functions through machine-readable schemas that AI assistants can understand and use.

With MCP, you can ask questions like:

- "What tables does the analytics team own?"
- "Show me all BigQuery datasets tagged as 'production'"
- "Find the upstream dependencies for the user_events table"
- "Who owns the payment processing API?"

## How It Works

Marmot's MCP server exposes your data catalog's metadata in real-time, enabling AI assistants to:

1. **Search Assets** - Query your catalog using natural language
2. **View Lineage** - Explore upstream and downstream dependencies
3. **Read Metadata** - Access descriptions, owners, tags and custom metadata
4. **Discover Context** - Understand relationships between assets

## Authentication

MCP uses the same authentication as Marmot's REST API. You'll need an API key to connect:

1. Navigate to your user profile in Marmot
2. Go to **Settings** → **API Keys**
3. Generate a new API key
4. Use this key in your MCP client configuration

The AI assistant will have the same permissions as your user account, respecting all role-based access controls.

## Getting Started

Choose your AI assistant to see configuration examples:

- **[Claude Desktop](./claude-desktop.md)** - Anthropic's official desktop application
- **[Claude Code](./claude-code.md)** - Claude's command-line interface
- **[Cursor](./cursor.md)** - The AI-first code editor
- **[Cline](./cline.md)** - VS Code extension for AI-powered coding
- **[LibreChat](./librechat.md)** - Universal AI chat interface supporting multiple providers

## Available Tools

Marmot's MCP server provides the following tools to AI assistants:

### discover_data

Unified data discovery for finding any asset in the catalog. Supports natural language queries, specific lookups by ID or MRN (qualified identifiers like `postgres://db/schema/table`), filtering by type/provider/tags and metadata-based queries.

Returns asset details including ownership, schema and lineage information.

### find_ownership

Bidirectional ownership queries to answer "Who owns this asset?", "What does this user own?" and "Show me all data owned by the data-eng team". Works for both data assets and glossary terms. Can query by asset ID, user ID/username or team ID/team name.

### lookup_term

Business glossary lookups for understanding terminology and definitions. Search for glossary terms by name or retrieve specific term definitions. Returns term details, ownership, related terms and parent/child relationships in the glossary hierarchy.

## Example Queries

Once configured, you can interact with Marmot through natural language:

- "Find all Kafka topics owned by the data-platform team"
- "What are the upstream dependencies for the analytics.user_events table?"
- "What does 'Monthly Active Users' mean in our glossary?"
- "Show me all BigQuery datasets tagged as production"
- "Who owns the customer_data table?"

For more help, join our [Discord community](https://discord.gg/tMgc9ayB).
