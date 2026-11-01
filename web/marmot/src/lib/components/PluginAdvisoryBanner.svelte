<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import type { AdvisoryBannerEntry } from '$lib/plugins/advisoryBanner.svelte';

	// Purely presentational: the root layout decides whether this is the one banner to show (there
	// is only one banner slot) and which entry is the most severe across every plugin.
	let { entry, onDismiss }: { entry: AdvisoryBannerEntry; onDismiss: () => void } = $props();
</script>

<mm-banner
	tone="danger"
	dismissible
	href={resolve('/settings/plugins')}
	link-text={m.plugin_versions_banner_link()}
	onmm-dismiss={onDismiss}
>
	{m.plugin_versions_banner_message({
		name: entry.name,
		version: entry.version,
		severity: entry.advisory.severity,
		id: entry.advisory.id,
		fixed: entry.advisory.fixed
	})}
</mm-banner>
