---
title: Confluent Cloud
description: Discover Kafka topics from Confluent Cloud clusters.
status: experimental
---

# Confluent Cloud

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span></div>
</div>
</div>

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Configure in the UI"
  description="This plugin can be configured directly in the Marmot UI with a step-by-step wizard."
  href="/docs/Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The Confluent Cloud plugin discovers Kafka topics from Confluent Cloud clusters. It uses the same discovery engine as the Kafka plugin with defaults tuned for Confluent Cloud.

## Connection

Confluent Cloud requires SASL/SSL authentication with an API key pair. You can create API keys in the Confluent Cloud Console.

```yaml
bootstrap_servers: "pkc-xxxxx.us-west-2.aws.confluent.cloud:9092"
client_id: "marmot-discovery"
authentication:
  type: "sasl_ssl"
  username: "your-api-key"
  password: "your-api-secret"
  mechanism: "PLAIN"
tls:
  enabled: true
```

## Schema Registry

If your Confluent Cloud environment has Schema Registry enabled, add the following to pull schema metadata:

```yaml
schema_registry:
  url: "https://psrc-xxxxx.us-west-2.aws.confluent.cloud"
  enabled: true
  config:
    basic.auth.user.info: "sr-key:sr-secret"
```

