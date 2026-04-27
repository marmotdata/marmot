# Anonymous Authentication

Marmot allows you to enable anonymous authentication, which provides restricted access to the application without requiring users to log in.

When anonymous authentication is enabled, users can still log in with other authentication methods to access their full permissions. Anonymous users will only have access to endpoints that match the permissions of their assigned role.

## Configuration

Anonymous authentication can be enabled using either YAML configuration or environment variables.

### YAML

Add the following to your `config.yaml` file:

```yaml
auth:
  anonymous:
    enabled: true
```

### Environment Variables

Set these environment variables:

```
MARMOT_AUTH_ANONYMOUS_ENABLED=true
```

## Options

| Option    | Description                                 | Default |
| --------- | ------------------------------------------- | ------- |
| `enabled` | Whether anonymous authentication is enabled | `false` |
| `role`    | The role to assign to anonymous users       | `user`  |
