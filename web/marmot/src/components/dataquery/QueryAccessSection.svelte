<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';

	interface Props {
		queryEnabled: boolean;
		ingestOnQuery: boolean;
		hasSchedule: boolean;
	}

	let {
		queryEnabled = $bindable(false),
		ingestOnQuery = $bindable(false),
		hasSchedule
	}: Props = $props();

	const canManage = auth.hasPermission('access', 'manage');

	function toggle() {
		if (!canManage) return;
		queryEnabled = !queryEnabled;
		if (queryEnabled) ingestOnQuery = true;
	}
</script>

<!-- Querying is a headline capability, so the card is tinted while it is on and quiet while it is off -->
<div
	id="query-access"
	class="scroll-mt-24 rounded-xl border p-5 transition-colors {queryEnabled
		? 'border-earthy-terracotta-200 bg-earthy-terracotta-50/60 dark:border-earthy-terracotta-800 dark:bg-earthy-terracotta-900/10'
		: 'border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800'}"
>
	<div class="flex items-center gap-4">
		<span
			class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg {queryEnabled
				? 'bg-earthy-terracotta-600 text-white'
				: 'bg-gray-100 text-gray-400 dark:bg-gray-700'}"
		>
			<IconifyIcon icon="material-symbols:bolt" class="h-6 w-6" />
		</span>
		<div class="min-w-0 flex-1">
			<span
				id="query-toggle-label"
				class="block text-base font-semibold text-gray-900 dark:text-gray-100"
				>{m.pipelines_query_toggle_label()}</span
			>
			<p class="mt-0.5 text-sm text-gray-600 dark:text-gray-400">
				{m.pipelines_query_toggle_hint()}
			</p>
		</div>
		<button
			type="button"
			role="switch"
			aria-checked={queryEnabled}
			aria-labelledby="query-toggle-label"
			disabled={!canManage}
			onclick={toggle}
			class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors disabled:cursor-not-allowed disabled:opacity-50 {queryEnabled
				? 'bg-earthy-terracotta-600'
				: 'bg-gray-300 dark:bg-gray-600'}"
		>
			<span
				class="inline-block h-5 w-5 transform rounded-full bg-white shadow transition-transform {queryEnabled
					? 'translate-x-5'
					: 'translate-x-0.5'}"
			></span>
		</button>
	</div>
	{#if !canManage}
		<p class="mt-3 pl-14 text-xs text-gray-500 dark:text-gray-400">
			{m.pipelines_query_no_permission()}
		</p>
	{/if}

	{#if queryEnabled}
		<div class="mt-4 space-y-2 pl-14">
			<label class="flex items-center gap-2.5 {canManage ? 'cursor-pointer' : 'opacity-60'}">
				<input
					type="checkbox"
					bind:checked={ingestOnQuery}
					disabled={!canManage}
					class="rounded text-earthy-terracotta-600 focus:ring-earthy-terracotta-500"
				/>
				<span class="text-sm text-gray-800 dark:text-gray-200"
					>{m.pipelines_ingest_on_query_label()}</span
				>
				<span class="text-xs text-gray-500 dark:text-gray-400"
					>{m.pipelines_ingest_on_query_hint()}</span
				>
			</label>
			{#if !hasSchedule}
				<p class="text-xs text-gray-500 dark:text-gray-400">{m.pipelines_query_only_hint()}</p>
			{/if}
		</div>
	{/if}
</div>
