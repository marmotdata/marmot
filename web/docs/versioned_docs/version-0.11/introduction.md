---
sidebar_position: 1
description: Who builds Marmot, what it is for, what we stand for, and where to go next in the documentation.
---

import { CalloutCard, DocCard, DocCardGrid, FeatureCard, FeatureGrid } from '@site/src/components/DocCard';
import IntroDiagram from '@site/src/components/IntroDiagram';

# Introduction

Marmot is the open source **context layer** for your whole stack. It is a single catalog for every asset your systems and teams depend on: services, APIs, queues, topics, brokers, databases, tables and pipelines. It records what each one is, who owns it, what it means and what it connects to, then hands that context to whoever needs it, whether that is an engineer in a browser or an AI agent over [MCP](MCP/index.md).

<CalloutCard
  title="Get started in five minutes"
  description="Run Marmot locally with Docker Compose and catalog your first assets."
  docId="quick-start"
  buttonText="Quick Start"
  icon="mdi:rocket-launch"
/>

## Who we are

Marmot Data is the company behind Marmot. We are a small team of engineers with a taste for small things: one binary, one database, one place to look for the answer. We believe the hardest problems in a stack are rarely inside any single system. They live in the gaps between systems, in the questions nobody can answer quickly. What is this topic for? Who owns that table? What breaks if we change this? Marmot is our answer, and we build it in the open.

