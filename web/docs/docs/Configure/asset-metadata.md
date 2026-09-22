# Asset metadata profiles

An optional YAML profile describes editable asset fields. Native fields keep
their existing storage; additional fields live in the asset's `metadata`
JSON. The profile never changes MRNs, creates another asset store, or
executes code.

## Configuration

### YAML

```yaml
metamodel:
  profile: /etc/marmot/metamodel.yaml
```

### Environment Variables

```
MARMOT_METAMODEL_PROFILE=/etc/marmot/metamodel.yaml
```

## Options

| Option | Description | Default | Environment Variable |
| --- | --- | --- | --- |
| `metamodel.profile` | Path to the YAML profile file | - (native schema only) | `MARMOT_METAMODEL_PROFILE` |

The server loads the file once at startup and refuses invalid
configuration; without a profile, existing native writes keep their usual
validation. Not yet available via the Helm chart: mounting a profile file
would need a volume the chart doesn't declare yet.

## The profile format

```yaml
formatVersion: 1
id: example
version: 1
defaultLocale: en
fields:
  - id: retention
    type: integer
    core: true
    required: true
    appliesTo: governed_assets
    storage: metadata.example.retention
    validation:
      minimum: 1
    presentation:
      labelKey: example.retention.label
```

`core` marks membership in the governed contract; `required` determines whether
a value is mandatory. Optional core fields are supported. This initial format
only declares core fields. `governed_assets` means non-stub assets; omit it to
apply a field to stubs too. Stub creation is an internal ingestion operation,
not a selectable exemption on the asset HTTP API.

Supported types are `string`, `integer`, `number`, `boolean`, `date` (YYYY-MM-DD),
`enum`, and `list` with a scalar `itemType`. Enums declare `values`. Constraints
include `minimum`/`maximum`, `minLength`/`maxLength`, and `minItems`/`maxItems`.
For lists, numeric/string constraints apply to each item. Configuration is
limited to 1 MiB and 256 declared fields; duplicate keys, IDs, overlapping
bindings, and unsupported types are rejected.

Native bindings are `marmot.name`, `marmot.description`,
`marmot.user_description`, and `marmot.tags`. A native override must keep its
ID, binding, type, and structural requirements. Additional fields use
`metadata.<namespace>.<field>`. Ownership stays on the native relationship API;
ownership constraints, arbitrary references, business areas, and scoped roles
are not part of this profile format.

Other native property IDs are reserved: a profile cannot introduce a second
`mrn`, `owners`, or `version` inside metadata. Discovery plugins' `AssetSchemas`
continue describing source fields; they do not activate a governance profile.

A field's `presentation.control` can request an alternate editor for its
value without changing its stored type — for example `control: user` on a
`string` field asks the UI for an inline user search instead of a text box;
the field still validates and stores as a plain string.

Presentation keys are identifiers, not translated strings. Native labels use
existing Marmot message keys. Applications supplying custom profiles also
supply their translations. This backend contribution exposes the keys but does
not yet render profile-driven forms or check custom translation catalogues.

## Read the effective schema

`GET /api/v1/metamodel` requires `assets/view` and returns the composed fields,
profile version, `enabled`, and a deterministic schema hash. Clients consume
this response instead of parsing the YAML themselves. The schema describes
editable fields, not every property or relationship in an asset.

## Write through the asset API

Create assets through `POST /api/v1/assets/` with native properties and metadata:

```json
{
  "name": "example_table",
  "type": "Table",
  "providers": ["PostgreSQL"],
  "metadata": {"example": {"retention": 30}}
}
```

Read the asset and use its quoted `ETag` in `If-Match`:

```http
PATCH /api/v1/assets/ASSET_ID
Content-Type: application/json
If-Match: "1"

{"fields":{"retention":90}}
```

PATCH requires `assets/manage`. Its body is an explicit field-ID map, not JSON
Merge Patch. An absent field stays unchanged. Lists are replaced. `null`
removes only an optional nullable field. Unknown fields are rejected. Unrelated
metadata and the MRN are preserved. PATCH returns the native asset and its new
quoted ETag. Native change notifications include the affected fields.

| Response | Meaning |
| --- | --- |
| 400 | Invalid body, field value, or malformed precondition |
| 404 | Asset does not exist |
| 412 | The asset changed since the supplied version |
| 428 | PATCH has no `If-Match`, or PUT changes configured metadata without it |

This contract accepts a single strong quoted numeric version, not wildcard,
weak, or multiple entity tags. Validation errors contain
`{"fields":[{"field":"retention","code":"required"}]}`.

PUT keeps its existing metadata replacement semantics, except that configured
metadata fields omitted from the replacement are preserved. Changing one
requires `If-Match`. PUT is not a general deep merge: other omitted metadata
may be removed. Ingestion and OpenLineage use the same asset service and are
subject to these rules. Configure producers to supply required values before
enabling a profile; changing a protected value requires a version-aware writer.

Validation covers the final state in Create, Update, PATCH, AddTag, and
RemoveTag. Unrelated ownership/term mutations continue using their native APIs;
the profile cannot declare constraints on those relationships.

## Compatibility and rollout

Migration `000054_asset_version.sql` adds only `assets.version`, initialized to
1. Updates compare the version in SQL and increment it atomically. Concurrent
writers must re-read on conflict; the service does not silently retry a stale
document. Version checks cover asset row writes, not independent relationship
tables. The resource version is separate from the profile version and hash.

Before enabling a required field, backfill existing assets and update all
producers. Existing noncompliant assets remain readable, but subsequent row
mutations fail validation. Disabling a profile disables its validation and
preservation; profiles do not provide access isolation.

This is an upstream backend proposal. Profile-driven UI and SDK convenience
methods are follow-up work; HTTP and the generated OpenAPI describe the initial
contract. No business-area migration or authorization change is included.
