# Translations

The web UI reads its text from a message catalog, so a new language is a JSON file rather than a fork of the UI. English (`web/marmot/messages/en.json`) is the source and the fallback, so a partial translation is still usable.

## Adding a language

1. Copy `messages/en.json` to `messages/<tag>.json` (`es.json`, `pt-BR.json`).
2. Translate the values, leaving the keys untouched.
3. Add the tag to `locales` in `web/marmot/project.inlang/settings.json`.
4. Run `pnpm run i18n:compile && pnpm run dev` in `web/marmot` to see it.

Keep every placeholder spelled exactly as in English, such as `{name}`.

Counted messages use one variant per plural category. English needs `one` and `other`, other languages may add `zero`, `two`, `few` or `many`:

```json
{
  "assets_match_count": [
    {
      "declarations": ["input count", "local countPlural = count: plural"],
      "selectors": ["countPlural"],
      "match": {
        "countPlural=one": "{count} matched asset",
        "countPlural=other": "{count} matched assets"
      }
    }
  ]
}
```

## Adding UI strings

Add a message rather than a literal:

```svelte
<script lang="ts">
	import { m } from '$lib/paraglide/messages';
</script>

<h2>{m.assets_heading()}</h2>
```

Keys are lowercase and namespaced by area (`nav_`, `login_`, `assets_`, `teams_`). Keep a whole sentence in one message and pass values as placeholders, never join translated fragments, because word order differs between languages. The `marmot/no-untranslated-strings` lint rule reports hard-coded text in templates.
