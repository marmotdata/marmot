---
sidebar_position: 1
---

# Introduction

Marmot is the open source **context layer** for your whole stack: a single catalog for every asset your systems and teams depend on, from services, APIs, queues, topics and brokers to databases, tables and pipelines. It exists to solve **context starvation**, the moment an engineer or an AI agent has to act without knowing what exists, who owns it, what it means, or what it connects to.

Marmot is built so both **humans and agents** can ask that question and get a real answer. [Catalog your assets](Populating/index.md) once, enrich them with ownership and business context, and expose them through the UI, a [REST API](api-reference.md), and a built-in [MCP server](MCP/index.md) that lets AI agents read your catalog, then write back the [lineage](open-lineage.md) they generate.

<div style={{maxWidth: '480px'}}>
<iframe width="100%" style={{aspectRatio: '16 / 9', border: 'none', borderRadius: '12px'}} src="https://www.youtube.com/embed/_JBcQGj_bFU" title="Marmot Demo" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowFullScreen></iframe>
</div>

import { CalloutCard, DocCard, DocCardGrid, FeatureCard, FeatureGrid } from '@site/src/components/DocCard';

<CalloutCard
  title="See Marmot in Action"
  description="Explore the interface and features with the interactive demo - no installation required."
  href="https://demo.marmotdata.io"
  buttonText="Try Live Demo"
  icon="mdi:rocket-launch"
/>

## Built for agents

AI agents are only as good as the context they can reach. Through a built-in MCP server and our SDKs, Marmot gives your agents a live, governed map of every asset in your stack: what exists, who owns it, what it means, and how it all connects.

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
</DocCardGrid>

## Why Marmot?

Most catalogs were built to help a data team document tables. Marmot is built to feed context to whoever needs it, human or agent, across every kind of asset: services, APIs, queues, topics, brokers, databases, tables and pipelines.

That means agents are first class, not an afterthought. A native MCP server and our SDKs are part of the core, so your agents read the same governed context your team does. And Marmot stays light enough to actually adopt: a single binary backed only by PostgreSQL, with no platform team required.

## Architecture

Marmot is built entirely in Go with PostgreSQL being the only external dependency, handling search, job scheduling and metadata storage. Unlike traditional catalogs that have opinionated ingestion methods, Marmot lets you populate your catalog however you like.

<div style={{textAlign: 'center', margin: '2rem 0'}}>
  <img src="/img/marmot-diagram.png" alt="Marmot architecture diagram" style={{maxWidth: '100%', borderRadius: '8px'}} />
</div>

## What Marmot stores

Marmot is a context layer, so it stores **metadata about your assets**, not the data inside them. That means schemas, field names and types, ownership, descriptions, tags, lineage and statistics. The rows in your tables, the messages on your topics and the payloads behind your APIs never enter Marmot.

Plugins read a source's structure and metadata into PostgreSQL; the data itself never moves. The easiest path is to run it on our platform, isolated per customer and under strict access controls. Need everything to stay within your control? Run Marmot yourself, free or with an enterprise license, in your own cloud on AWS, Google Cloud, Azure, OVHcloud or anywhere else.

<CalloutCard
  title="Building for a regulated environment?"
  description="Run it yourself so nothing leaves your VPC, and read the source to verify exactly what is collected."
  href="/pricing#contact"
  buttonText="Talk to us"
  variant="secondary"
  icon="mdi:shield-check-outline"
/>

## Features

Everything you need to turn scattered assets into a context layer that humans and agents can both query.

<FeatureGrid>
  <FeatureCard
    title="Discovery for humans and agents"
    description="Find any asset across your whole stack in seconds, from the UI or straight through MCP."
    icon="mdi:magnify"
    docId="queries"
  />
  <FeatureCard
    title="Lineage and impact"
    description="Trace how data flows and what depends on what, so people and agents can reason about change before they make it."
    icon="mdi:source-branch"
    docId="open-lineage"
  />
  <FeatureCard
    title="Context that gives meaning"
    description="Ownership, business definitions, tags and custom fields turn raw assets into answers, not guesses."
    icon="mdi:tag-text-outline"
    docId="glossary"
  />
  <FeatureCard
    title="Every asset, one catalog"
    description="Services, APIs, queues, topics, brokers, databases, tables and pipelines, all in a single context layer."
    icon="mdi:shape-outline"
    docId="Populating/index"
  />
  <FeatureCard
    title="Built for agents"
    description="A native MCP server and our SDKs let your agents read context and write back the lineage they generate."
    icon="mdi:robot-outline"
    docId="Agents/index"
  />
  <FeatureCard
    title="Data products"
    description="Group related assets into curated bundles or dynamic rules that grow with your catalog."
    icon="mdi:package-variant-closed"
    docId="data-products"
  />
</FeatureGrid>

## Get started

Pick a starting point. The [Quick Start](quick-start.md) walks you from an empty deployment to a populated catalog, step by step.

<DocCardGrid>
  <DocCard
    title="Quick Start"
    description="Spin up Marmot with Docker Compose in a couple of minutes, then start cataloging."
    docId="quick-start"
    icon="mdi:rocket-launch-outline"
  />
  <DocCard
    title="Marmot for agents"
    description="Connect your agents through the built-in MCP server and our SDKs."
    docId="Agents/index"
    icon="mdi:robot-outline"
  />
</DocCardGrid>

<CalloutCard
  title="Join the Community"
  description="Get help, share feedback, and connect with other Marmot users on Discord."
  href="https://discord.gg/TWCk7hVFN4"
  buttonText="Join Discord"
  variant="secondary"
  icon="mdi:account-group"
/>
