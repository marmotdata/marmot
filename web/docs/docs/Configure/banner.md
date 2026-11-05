---
title: Banner
description: Show a message across the top of Marmot, such as a maintenance notice.
---

# Banner

Show a message across the top of every page, such as a maintenance notice or an announcement.

## Show a banner

Add a banner to `config.yaml`:

```yaml
ui:
  banner:
    enabled: true
    variant: warning
    message: Maintenance on 25 January from 02:00 to 04:00 GMT. Search may be slow.
    id: maintenance-jan-25
```

Or with environment variables:

```bash
MARMOT_UI_BANNER_ENABLED=true
MARMOT_UI_BANNER_VARIANT=warning
MARMOT_UI_BANNER_MESSAGE="Maintenance on 25 January from 02:00 to 04:00 GMT. Search may be slow."
MARMOT_UI_BANNER_ID=maintenance-jan-25
```

## Dismissing it

People can close a dismissible banner, and their browser remembers that by the banner's `id`. To show a new message to everyone again, including people who closed the last one, give it a new `id`. Set `dismissible: false` for a message that has to stay up, such as an incident in progress.

## Options

| Key                     | Description                                         | Default    |
| ----------------------- | --------------------------------------------------- | ---------- |
| `ui.banner.enabled`     | Show the banner                                     | `false`    |
| `ui.banner.dismissible` | Let people close it                                 | `true`     |
| `ui.banner.variant`     | `info`, `warning`, `error` or `success`             | `info`     |
| `ui.banner.message`     | The text to show                                    | None       |
| `ui.banner.id`          | Identifies the message, so a closed banner stays closed | `banner-1` |

## Next steps

- [Configuration reference](index.md): every other setting.
