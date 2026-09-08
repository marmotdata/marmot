The Bigtable plugin discovers instances, tables and column families from Google Cloud Bigtable.

Tables are named `<instance>.<table>`, because a table name is only unique within an instance. Each instance holds its tables through CONTAINS lineage.

## Columns

Bigtable declares column families but not the qualifiers under them, and every value is raw bytes. The plugin reads `sample_rows` rows per table and records the `family:qualifier` pairs it saw, how many sampled rows held each one, and whether the values looked like text, an 8 byte number, or binary. Set `include_columns` to false to skip the sample.

## Emulator

`emulator_host` connects to a local emulator without TLS or credentials. The emulator cannot list its own instances, so `instances` has to be listed alongside it. The plugin never reads `BIGTABLE_EMULATOR_HOST`, so one pipeline cannot change how another one connects.

## Tests

Unit tests need nothing. The end to end tests need an emulator:

```
docker run -d --name bigtable-emulator -p 8086:8086 \
  gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators \
  gcloud beta emulators bigtable start --host-port=0.0.0.0:8086

MARMOT_TEST_BIGTABLE_EMULATOR=127.0.0.1:8086 go test ./...
```
