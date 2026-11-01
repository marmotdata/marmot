---
sidebar_position: 9
title: Serving Queries from Your Network
---

# Serving Queries from Your Network

A queryable source normally runs its plugin on the Marmot server, which means the server needs network access to the database and holds its credentials. When that is not possible or not wanted, run the plugin yourself, next to the database, and let it serve Marmot's queries from there. The plugin keeps the credentials, and Marmot never connects to your network: the plugin dials out to Marmot and answers queries over that connection.

The same command that pushes ingestion results serves queries, with one extra flag:

```bash
marmot plugin -c pipeline.yaml --serve
```

`marmot plugin` is another name for `marmot ingest`. It ingests as usual, then stays running and answers queries for the run marked `queryable: true`.

## Enabling It on the Server

Serving is off by default. Turn it on in the server config:

```yaml
query:
  serve:
    enabled: true
    listen_address: ":8443"
```

| Key                            | Description                                                                                                        | Default |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------ | ------- |
| `query.serve.enabled`          | Accept plugins that serve queries from outside Marmot                                                               | `false` |
| `query.serve.listen_address`   | Address the serve listener binds                                                                                    | `:8443` |
| `query.serve.public_address`   | `host:port` serving plugins dial, when that is not the Marmot host on the listen port                               | -       |
| `query.serve.replica_address`  | Host, or `host:port`, other replicas reach this one on to forward queries. Only needed with more than one replica   | -       |
| `query.serve.ca_key_uri`       | KMS key for the serving CA; see [Keeping the Keys in a KMS](#keeping-the-keys-in-a-kms)                             | -       |
| `query.serve.signing_key_uri`  | KMS key that signs every query                                                                                      | -       |

Serving requires the [encryption key](../Deploy/Helm.md#encryption-key), which protects the keys Marmot uses for serving connections. Marmot refuses to start with serving enabled and no key.

On Kubernetes, set `config.query.serve.enabled: true` in the Helm values. The chart opens the port, creates a `<release>-serve` Service and gives each replica the address its peers forward queries to.

## Running a Serving Plugin

Mark one run in the pipeline as queryable and keep its credentials in the environment:

```yaml
name: warehouse
runs:
  - postgresql:
      host: "db.internal"
      user: "marmot_reader"
      password: "${PG_PASSWORD}"
      query_database: "analytics"
      queryable: true
```

The served source takes the pipeline's name, so this one is queried as `warehouse`. A pipeline serves one queryable run; split others into their own pipelines.

The process authenticates with an ordinary Marmot credential, the same as `marmot ingest`. Use a service account with a role that grants **Serve ingestion** (`ingestion:serve`), which admins hold by default, plus the ingestion permissions it needs to push results:

```bash
export MARMOT_HOST=https://marmot.example.com
export MARMOT_API_KEY=...        # service account key
export PG_PASSWORD=...
marmot plugin -c warehouse.yaml --serve
```

`MARMOT_HOST` must use HTTPS, with no exception for `localhost`: the process sends its credential to that address and learns which keys to trust from the reply, and on a shared machine another user could be listening on a local port first. If Marmot's certificate comes from a private CA, pass it with `--ca-file` (or `MARMOT_CA_FILE`) rather than turning verification off.

Because it is the normal CLI, it runs anywhere the CLI runs: a VM under systemd, a Docker container, a pod you manage yourself or a CI runner. It needs outbound access to Marmot's API and serve port and to the database; it listens on no port. Stop it with `SIGTERM` or `SIGINT`.

### With the Kubernetes Operator

Set `serve: true` on a `Run`. The operator runs it as a single-replica Deployment instead of a Job, so it ingests when it starts and then serves:

```yaml
apiVersion: runs.marmotdata.io/v1alpha1
kind: Run
metadata:
  name: warehouse
spec:
  serve: true
  env:
    - name: PG_PASSWORD
      valueFrom:
        secretKeyRef:
          name: warehouse-db
          key: password
  runs:
    - postgresql:
        host: "db.internal"
        user: "marmot_reader"
        password: "${PG_PASSWORD}"
        queryable: true
```

A serving Run cannot have a `schedule`. Its pod authenticates with a ServiceAccount token projected for the `marmot` audience, which Marmot accepts and the Kubernetes API refuses; the pod does not mount the usual Kubernetes API token at all.

## Networking

Traffic only flows one way: the serving plugin opens a TCP connection to Marmot's serve port. Nothing connects into the network the plugin runs in.

| From           | To                                          | Port                   |
| -------------- | ------------------------------------------- | ---------------------- |
| Serving plugin | Marmot API                                  | 443 (or your API port) |
| Serving plugin | Marmot serve port                           | 8443                   |
| Serving plugin | Your database                               | Its usual port         |
| Marmot replica | Other Marmot replicas, on the serve port    | 8443                   |

The serve port carries mutual TLS from the plugin all the way to Marmot. Put only a TCP (layer 4) load balancer or TLS passthrough in front of it. A proxy or Ingress that terminates TLS breaks it, and the plugin reports a certificate error rather than connecting. With the Helm chart, expose the port on its own with `serveService`:

```yaml
serveService:
  type: LoadBalancer
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: nlb
config:
  query:
    serve:
      enabled: true
      public_address: "serve.marmot.example.com:8443"
```

When the serve port has a different host name to the API, set `public_address` so plugins know where to dial. With more than one Marmot replica, a plugin connects to one of them and the others forward its queries there, so replicas must reach each other on the serve port; the chart sets this up.

To keep a serving pod to exactly the traffic it needs, a NetworkPolicy like this allows egress to Marmot, DNS and the database and nothing inbound:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: warehouse-serve
spec:
  podSelector:
    matchLabels:
      runs.marmotdata.io/run: warehouse
  policyTypes: ["Ingress", "Egress"]
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: marmot
      ports:
        - port: 8080
        - port: 8443
    - to:
        - namespaceSelector: {}
          podSelector:
            matchLabels:
              k8s-app: kube-dns
      ports:
        - port: 53
          protocol: UDP
    - to:
        - ipBlock:
            cidr: 10.0.12.0/24
      ports:
        - port: 5432
```

## Giving the Database the Least Access

Marmot runs every statement through the plugin's read-only path, but the database account is what really decides what a query can do. Connect with an account that can only read the tables you want to expose. On PostgreSQL that means a role granted `SELECT` on those tables, or `pg_read_all_data`, rather than relying on read-only transactions, which a session can switch off.

## Stopping a Serving Plugin

Every Marmot replica rechecks its serving connections every 15 seconds, so any of these cuts a plugin off within that time:

- **Delete or expire the API key it enrolled with.** Each connection remembers which key created it.
- **Deactivate its service account or user, revoke the user's sessions or remove the Serve ingestion permission.**
- **Delete the served source** in Marmot. Queries for it stop at once, though the plugin claims the name again the next time it connects unless its credential is also revoked.

The check costs one database query per replica every 15 seconds however many keys you have, and nothing at all happens until a credential is actually revoked. API keys created before Marmot indexed keys cannot be watched this way, so they are refused for serving; create a new key.

## Rotating the Serving Keys

Marmot secures serving with two keys of its own: a certificate authority that issues the short-lived certificates, and a key that signs every query. Rotate them with:

```bash
marmot admin rotate-serve-keys
marmot admin serve-keys        # list generations and when they activate and retire
```

A rotation is graceful by default. The new keys are published at once but only used after 20 minutes, by which time every connected plugin has renewed and learnt them, and the old keys stop being trusted 20 minutes after that. Plugins carry on serving throughout.

If a key may have been compromised, rotate with `--immediate`. Marmot switches to new keys at once, stops trusting the old ones and closes every connection made under them. A running plugin will not accept keys it was never told about, so restart every serving plugin afterwards.

## Keeping the Keys in a KMS

By default the serving keys are generated by Marmot and stored in its database, encrypted with the server encryption key. To keep them out of Marmot entirely, create two ECDSA P-256 keys in a KMS and name them:

```yaml
query:
  serve:
    enabled: true
    ca_key_uri: "awskms:///arn:aws:kms:eu-west-1:111122223333:key/1234abcd-..."
    signing_key_uri: "awskms:///arn:aws:kms:eu-west-1:111122223333:key/5678efgh-..."
```

| KMS | URI | Credentials |
| --- | --- | --- |
| AWS KMS | `awskms:///<key ARN or alias ARN>` | The usual AWS credential chain, for example IRSA |
| Google Cloud KMS | `gcpkms://projects/<p>/locations/<l>/keyRings/<r>/cryptoKeys/<k>/cryptoKeyVersions/<v>` | Application default credentials or Workload Identity |
| Azure Key Vault | `azurekms://<vault>.vault.azure.net/<key>` | The Azure default credential chain |
| HashiCorp Vault transit | `hashivault://<key>` | `VAULT_ADDR` and `VAULT_TOKEN` |

These are the same key URIs cosign uses. Marmot then stores only the URIs and asks the KMS to sign each certificate and query, so a copy of Marmot's database and encryption key is no longer enough to impersonate Marmot to your plugins. The KMS keys are used when serving first starts; to move existing keys into a KMS, or to change KMS keys later, rotate with `--ca-key-uri` and `--signing-key-uri`. Marmot refuses a rotation that would take keys out of a KMS and back into its database.


## How It Is Secured

The plugin never listens on a port, so there is nothing for anyone to connect to. Everything else protects the one connection it makes to Marmot.

**Enrolment.** The process generates a private key in memory and sends Marmot only a certificate request, over HTTPS, authenticated with its Marmot credential. Marmot checks the credential holds Serve ingestion, records which credential it was, and issues a ten-minute client certificate from a certificate authority private to this Marmot instance. The process enrols again with a fresh key at half-life. The CA can only vouch for this Marmot instance's own names, never for real host names or IP addresses, so even a stolen CA key cannot mint certificates anything else would accept. Its key and the query signing key are either encrypted with the server encryption key or held in a KMS.

**Ownership.** The first credential to serve a source name owns it. No other credential can serve that name, and a name that belongs to a pipeline Marmot runs itself cannot be served at all, so a serve credential cannot take over someone else's source or receive their queries.

**Transport.** The connection is TLS 1.3 only, with certificates on both sides. Marmot accepts only certificates its own CA issued for a live enrolment whose key matches, and closes the connection when the certificate expires, its credential is revoked or the principal loses the right to serve. The plugin accepts only a server certificate from the CAs it pinned when it first enrolled, and accepts new keys only when Marmot announced them alongside ones it already trusts, during a graceful rotation. Every call over the connection, in both directions, is checked against the verified certificate of the caller, not only when the connection opens.

**Every query is signed.** Marmot signs each query with an ECDSA key the plugin pinned at enrolment. The signature covers the statement, row limit, source, the user who ran it, a one-minute expiry, a single-use nonce and the connection's TLS channel binding ([RFC 9266](https://www.rfc-editor.org/rfc/rfc9266)). The plugin checks all of these before the query reaches the database, so a query cannot be altered, replayed or moved to another connection, even by something that sits in the middle of the network path.

**Limits on both sides.** The plugin stops at the row limit Marmot signed whatever the database returns, and both sides cap a result at 256 MiB.

**Audit.** Marmot records every query in its query log as usual. The plugin logs each query it serves with the same query id, the user, a hash of the statement and the row count, so the two records can be matched.

**The plugin itself.** Marmot and the CLI refuse a plugin binary that another user could have written, check its SHA-256 on every launch, talk to it over mutual TLS on a private Unix socket and pass it the environment without Marmot's own `MARMOT_` variables. Core plugins are pinned by digest in each Marmot release and installed only by that digest.
