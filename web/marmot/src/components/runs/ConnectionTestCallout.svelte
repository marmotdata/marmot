<script lang="ts">
	import { m } from '$lib/paraglide/messages';
	import { formatNumber } from '$lib/utils/format';
	import type { ConnectionTestResult } from './pipeline';

	// What the last connection test found: what the plugin can see, why it could not connect, or that it ran out of time
	let { result, error = '' }: { result: ConnectionTestResult | null; error?: string } = $props();
</script>

{#if error}
	<mm-callout tone="danger" compact heading={error}></mm-callout>
{:else if result?.succeeded}
	<mm-callout tone="success" heading={m.pipelines_test_ok()}>
		<mm-description-list layout="inline-grid" columns={2}>
			<mm-description term={m.pipelines_tables_visible()}
				>{formatNumber(result.tables)}</mm-description
			>
			<mm-description term={m.pipelines_assets_visible()}
				>{formatNumber(result.assets)}</mm-description
			>
		</mm-description-list>
	</mm-callout>
{:else if result?.timed_out}
	<mm-callout tone="warning" heading={m.pipelines_test_timed_out()}>
		{m.pipelines_test_timed_out_body()}
	</mm-callout>
{:else if result}
	<mm-callout tone="danger" heading={m.pipelines_test_failed()}>
		{#if result.error}
			<mm-text variant="mono-dense">{result.error}</mm-text>
		{/if}
	</mm-callout>
{/if}
