---
sidebar_position: 1
---

# Introduction

Marmot is an open-source data catalog designed for teams who want powerful data discovery without enterprise complexity. Built with a focus on simplicity and speed, Marmot helps you catalog assets across your entire data stack - from databases and APIs to message queues and data pipelines.

import { CalloutCard, DocCard, DocCardGrid, FeatureCard, FeatureGrid } from '@site/src/components/DocCard';

<CalloutCard
  title="See Marmot in Action"
  description="Explore the interface and features with the interactive demo - no installation required."
  href="https://demo.marmotdata.io"
  buttonText="Try Live Demo"
  icon="mdi:rocket-launch"
/>

## Why Marmot?

Unlike traditional catalogs that require extensive infrastructure and configuration, Marmot ships as a **single binary** with an intuitive UI, making it easy to deploy and start cataloging in minutes.

<FeatureGrid>
  <FeatureCard
    title="Deploy in Minutes"
    description="Single binary, Docker, or Kubernetes - no complex setup required"
    icon="mdi:lightning-bolt"
  />
  <FeatureCard
    title="Powerful Search"
    description="Query language with full-text, metadata, and boolean operators"
    icon="mdi:magnify"
  />
  <FeatureCard
    title="Track Lineage"
    description="Interactive dependency graphs to understand data flows and impact"
    icon="mdi:source-branch"
  />
  <FeatureCard
    title="Flexible Integrations"
    description="CLI, REST API, Terraform, and Pulumi - catalog assets your way"
    icon="mdi:puzzle"
  />
</FeatureGrid>

## Features

### Search Everything

Find any data asset across your entire organisation in seconds. Combine full-text search with structured queries using metadata filters, boolean logic, and comparison operators.

### Interactive Lineage Visualisation

Trace data flows from source to destination with interactive dependency graphs. Understand upstream and downstream dependencies, identify bottlenecks, and analyse impact before making changes.

### Metadata-First Architecture

Store rich metadata for any asset type. From tables and topics to APIs and dashboards - if it matters to your data stack, you can catalog it in Marmot.

### Team Collaboration

Assign ownership, document business context, and create glossaries. Keep your entire team aligned with centralised knowledge about your data assets.

### Data Products

Group related assets into logical collections. Use manual assignment for curated bundles or dynamic rules that use the query language to automatically include matching assets as your catalog grows.

## Getting Started

Ready to dive in? Here's where to go next:

<DocCardGrid>
  <DocCard
    title="Populating Your Catalog"
    description="Learn how to add assets using plugins, CLI, or API"
    href="/docs/Populating"
    icon="mdi:database-plus"
  />
  <DocCard
    title="Data Products"
    description="Group assets into logical collections with manual or dynamic rules"
    href="/docs/data-products"
    icon="mdi:package-variant-closed"
  />
  <DocCard
    title="Glossary"
    description="Define business terms and create a shared vocabulary"
    href="/docs/glossary"
    icon="mdi:book-alphabet"
  />
  <DocCard
    title="Query Language"
    description="Use Marmot's powerful search capabilities"
    href="/docs/queries"
    icon="mdi:code-tags"
  />
  <DocCard
    title="Deployment Options"
    description="Deploy to production with Docker, Helm, or the CLI"
    href="/docs/Deploy"
    icon="mdi:cloud-upload"
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
