The Google Pub/Sub plugin discovers topics and subscriptions from a Google Cloud project, along with the links between them and the systems a subscription exports to.

Topics carry their labels, message storage policy, retention and schema. Subscriptions carry their delivery type (pull, push, BigQuery or Cloud Storage), acknowledgement settings, filter and dead letter policy.

## Authentication

With neither `credentials_file` nor `credentials_json` set, Application Default Credentials are used. The service account needs `roles/pubsub.viewer`; without it, schemas are skipped and discovery continues.

## Emulator

Set `emulator_host` to a `host:port` address to run against the Pub/Sub emulator. The connection then uses plaintext gRPC with no credentials, and assets get no Google Cloud console links.

## Sample Messages

With `include_sample_messages: true`, asset previews read up to 20 messages from a pull subscription for 5 seconds. Every message is nacked, so Pub/Sub redelivers it to the real consumer straight after.
