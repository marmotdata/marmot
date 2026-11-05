---
sidebar_position: 1
title: Marmot Cloud
description: Marmot run for you, with access control per resource, secret stores that read from your own vault and pipelines that need no stored credential.
---

import { DocCard, DocCardGrid } from '@site/src/components/DocCard';

# Marmot Cloud

Marmot Cloud runs Marmot for you on a hostname of its own, with the database, backups and TLS managed. The catalog, the query language, the plugins and the MCP server work as they do everywhere else in these docs. Cloud adds access control per resource, secret stores that read from the vault you already run and pipelines that sign in to your cloud with no stored credential.

<DocCardGrid>
  <DocCard
    title="Get started"
    description="Launch an instance and catalog your first source"
    docId="Cloud/getting-started"
    icon="mdi:flag-checkered"
  />
  <DocCard
    title="Instances"
    description="The console, compute units, plans and the lifecycle of an instance"
    docId="Cloud/instances"
    icon="mdi:server"
  />
  <DocCard
    title="Invite your team"
    description="Add people to your organisation and choose who runs billing"
    docId="Cloud/members"
    icon="mdi:account-multiple-plus"
  />
  <DocCard
    title="Access control"
    description="Grant roles on the whole catalog or on one data product, asset or source"
    docId="Cloud/access-control"
    icon="mdi:shield-key"
  />
  <DocCard
    title="Query access"
    description="Choose which sources answer queries and who can run them"
    docId="Cloud/query-access"
    icon="mdi:database-lock"
  />
  <DocCard
    title="Single sign-on"
    description="Let your team sign in through your identity provider"
    docId="Cloud/single-sign-on"
    icon="mdi:login-variant"
  />
  <DocCard
    title="Pipelines"
    description="How pipelines sign in to your cloud without a stored key"
    docId="Cloud/pipelines"
    icon="mdi:key-remove"
  />
  <DocCard
    title="Connect your cloud"
    description="Catalog Google Cloud, AWS or Azure with no credential"
    docId="Cloud/Guides/google"
    icon="mdi:cloud-sync"
  />
  <DocCard
    title="Secret stores"
    description="Read passwords from your own secret manager at run time"
    docId="Cloud/secret-stores"
    icon="mdi:safe"
  />
  <DocCard
    title="Private plugins"
    description="Publish your own plugins to your instance"
    docId="Cloud/plugin-registry"
    icon="mdi:package-variant-closed"
  />
  <DocCard
    title="Security and isolation"
    description="How instances are kept apart and what Marmot stores"
    docId="Cloud/security"
    icon="mdi:shield-lock"
  />
</DocCardGrid>
