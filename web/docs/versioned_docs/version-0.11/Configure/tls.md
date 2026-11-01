# TLS

## Configuration

To enable TLS, provide a certificate and private key:

### YAML

```yaml
server:
  port: 8443
  tls:
    cert_path: "/etc/ssl/certs/marmot.pem"
    key_path: "/etc/ssl/private/marmot-key.pem"
```

### Environment Variables

```
MARMOT_SERVER_TLS_CERT_PATH=/etc/ssl/certs/marmot.pem
MARMOT_SERVER_TLS_KEY_PATH=/etc/ssl/private/marmot-key.pem
```

## Options

| Option                    | Description                                                                   | Default | Environment Variable             |
| ------------------------- | ----------------------------------------------------------------------------- | ------- | -------------------------------- |
| `server.tls.cert_path`    | Path to a PEM-encoded server certificate                                      | -       | `MARMOT_SERVER_TLS_CERT_PATH`    |
| `server.tls.key_path`     | Path to the server certificate's private key                                  | -       | `MARMOT_SERVER_TLS_KEY_PATH`     |
| `server.tls.ca_cert_path` | Path to a PEM-encoded CA certificate for verifying client certificates (mTLS) | -       | `MARMOT_SERVER_TLS_CA_CERT_PATH` |

Both `cert_path` and `key_path` are required when TLS is enabled. Omitting the `tls` section entirely keeps the server on plain HTTP.

## Mutual TLS (mTLS)

To require clients to present a valid certificate, add `ca_cert_path` pointing to the CA that signed your client certificates:

```yaml
server:
  port: 8443
  tls:
    cert_path: "/etc/ssl/certs/marmot.pem"
    key_path: "/etc/ssl/private/marmot-key.pem"
    ca_cert_path: "/etc/ssl/certs/client-ca.pem"
```

When `ca_cert_path` is set, the server requires and verifies a client certificate on every request. Clients that do not present a certificate signed by the specified CA will be rejected.
