---
title: Anonymous access
---

# Anonymous access

Anonymous access lets people browse the catalog without signing in. Requests without credentials get the permissions of one role, and anyone who signs in still gets their own.

Turn it on in your configuration:

```yaml
auth:
  anonymous:
    enabled: true
    role: user
```

Or with environment variables:

```bash
MARMOT_AUTH_ANONYMOUS_ENABLED=true
MARMOT_AUTH_ANONYMOUS_ROLE=user
```

| Key | Description | Default |
| --- | --- | --- |
| `auth.anonymous.enabled` | Serve requests that carry no credentials | `false` |
| `auth.anonymous.role` | The role those requests get | `user` |

The `user` role reads assets, the glossary, ingestion runs, teams and metrics. To show less, [create a role](access-control.md#roles) with only the permissions you want public, such as `assets:view` and `glossary:view`, and name it in `role`.

## Next steps

- [Users, roles and access](access-control.md): what each permission allows.
