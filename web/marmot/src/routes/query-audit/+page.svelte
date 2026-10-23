<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import Panel from '$components/ui/Panel.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import SegmentedControl from '$components/ui/SegmentedControl.svelte';
	import QueryLog from '$components/dataquery/QueryLog.svelte';
	import { m } from '$lib/paraglide/messages';
	import { myAccess, loadMyAccess, holdsAtRoot } from '$lib/iam/myAccess';

	// The record of every data query, for whoever answers for how data is used: stewards and auditors as much as engineers, so it lives with governance rather than system settings.
	type Range = '24h' | '7d' | '30d' | '90d';
	const HOURS: Record<Range, number> = { '24h': 24, '7d': 7 * 24, '30d': 30 * 24, '90d': 90 * 24 };

	let range = $state<Range>('7d');

	// Rounded to the minute so a re-render does not ask for a new window
	let from = $derived(
		new Date(Math.floor((Date.now() - HOURS[range] * 3_600_000) / 60_000) * 60_000)
	);

	const rangeOptions: { id: Range; label: string }[] = [
		{ id: '24h', label: m.dataquery_audit_range_24h() },
		{ id: '7d', label: m.dataquery_audit_range_7d() },
		{ id: '30d', label: m.dataquery_audit_range_30d() },
		{ id: '90d', label: m.dataquery_audit_range_90d() }
	];

	onMount(() => {
		loadMyAccess();
	});

	let canView = $derived(holdsAtRoot($myAccess, 'access:view'));
</script>

<svelte:head><title>{m.queryaudit_page_title()}</title></svelte:head>

<div class="w-full px-4 py-6 sm:px-6 lg:px-8">
	<PageHeader title={m.queryaudit_heading()} description={m.queryaudit_description()}>
		{#snippet actions()}
			{#if canView}
				<SegmentedControl
					options={rangeOptions}
					value={range}
					onChange={(id) => (range = id)}
					size="sm"
					label={m.dataquery_audit_range_label()}
				/>
			{/if}
		{/snippet}
	</PageHeader>

	{#if canView}
		<Panel flush>
			<QueryLog flush heading={false} from={from.toISOString()} />
		</Panel>
	{:else if $myAccess.checked}
		<EmptyState
			icon="material-symbols:lock-outline"
			title={m.queryaudit_no_access_title()}
			description={m.queryaudit_no_access_description()}
		/>
	{/if}
</div>
