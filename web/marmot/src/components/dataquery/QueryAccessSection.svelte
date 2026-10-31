<script lang="ts">
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';

	interface Props {
		queryEnabled: boolean;
		ingestOnQuery: boolean;
	}

	let { queryEnabled = $bindable(false), ingestOnQuery = $bindable(false) }: Props = $props();

	const canManage = auth.hasPermission('access', 'manage');
</script>

<!-- Querying is what people connect sources for, so it leads the pipeline's setup -->
<mm-panel
	id="query-access"
	heading={m.pipelines_query_toggle_label()}
	icon="terminal"
	padding="default"
>
	<div class="mm-stack" style="--mm-gap: var(--space-4)">
		<mm-switch
			label={m.pipelines_query_switch_label()}
			description={canManage ? m.pipelines_query_toggle_hint() : m.pipelines_query_no_permission()}
			checked={queryEnabled}
			disabled={!canManage}
			onmm-change={(e: CustomEvent<{ checked: boolean }>) => {
				queryEnabled = e.detail.checked;
				if (queryEnabled) ingestOnQuery = true;
			}}
		></mm-switch>
		{#if queryEnabled}
			<mm-switch
				label={m.pipelines_ingest_on_query_label()}
				description={m.pipelines_ingest_on_query_hint()}
				checked={ingestOnQuery}
				disabled={!canManage}
				onmm-change={(e: CustomEvent<{ checked: boolean }>) => (ingestOnQuery = e.detail.checked)}
			></mm-switch>
		{/if}
	</div>
</mm-panel>
