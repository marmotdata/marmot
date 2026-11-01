---
sidebar_position: 1
description: What Marmot is, how it fits into your stack, and where to start in the documentation.
---

import { DocCard, DocCardGrid } from '@site/src/components/DocCard';
import IntroDiagram from '@site/src/components/IntroDiagram';

# Introduction

Marmot gives people and AI agents one governed way in to your data. It catalogs the tables, topics, dashboards and pipelines you already run, records what each one means and who owns it, and serves that context in the web UI, over the API and to agents through its [MCP server](MCP/index.md).

## Start here

<DocCardGrid>
  <DocCard
    title="Quick start"
    description="Run Marmot locally with Docker Compose and catalog your first assets in about five minutes."
    docId="quick-start"
    icon="mdi:rocket-launch-outline"
  />
  <DocCard
    title="Marmot Cloud"
    description="Launch a managed instance in about a minute. The free plan needs no card."
    docId="Cloud/getting-started"
    icon="mdi:cloud-outline"
  />
</DocCardGrid>

## What you can do with it

<DocCardGrid>
  <DocCard
    title="Find the right asset"
    description="Search every source in plain words, then narrow the results with the query language."
    docId="queries"
    icon="mdi:magnify"
  />
  <DocCard
    title="Explain what it means"
    description="Owners, descriptions, glossary terms and data products travel with every asset."
    docId="glossary"
    icon="mdi:book-open-variant"
  />
  <DocCard
    title="See where data flows"
    description="Lineage from your plugins and OpenLineage runs, upstream and downstream."
    docId="open-lineage"
    icon="mdi:graph-outline"
  />
  <DocCard
    title="Decide who can read what"
    description="Roles for people, teams and agents. Marmot Cloud adds grants per table, data product and term."
    docId="Configure/access-control"
    icon="mdi:shield-account-outline"
  />
  <DocCard
    title="Give agents context"
    description="Connect Claude, Cursor and any MCP client to the catalog, or build on the SDKs."
    docId="Agents/index"
    icon="mdi:robot-outline"
  />
  <DocCard
    title="Keep it current"
    description="More than 60 plugins read your sources on a schedule, and asset rules enrich what arrives."
    docId="Plugins/index"
    icon="mdi:puzzle-outline"
  />
</DocCardGrid>

## How it fits together

Plugins read metadata from the systems you already run into one catalog: schemas, owners, descriptions and lineage, never the rows themselves. People use it in the web UI, agents through the MCP server and your own code through the API, CLI and SDKs, so everyone works from the same answers.

<IntroDiagram />

## Where to run it

Marmot is one Go binary backed by PostgreSQL, and the core is open source under the MIT licence. Everything in these docs applies wherever it runs: the data model, the query language, the plugins and the MCP server are the same in every edition.

<DocCardGrid>
  <DocCard
    title="Marmot Cloud"
    description="We run it for you, with access control per resource, secret stores and keyless pipelines."
    docId="Cloud/index"
    icon="mdi:cloud-outline"
  />
  <DocCard
    title="Run it yourself"
    description="Docker Compose, Helm or the single binary, on any cloud or your own hardware."
    docId="Deploy/index"
    icon="mdi:server"
  />
</DocCardGrid>

## Get help

<DocCardGrid>
  <DocCard
    title="Discord"
    description="Ask questions and talk to the team and other people running Marmot."
    href="https://discord.gg/TWCk7hVFN4"
    icon="mdi:forum-outline"
  />
  <DocCard
    title="GitHub"
    description="Open an issue, read the code or send a pull request."
    href="https://github.com/marmotdata/marmot"
    icon="mdi:github"
  />
  <DocCard
    title="Blog"
    description="Guides and engineering writing from the Marmot team."
    href="/blog"
    icon="mdi:post-outline"
  />
  <DocCard
    title="Report a vulnerability"
    description="How to tell us about a security issue, privately."
    href="/security"
    icon="mdi:shield-bug-outline"
  />
</DocCardGrid>
