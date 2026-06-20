---
sidebar_position: 1
---

# Marmot for Agents

An agent acting without context is guessing. It doesn't know which table holds customer orders, who owns the payments service, what a column means, or what breaks downstream if it changes something. **Marmot for Agents** ends that context starvation: it plugs your LLM agents into the catalog so they read it for context and write back the lineage they generate.

import { CalloutCard, DocCard, DocCardGrid, FeatureCard, FeatureGrid } from '@site/src/components/DocCard';

## What your agents can do

<FeatureGrid>
  <FeatureCard
    title="Discover assets"
    description="Find any service, API, queue, topic, table or pipeline by name, type or metadata."
    icon="mdi:magnify"
  />
  <FeatureCard
    title="Resolve ownership"
    description="Answer who owns an asset and who to contact before acting on it."
    icon="mdi:account-search-outline"
  />
  <FeatureCard
    title="Understand meaning"
    description="Pull business definitions and glossary terms so answers use your language."
    icon="mdi:book-open-page-variant-outline"
  />
  <FeatureCard
    title="Trace lineage"
    description="See upstream and downstream dependencies to reason about impact."
    icon="mdi:source-branch"
  />
</FeatureGrid>

## Two ways to connect

<DocCardGrid>
  <DocCard
    title="MCP Server"
    description="The fastest path: point any client that speaks MCP (Claude, Cursor, ChatGPT, Cline) at Marmot and start asking."
    docId="MCP/index"
    icon="mdi:protocol"
  />
  <DocCard
    title="SDK"
    description="Build agents directly against the catalog with the typed Python, Go or TypeScript client."
    docId="SDK/index"
    icon="mdi:code-braces"
  />
</DocCardGrid>

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
