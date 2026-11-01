# Customisable Banner

Marmot allows you to display a customisable banner at the top of the application to communicate important information to users, such as maintenance notices, announcements, or warnings.

## Configuration

The banner can be configured using either YAML configuration or environment variables.

### YAML

Add the following to your `config.yaml` file:

```yaml
ui:
  banner:
    enabled: true
    dismissible: true
    variant: info
    message: Welcome to Marmot!
    id: welcome-banner
```

### Environment Variables

Set these environment variables:

```
MARMOT_UI_BANNER_ENABLED=true
MARMOT_UI_BANNER_DISMISSIBLE=true
MARMOT_UI_BANNER_VARIANT=info
MARMOT_UI_BANNER_MESSAGE=Welcome to Marmot!
MARMOT_UI_BANNER_ID=welcome-banner
```

## Options

| Option        | Description                                                          | Default | Values                                  |
| ------------- | -------------------------------------------------------------------- | ------- | --------------------------------------- |
| `enabled`     | Whether the banner is displayed                                      | `false` | `true`, `false`                         |
| `dismissible` | Whether users can dismiss the banner                                 | `false` | `true`, `false`                         |
| `variant`     | Visual style of the banner                                           | `info`  | `info`, `warning`, `error`, `success`   |
| `message`     | Message to display in the banner                                     | -       | Any string                              |
| `id`          | Unique identifier for the banner (used for tracking dismissal state) | -       | Any string (e.g., `maintenance-jan-25`) |

## Variants

The banner supports four visual variants:

- **info**: Blue banner for general information and announcements
- **warning**: Orange banner for warnings and important notices
- **error**: Red banner for critical alerts and errors
- **success**: Green banner for positive messages and confirmations

## Dismissible Banners

When `dismissible` is set to `true`, users can close the banner. The dismissal state is stored locally using the banner's `id`. If you update the `id`, the banner will reappear for all users, even if they previously dismissed it.

This is useful for new announcements where you want to ensure all users see the updated message.

## Examples

### Maintenance Notice

```yaml
ui:
  banner:
    enabled: true
    dismissible: true
    variant: warning
    message: Scheduled maintenance on 25th January, 2025 from 02:00-04:00 GMT
    id: maintenance-jan-25
```

### Critical Alert

```yaml
ui:
  banner:
    enabled: true
    dismissible: false
    variant: error
    message: Production deployment in progress. Data may be temporarily unavailable.
    id: prod-deployment-jan-25
```

### General Announcement

```yaml
ui:
  banner:
    enabled: true
    dismissible: true
    variant: info
    message: New features released! Check out the updated glossary and metrics pages.
    id: release-v2-0
```
