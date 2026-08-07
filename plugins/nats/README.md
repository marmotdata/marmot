The NATS plugin discovers JetStream streams from NATS servers. It connects using the NATS client protocol and enumerates streams via the JetStream API, collecting configuration and runtime state for each stream.

## Requirements

- **JetStream must be enabled** on the NATS server (start with `-js` flag or configure in `nats-server.conf`). Core NATS subjects are ephemeral and not discoverable as persistent assets.
- The connecting user needs permission to access the JetStream API (`$JS.API.>`).

## Authentication

The plugin supports several authentication methods:

- **Token**: Set the `token` field for token-based auth.
- **Username/Password**: Set `username` and `password` fields.
- **Credentials file**: Set `credentials_file` to the path of a `.creds` file (NKey-based auth).
- **TLS**: Enable `tls` for encrypted connections. Use `tls_insecure` to skip certificate verification in development.