Everything we make starts as [open source](https://github.com/marmotdata/marmot) and stays that way. Marmot Cloud exists for the teams who would rather someone else ran the servers, and it pays for the work on the project everyone else runs for free. We talk to the people who use Marmot directly, on [Discord](https://discord.gg/TWCk7hVFN4) and on GitHub, we write down what we learn on the [blog](/blog), and when you report a bug, an engineer reads it.

## Our mission

**Make anyone, human or agent, autonomous in the systems they work with.**

Autonomy is the ability to act without waiting for someone else to explain things. An engineer joining a team should be able to open an unfamiliar topic and know what it is for, who owns it and what depends on it, without booking a meeting. An AI agent asked to change a pipeline should be able to check the real schema, the real owner and the real downstream consumers, instead of inventing a plausible table name and hoping.

Today, most of that knowledge is not written down anywhere a person or a program can query. It is scattered across Slack threads, stale wiki pages and the heads of a few senior people. Every decision made without it is a guess, and a confident guess is worse than no answer at all. We call this **context starvation**.

Marmot exists to end it. It gathers the metadata your systems already have, adds the ownership and business meaning only your team can supply, and serves the result as one governed, queryable source of truth that people and agents share. When the context is real, current and one query away, anyone can act on their own.

## What we stand for

<FeatureGrid>
  <FeatureCard
    title="Open source, for real"
    description="The core is MIT licensed with no usage limits. Self-host it for free, forever, and read the code to check what it collects."
    icon="mdi:source-branch-check"
    href="https://github.com/marmotdata/marmot"
  />
  <FeatureCard
    title="Simple to run"
    description="One Go binary and PostgreSQL. No search cluster, no message broker, no platform team. Search, scheduling and storage all live in Postgres."
    icon="mdi:cube-outline"
    docId="Deploy/index"
  />
  <FeatureCard
    title="Metadata, never your data"
    description="Marmot stores schemas, ownership, descriptions, lineage and statistics. The rows, messages and payloads behind your assets never enter it."
    icon="mdi:shield-check-outline"
    docId="Plugins/index"
  />
  <FeatureCard
    title="Vendor neutral"
    description="Postgres, Kafka, S3, dbt, Airflow, BigQuery and your own services in one graph. A catalog that sees only one vendor is a partial picture."
    icon="mdi:vector-combine"
    href="https://plugins.marmotdata.io"
  />
  <FeatureCard
    title="Agents are first class"
    description="The MCP server and SDKs ship in the core. Agents read the same governed context your team sees, and write back the lineage they create."
    icon="mdi:robot-outline"
    docId="Agents/index"
  />
  <FeatureCard
    title="Your way in"
    description="Plugins, Terraform, Pulumi, a Kubernetes operator, the CLI, the API or the UI. Marmot has no opinion about how the catalog gets filled."
    icon="mdi:door-open"
    docId="Populating/index"
  />
</FeatureGrid>

## What Marmot does

Marmot is built entirely in Go, with PostgreSQL as its only external dependency. Plugins and integrations read the structure of your sources into the catalog. People and rules enrich it with ownership, glossary terms, tags and custom fields. The UI, the REST API and the MCP server then answer questions from that one shared model.

<IntroDiagram />

<FeatureGrid>
  <FeatureCard
    title="Discovery"
    description="Find any asset in seconds with free text, or be precise with a query language that filters on type, owner, tags and metadata."
    icon="mdi:magnify"
    docId="queries"
  />
  <FeatureCard
    title="Lineage and impact"
    description="Trace how data flows and what depends on what, from plugins, OpenLineage events or the lineage your agents write back."
    icon="mdi:source-branch"
    docId="open-lineage"
  />
  <FeatureCard
    title="Ownership and meaning"
    description="Owners, business definitions, tags and custom fields turn raw assets into answers instead of guesses."
    icon="mdi:tag-text-outline"
    docId="glossary"
  />
  <FeatureCard
    title="Data products"
    description="Group related assets into curated bundles or dynamic rules that grow with your catalog."
    icon="mdi:package-variant-closed"
    docId="data-products"
  />
  <FeatureCard
    title="Asset rules"
    description="Define an enrichment once and Marmot applies it to every matching asset, including the ones that arrive later."
    icon="mdi:auto-fix"
    docId="asset-rules"
  />
  <FeatureCard
    title="Notifications"
    description="Hear about schema changes, pipeline runs and mentions in the UI or through webhooks."
    icon="mdi:bell-outline"
    docId="Notifications/index"
  />
</FeatureGrid>

## How you can run it

Everything in these docs applies to every edition. The data model, the query language, the plugins and the MCP server are the same wherever Marmot runs.

<DocCardGrid>
  <DocCard
    title="Self-hosted"
    description="Docker Compose, Helm or a single binary, on AWS, Google Cloud, Azure, OVHcloud or your own hardware. Nothing leaves your network."
    docId="Deploy/index"
    icon="mdi:server"
  />
</DocCardGrid>

<CalloutCard
  title="Building for a regulated environment?"
  description="Run Marmot inside your own VPC, or talk to us about an enterprise plan with data residency, VPC peering and audit export on Marmot Cloud."
  href="/pricing#contact"
  buttonText="Talk to us"
  variant="secondary"
  icon="mdi:shield-lock-outline"
/>

## Marmot for agents

AI agents are only as good as the context they can reach. Marmot gives them a live, governed map of your stack through a built-in MCP server, ready-made tools for agent frameworks, and typed SDKs in Python, Go and TypeScript.

<DocCardGrid>
  <DocCard
    title="Marmot for Agents"
    description="Plug your LLM agents into the catalog: they read it for context and write back the lineage they generate."
    docId="Agents/index"
    icon="mdi:robot-outline"
  />
  <DocCard
    title="MCP Server"
    description="Let Claude, Cursor, ChatGPT and any MCP client answer questions backed by your real catalog."
    docId="MCP/index"
    icon="mdi:protocol"
  />
  <DocCard
    title="SDK"
    description="Typed clients for Python, Go and TypeScript that authenticate from the environment automatically."
    docId="SDK/index"
    icon="mdi:code-braces"
  />
  <DocCard
    title="REST API"
    description="The HTTP API everything else is built on. Use it directly from any language."
    docId="api-reference"
    icon="mdi:api"
  />
</DocCardGrid>

## Find your way around

<DocCardGrid>
  <DocCard
    title="Quick Start"
    description="From an empty deployment to a populated catalog, step by step."
    docId="quick-start"
    icon="mdi:rocket-launch-outline"
  />
  <DocCard
    title="Populating your catalog"
    description="Plugins, Terraform, Pulumi, the Kubernetes operator, the CLI, the API and the UI."
    docId="Populating/index"
    icon="mdi:database-import-outline"
  />
  <DocCard
    title="Plugins"
    description="Auto-discover assets and lineage from the systems you already run."
    docId="Plugins/index"
    icon="mdi:puzzle-outline"
  />
  <DocCard
    title="Configure"
    description="Authentication, anonymous access, TLS, banners, Elasticsearch and telemetry."
    docId="Configure/index"
    icon="mdi:cog-outline"
  />
  <DocCard
    title="CLI Reference"
    description="Manage assets, ingest from sources and talk to any instance from the terminal."
    docId="cli"
    icon="mdi:console"
  />
  <DocCard
    title="Build a plugin"
    description="Write a plugin in Go with the plugin SDK and ship it in its own repository."
    docId="Develop/creating-plugins"
    icon="mdi:hammer-wrench"
  />
</DocCardGrid>

## Join us

Marmot is shaped by the people who run it. Questions, bug reports, plugin ideas and pull requests are all welcome.

<DocCardGrid>
  <DocCard
    title="Discord"
    description="Get help, share feedback and talk to the team and other Marmot users."
    href="https://discord.gg/TWCk7hVFN4"
    icon="mdi:forum-outline"
  />
  <DocCard
    title="GitHub"
    description="Star the project, open an issue or send a pull request."
    href="https://github.com/marmotdata/marmot"
    icon="mdi:github"
  />
  <DocCard
    title="Blog"
    description="Engineering writing on catalogs, context for AI, lineage and running on just Postgres."
    href="/blog"
    icon="mdi:post-outline"
  />
  <DocCard
    title="Security and bug bounty"
    description="How to report a vulnerability, and how we reward the people who do."
    href="/security"
    icon="mdi:bug-outline"
  />
</DocCardGrid>
