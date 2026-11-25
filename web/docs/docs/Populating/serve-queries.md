---
title: Serve queries from your network
description: Run a runner next to a database in your own network, so Marmot can query it without holding its credentials or connecting in.
---

import { Steps, Step, Tabs, TabPanel } from '@site/src/components/Steps';

# Serve queries from your network

Run a runner next to a database, so people, agents and tasks can query it through Marmot while its credentials stay in your network. The runner connects out to Marmot, and Marmot never connects in. [Runners](../Configure/serving-plugins.md) explains how it works and what it trusts.

## Before you start

- Your Marmot is reachable over HTTPS from the machine that will run the runner. On Marmot Cloud it is.
- The machine can reach Marmot on port 443 and on the serve port, 8443, and can reach the database.
- The [Marmot CLI](../Deploy/CLI.md) is installed on that machine.
- You have a database account that can only read the tables you want to expose. On PostgreSQL, a role granted `SELECT` on those tables or `pg_read_all_data`.

## Serve the source

<Steps>
<Step title="Turn on serving (self-hosted only)">

On Marmot Cloud, serving is already on; skip this step. A self-hosted server needs it turned on in its config, along with the [encryption key](../Deploy/Helm.md#encryption-key) that protects the serving keys:

```yaml
query:
  serve:
    enabled: true
```

On Kubernetes, set `config.query.serve.enabled: true` in the Helm values. The chart opens the port and creates a `<release>-serve` Service. See [Networking](#networking) for exposing it.

</Step>
<Step title="Create a credential for the runner">

The runner authenticates like any CLI. Give it a [service account](../Cloud/service-accounts.md) of its own, with a role that grants **Serve ingestion** (`ingestion:serve`) and the ingestion permissions it needs to push results. Admins hold both already. Revoking this credential later is how you cut the runner off.

</Step>
<Step title="Write the pipeline">

Mark the run as queryable and read the password from the environment:

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

The source takes the pipeline's name, so this one is queried as `warehouse`.

</Step>
<Step title="Optional: limit what the runner serves">

A policy file sets limits that hold even if Marmot is misconfigured. Once you give one, any source it doesn't name is refused:

```yaml
sources:
  warehouse:
    max_rows: 5000
    actors: ["user:*", "service_account:support-triage-agent"]
```

Every key is in the [runner reference](../Configure/serving-reference.md#policy-file).

</Step>
<Step title="Start the runner">

Run it wherever the CLI runs, or let the Kubernetes operator run it for you:

<Tabs items={['CLI', 'Kubernetes operator']}>
<TabPanel>

Set the credentials, then start it:

```bash
export MARMOT_HOST=https://<instance>.marmotdata.cloud
export MARMOT_API_KEY=<api-key>
export PG_PASSWORD=<database-password>
marmot plugin -c warehouse.yaml --serve \
  --policy serve-policy.yaml \
  --audit-log /var/log/marmot/serve-audit.jsonl
```

Use a systemd unit, a container or a pod to keep it running. Stop it with `SIGTERM`.

</TabPanel>
<TabPanel>

Set `serve: true` on a `Run`. The operator runs it as a single-replica Deployment that ingests when it starts, then serves:

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

A serving `Run` can't have a `schedule`. Its pod authenticates with a token only Marmot accepts and doesn't mount a Kubernetes API token.

</TabPanel>
</Tabs>

`MARMOT_HOST` must use HTTPS, even for `localhost`, because the runner sends its credential there and learns which keys to trust from the reply. For a private CA, pass it with `--ca-file`.

</Step>
<Step title="Optional: check Marmot's CA on first connect">

The runner trusts the keys in its first reply from Marmot. To check that reply too, copy the CA fingerprint from Marmot:

```bash
marmot admin serve-keys
```

Then pass it when you start the runner:

```bash
marmot plugin -c warehouse.yaml --serve \
  --ca-fingerprint sha256:<fingerprint>
```

</Step>
</Steps>

## Check that it worked

Open **Plugins** in Marmot. The `warehouse` source shows **Connected**. Run a query against it from **Query**, then open **Query audit**: the query's receipt shows `verified`.

## Networking

Traffic only flows out of your network:

| From | To | Port |
| --- | --- | --- |
| Runner | Marmot API | 443 |
| Runner | Marmot serve port | 8443 |
| Runner | Your database | Its usual port |
| Marmot replica | Other replicas, self-hosted only | 8443 |

On a self-hosted server, put only a TCP load balancer or TLS passthrough in front of the serve port, because the runner's mutual TLS runs all the way to Marmot. Expose it with `serveService` in the Helm values, and set `public_address` when it has a different host name to the API:

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

To keep a runner pod to the traffic it needs, give it a NetworkPolicy with no ingress and egress only to Marmot, DNS and the database.

## Stop a runner

Each of these cuts a runner off within 15 seconds:

- Delete or expire the API key it enrolled with.
- Deactivate its service account, or remove **Serve ingestion** from it.
- Delete the source in Marmot. The runner claims the name again next time it connects unless its credential is revoked too.

## Rotate and protect the keys

This section applies to self-hosted servers. Marmot Cloud manages these keys for you.

Rotate the serving keys with:

```bash
marmot admin rotate-serve-keys
```

A rotation is graceful: runners learn the new keys before they're used and keep serving. If a key may have been compromised, add `--immediate`, then restart every runner, since they won't accept keys they were never told about.

To keep the keys out of Marmot's database, name KMS keys in the server config. Each is an ECDSA P-256 key, and the approval key must be its own key:

```yaml
query:
  serve:
    enabled: true
    ca_key_uri: "awskms:///<key-arn>"
    signing_key_uri: "awskms:///<key-arn>"
    approval_key_uri: "awskms:///<key-arn>"
```

AWS KMS, Google Cloud KMS, Azure Key Vault and HashiCorp Vault are supported; the URI formats are in the [runner reference](../Configure/serving-reference.md#server-configuration).

## Troubleshooting

| What you see | What to do |
| --- | --- |
| The source shows **Not connected** | Check the runner's log. It needs to reach Marmot on 443 and 8443, and its credential needs **Serve ingestion**. |
| `x509: certificate signed by unknown authority` from the runner | Something between the runner and Marmot terminates TLS. Use a TCP load balancer or TLS passthrough for the serve port. |
| The runner stops on first start with a fingerprint error | The `--ca-fingerprint` value doesn't match the Marmot it reached. Copy it again from `marmot admin serve-keys`. |
| A query fails with a message naming the policy file | The runner's policy refused it. Add the source, actor or origin to the policy and restart the runner. |
| An API key is refused for serving | The key predates indexed keys. Create a new key. |

## Next steps

- [Let tasks act on a Kubernetes cluster](kubernetes-tools.md): add reads, tools and triggers to a runner.
- [Control who can query data](../Cloud/query-access.md): grant query access on the source.
- [Runner reference](../Configure/serving-reference.md): every flag and policy key.
