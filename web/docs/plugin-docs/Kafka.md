The Kafka plugin discovers topics from Kafka clusters. It captures topic configurations, partition details, and schema information from Schema Registry.

:::tip Looking for a managed service?
Marmot has dedicated plugins for [Confluent Cloud](/docs/Plugins/Confluent%20Cloud) and [Redpanda](/docs/Plugins/Redpanda) with pre-configured defaults.
:::

## Connection Examples

import { Collapsible } from "@site/src/components/Collapsible";

<Collapsible title="Self-Hosted with SASL" icon="mdi:server">

```yaml
bootstrap_servers: "kafka-1.prod.com:9092,kafka-2.prod.com:9092"
client_id: "marmot-discovery"
authentication:
  type: "sasl_ssl"
  username: "your-username"
  password: "your-password"
  mechanism: "SCRAM-SHA-512"
tls:
  enabled: true
  ca_cert_path: "/path/to/ca.pem"
  cert_path: "/path/to/client.pem"
  key_path: "/path/to/client-key.pem"
```

</Collapsible>
