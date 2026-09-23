<script lang="ts">
	import { m } from '$lib/paraglide/messages';

	let {
		page,
		pageSize,
		total,
		disabled = false,
		summary = m.common_pagination_showing,
		onChange
	}: {
		page: number;
		pageSize: number;
		total: number;
		disabled?: boolean;
		summary?: (range: { from: number; to: number; total: number }) => string;
		onChange: (page: number) => void;
	} = $props();

	let pages = $derived(Math.max(1, Math.ceil(total / pageSize)));
</script>

{#if pages > 1}
	<div class="flex justify-between items-center pt-4 border-t border-gray-200 dark:border-gray-700">
		<p class="text-sm text-gray-600 dark:text-gray-400">
			{summary({ from: (page - 1) * pageSize + 1, to: Math.min(page * pageSize, total), total })}
		</p>
		<div class="flex gap-2">
			<button
				onclick={() => onChange(page - 1)}
				disabled={page <= 1 || disabled}
				class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
			>
				{m.common_previous()}
			</button>
			<button
				onclick={() => onChange(page + 1)}
				disabled={page >= pages || disabled}
				class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
			>
				{m.common_next()}
			</button>
		</div>
	</div>
{/if}
