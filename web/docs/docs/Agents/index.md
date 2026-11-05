---
sidebar_position: 1
title: Marmot for agents
---

# Marmot for agents

An agent without context guesses: it doesn't know which table holds customer orders, who owns the payments service or what breaks downstream if it changes something. Marmot gives your agents the catalog to read, so they can find assets, look up owners and glossary terms and trace lineage, and records the lineage they create in return.

An agent that drives the Marmot CLI should run it in [read-only mode](../cli.md#read-only-mode-and-exit-codes) unless it's meant to change things, and branch on its exit codes.

import { DocCard, DocCardGrid } from '@site/src/components/DocCard';

<DocCardGrid>
  <DocCard
    title="MCP server"
    description="Point Claude Code, Claude Desktop, Cursor, Cline or LibreChat at Marmot and start asking."
    docId="MCP/index"
    icon="mdi:protocol"
  />
  <DocCard
    title="Skills"
    description="Teach any coding agent the Marmot CLI and REST API."
    docId="MCP/skills"
    icon="mdi:brain"
  />
  <DocCard
    title="LangChain"
    description="Catalog tools and a callback handler for Python and TypeScript agents."
    docId="Agents/langchain"
    icon="simple-icons:langchain"
  />
  <DocCard
    title="Claude Agent SDK"
    description="Hooks that register the agent and capture lineage from its MCP tool calls."
    docId="Agents/claude-agent"
    icon="simple-icons:claude"
  />
</DocCardGrid>
