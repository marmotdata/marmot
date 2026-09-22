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

### Profile keys

| Key | Required | Description |
| --- | --- | --- |
| `formatVersion` | yes | Must be `1` |
| `id` | yes | Profile identifier (`a-z`, digits, `_`, `-`) |
| `version` | yes | Profile revision (≥ 1). Independent of `assets.version` |
| `defaultLocale` | yes | Default locale for presentation message keys |
| `fields` | yes | Field definitions (max `256`; file max `1 MiB`) |
| `messages` | no | Message catalogue resolving presentation keys to text, by locale (see below) |

Duplicate field IDs, overlapping storage bindings, and unsupported types are
rejected at startup.

### Field keys

| Key | Required | Description |
| --- | --- | --- |
| `id` | yes | Field identifier |
| `type` | yes | Value type (see below) |
| `storage` | yes | Binding: native `marmot.*` or `metadata.<namespace>.<field>` |
| `core` | yes | Membership in the governed contract. This format only declares core fields |
| `required` | yes | Whether a value is mandatory. Optional core fields are allowed |
| `nullable` | no | Allows explicit `null` on PATCH for optional fields |
| `appliesTo` | no | `governed_assets` = non-stub assets only. Omit to include stubs too |
| `itemType` | for `list` | Scalar item type |
| `values` | for `enum` | Allowed enum members |
| `validation` | no | Constraints object (see below) |
| `presentation` | no | UI/message hints (see below) |

Stub creation is an internal ingestion operation, not an HTTP exemption.

### Types

| `type` | Notes |
| --- | --- |
| `string` | `"text"` |
| `integer` | `12` |
| `number` | `12.34` |
| `boolean` | `true` |
| `date` | `YYYY-MM-DD`, example: `2026-01-01` |
| `enum` | Requires `values`, example: `["red", "green", "blue"]` |
| `list` | Requires scalar `itemType`, example: `["integer"]`; list constraints apply to the list, item constraints to each item |

### Validation keys

| Key | Applies to |
| --- | --- |
| `minimum` / `maximum` | `integer`, `number`, and list items of those types |
| `minLength` / `maxLength` | `string`, and list items of type `string` |
| `minItems` / `maxItems` | `list` |

### Presentation keys

| Key | Description |
| --- | --- |
| `labelKey` | Message key for the field label (identifier, not a translated string) |
| `helpTextKey` | Optional help-text message key |
| `descriptionKey` | Optional description message key |
| `section` | Optional UI section id |
| `order` | Optional sort order within the section |
| `control` | Alternate editor without changing storage. Only `user` is defined so far (string holding a Marmot user ID) |

Native labels reuse existing Marmot message keys. A profile with custom fields
should also ship a `messages` catalogue (below) so clients can resolve their
keys without a Marmot rebuild; this backend contribution exposes the keys and
serves the catalogue, it does not yet render profile-driven forms itself.

### Message catalogues

```yaml
messages:
  en:
    example.retention.label: Retention (days)
  es:
    example.retention.label: Retención (días)
```

`messages` maps locale to a flat key → text catalogue. `GET /api/v1/metamodel`
returns it verbatim as `messages`. A client resolves a presentation key against
the current locale's catalogue, then `defaultLocale`'s, then its own native
messages, then falls back to the raw key — the server does no translation or
fallback itself. Locales and keys are validated as identifiers; values must be
non-empty. Keeping labels in this catalogue, not in Marmot's own message
files, means editing a profile's text never requires a kernel change or
rebuild.

### Native storage bindings

| Binding | Field id |
| --- | --- |
| `marmot.name` | `name` |
| `marmot.description` | `description` |
| `marmot.user_description` | `user_description` |
| `marmot.tags` | `tags` |

A native override must keep its id, binding, type, and structural requirements.
Additional fields use `metadata.<namespace>.<field>`. Other native property ids
(`mrn`, `owners`, `version`, …) are reserved: a profile cannot reintroduce them
inside metadata. Discovery plugins' `AssetSchemas` still describe source fields;
they do not activate a governance profile.

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
