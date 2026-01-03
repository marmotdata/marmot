The Kafka plugin discovers topics from Kafka clusters. It captures topic configurations, partition details, and schema information from Schema Registry.

## Connection Examples

import { Collapsible } from "@site/src/components/Collapsible";

<Collapsible title="Confluent Cloud" icon="simple-icons:confluent">

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
schema_registry:
  url: "https://psrc-xxxxx.us-west-2.aws.confluent.cloud"
  enabled: true
  config:
    basic.auth.user.info: "sr-key:sr-secret"
```

</Collapsible>

<Collapsible title="Redpanda Cloud" icon="simple-icons:redpanda">

```yaml
bootstrap_servers: "seed-xxxxx.cloud.redpanda.com:9092"
client_id: "marmot-discovery"
authentication:
  type: "sasl_ssl"
  username: "your-username"
  password: "your-password"
  mechanism: "SCRAM-SHA-256"
tls:
  enabled: true
```

</Collapsible>

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
