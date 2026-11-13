---
title: TLS
description: Serve Marmot over HTTPS with your own certificate, and require client certificates with mutual TLS.
---

# TLS

Serve Marmot over HTTPS with your own certificate. Without a `tls` section the server stays on plain HTTP, which is fine behind a load balancer that terminates TLS.

## Use a certificate

Point Marmot at a PEM certificate and its private key. Both are required:

```yaml
server:
  port: 8443
  tls:
    cert_path: "/etc/ssl/certs/marmot.pem"
    key_path: "/etc/ssl/private/marmot-key.pem"
```

Or with environment variables:

```bash
MARMOT_SERVER_TLS_CERT_PATH=/etc/ssl/certs/marmot.pem
MARMOT_SERVER_TLS_KEY_PATH=/etc/ssl/private/marmot-key.pem
```

## Require client certificates

To accept only clients that present a certificate signed by your CA, add `ca_cert_path`. Marmot then rejects every request without a valid client certificate:

```yaml
server:
  port: 8443
  tls:
    cert_path: "/etc/ssl/certs/marmot.pem"
    key_path: "/etc/ssl/private/marmot-key.pem"
    ca_cert_path: "/etc/ssl/certs/client-ca.pem"
```

## Options

| Key                       | Description                                            | Environment variable             |
| ------------------------- | ------------------------------------------------------ | -------------------------------- |
| `server.tls.cert_path`    | PEM-encoded server certificate                         | `MARMOT_SERVER_TLS_CERT_PATH`    |
| `server.tls.key_path`     | The certificate's private key                          | `MARMOT_SERVER_TLS_KEY_PATH`     |
| `server.tls.ca_cert_path` | CA certificate that client certificates must chain to  | `MARMOT_SERVER_TLS_CA_CERT_PATH` |

## Next steps

- [Configuration reference](index.md): every other server setting.
