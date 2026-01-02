---
sidebar_position: 6
---

# Model Context Protocol (MCP)

Marmot includes a built-in **Model Context Protocol (MCP)** server that enables AI assistants like Claude, ChatGPT and other LLM-powered tools to interact with your data catalog using natural language.

import { CalloutCard, DocCard, DocCardGrid, FeatureCard, FeatureGrid } from '@site/src/components/DocCard';

<CalloutCard
  title="What is MCP?"
  description="The Model Context Protocol is a standardised way for AI assistants to connect with external data sources - like a universal translator between AI models and your data."
  href="https://modelcontextprotocol.io"
  buttonText="Learn More"
  icon="mdi:robot"
/>

## What Can You Do?

With MCP, you can ask questions like:

- "What tables does the analytics team own?"
- "Show me all BigQuery datasets tagged as 'production'"
- "Find the upstream dependencies for the user_events table"
- "Who owns the payment processing API?"

<FeatureGrid>
  <FeatureCard
    title="Search Assets"
    description="Query your catalog using natural language"
    icon="mdi:magnify"
  />
  <FeatureCard
    title="View Lineage"
    description="Explore upstream and downstream dependencies"
    icon="mdi:source-branch"
  />
  <FeatureCard
    title="Read Metadata"
    description="Access descriptions, owners, tags and custom metadata"
    icon="mdi:tag-multiple"
  />
  <FeatureCard
    title="Discover Context"
    description="Understand relationships between assets"
    icon="mdi:graph"
  />
</FeatureGrid>

## Choose Your AI Assistant

<DocCardGrid>
  <DocCard
    title="Claude Desktop"
    description="Anthropic's official desktop application"
    href="/docs/MCP/claude-desktop"
    icon="simple-icons:anthropic"
  />
  <DocCard
    title="Claude Code"
    description="Claude's command-line interface"
    href="/docs/MCP/claude-code"
    icon="mdi:console"
  />
  <DocCard
    title="Cursor"
    description="The AI-first code editor"
    href="/docs/MCP/cursor"
    icon="mdi:cursor-default"
  />
  <DocCard
    title="Cline"
    description="VS Code extension for AI-powered coding"
    href="/docs/MCP/cline"
    icon="mdi:microsoft-visual-studio-code"
  />
  <DocCard
    title="LibreChat"
    description="Universal AI chat interface supporting multiple providers"
    href="/docs/MCP/librechat"
    icon="mdi:chat"
  />
</DocCardGrid>

## Authentication

MCP uses the same authentication as Marmot's REST API. You'll need an API key to connect:

1. Navigate to your user profile in Marmot
2. Go to **Settings** → **API Keys**
3. Generate a new API key
4. Use this key in your MCP client configuration

The AI assistant will have the same permissions as your user account, respecting all role-based access controls.

## Available Tools

Marmot's MCP server provides these tools to AI assistants:

### discover_data

Unified data discovery for finding any asset in the catalog. Supports natural language queries, specific lookups by ID or MRN (qualified identifiers like `postgres://db/schema/table`), filtering by type/provider/tags and metadata-based queries.

### find_ownership

Bidirectional ownership queries to answer "Who owns this asset?", "What does this user own?" and "Show me all data owned by the data-eng team". Works for both data assets and glossary terms.

### lookup_term

Business glossary lookups for understanding terminology and definitions. Search for glossary terms by name or retrieve specific term definitions.

<CalloutCard
  title="Need Help?"
  description="Join our Discord community to get help, share feedback, and connect with other Marmot users."
  href="https://discord.gg/tMgc9ayB"
  buttonText="Join Discord"
  variant="secondary"
  icon="mdi:account-group"
/>
