---
sidebar_position: 1
---

# Marmot for Agents

**Marmot for Agents** plugs your LLM agents into the catalog. They read it for context and write back the lineage they generate.

import { CalloutCard, DocCard, DocCardGrid, FeatureCard, FeatureGrid } from '@site/src/components/DocCard';

## Supported frameworks

<DocCardGrid>
  <DocCard
    title="LangChain"
    description="Catalog tools and a callback handler for Python and TypeScript agents."
    docId="Agents/langchain"
    icon="simple-icons:langchain"
  />
  <DocCard
    title="Claude Agent SDK"
    description="Hooks for Anthropic's Agent SDK that auto-register the agent and capture lineage from MCP tool calls."
    docId="Agents/claude-agent"
    icon="simple-icons:claude"
  />
</DocCardGrid>
