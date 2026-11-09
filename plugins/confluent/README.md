The Confluent Cloud plugin discovers Kafka topics from Confluent Cloud clusters. It uses the same discovery engine as the Kafka plugin with defaults tuned for Confluent Cloud.

Because it is the same engine, topics are catalogued under the Kafka provider and addressed as `mrn://asset/topic/kafka@<zone>/<topic>`, not under a Confluent provider. Set `zone` to tell clusters apart (a name you choose, not a cloud availability zone); clusters left without a zone that share a topic name will land on one asset.

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
