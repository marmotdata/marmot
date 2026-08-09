The Redpanda plugin discovers topics from Redpanda clusters. It uses the same discovery engine as the Kafka plugin since Redpanda is Kafka API-compatible.

Because it is the same engine, topics are catalogued under the Kafka provider and addressed as `mrn://topic/kafka/<topic>`, not under a Redpanda provider. Running Kafka, Redpanda and Confluent Cloud against clusters that share a topic name will therefore land them on one asset.

## Connection

### Redpanda Cloud

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

### Self-Hosted Redpanda

```yaml
bootstrap_servers: "redpanda-0.example.com:9092,redpanda-1.example.com:9092"
client_id: "marmot-discovery"
```
