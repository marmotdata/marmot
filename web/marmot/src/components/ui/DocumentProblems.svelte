<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import type { EditorDiagnostic } from '$lib/document/types';

	// The problems and warnings in a document as a list beside its editor; choosing one moves the editor's cursor to it.
	let {
		diagnostics,
		checking = false,
		onSelect
	}: {
		diagnostics: EditorDiagnostic[];
		checking?: boolean;
		onSelect?: (diagnostic: EditorDiagnostic) => void;
	} = $props();

	let errors = $derived(diagnostics.filter((d) => d.severity === 'error').length);
	let warnings = $derived(diagnostics.length - errors);
</script>

<div class="rounded-lg border border-gray-200 dark:border-gray-700" aria-live="polite">
	<div
		class="flex items-center justify-between gap-3 border-b border-gray-100 px-3 py-2 dark:border-gray-700/60"
	>
		<h4 class="text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
			{m.ui_document_problems_heading()}
		</h4>
		<span class="flex items-center gap-3 text-xs tabular-nums text-gray-500 dark:text-gray-400">
			{#if checking}
				<span class="flex items-center gap-1">
					<IconifyIcon icon="material-symbols:progress-activity" class="h-3.5 w-3.5 animate-spin" />
					{m.ui_document_checking()}
				</span>
			{/if}
			{#if errors > 0}
				<span class="flex items-center gap-1 text-red-600 dark:text-red-400">
					<IconifyIcon icon="material-symbols:error-outline" class="h-3.5 w-3.5" />
					{m.ui_document_error_count({ count: errors })}
				</span>
			{/if}
			{#if warnings > 0}
				<span class="flex items-center gap-1 text-amber-700 dark:text-amber-400">
					<IconifyIcon icon="material-symbols:warning-outline" class="h-3.5 w-3.5" />
					{m.ui_document_warning_count({ count: warnings })}
				</span>
			{/if}
		</span>
	</div>
	{#if diagnostics.length === 0}
		<p class="flex items-center gap-1.5 px-3 py-2.5 text-xs text-gray-500 dark:text-gray-400">
			<IconifyIcon
				icon="material-symbols:check-circle-outline"
				class="h-3.5 w-3.5 text-earthy-green-700 dark:text-earthy-green-400"
			/>
			{m.ui_document_no_problems()}
		</p>
	{:else}
		<ul class="max-h-48 divide-y divide-gray-100 overflow-y-auto dark:divide-gray-700/60">
			{#each diagnostics as diagnostic, index (index)}
				<li>
					<button
						type="button"
						class="flex w-full items-start gap-2 px-3 py-2 text-left text-xs transition-colors hover:bg-gray-50 focus:outline-none focus-visible:bg-gray-50 dark:hover:bg-gray-700/40 dark:focus-visible:bg-gray-700/40"
						onclick={() => onSelect?.(diagnostic)}
					>
						<IconifyIcon
							icon={diagnostic.severity === 'error'
								? 'material-symbols:error-outline'
								: 'material-symbols:warning-outline'}
							class="mt-0.5 h-3.5 w-3.5 shrink-0 {diagnostic.severity === 'error'
								? 'text-red-600 dark:text-red-400'
								: 'text-amber-600 dark:text-amber-400'}"
						/>
						<span class="min-w-0 flex-1 text-gray-700 dark:text-gray-300">
							{diagnostic.message}
							{#if diagnostic.path}
								<code class="ml-1 font-mono text-[11px] text-gray-500 dark:text-gray-400"
									>{diagnostic.path}</code
								>
							{/if}
						</span>
						{#if diagnostic.line}
							<span class="shrink-0 tabular-nums text-gray-400 dark:text-gray-500">
								{m.ui_document_line({ line: diagnostic.line })}
							</span>
						{/if}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>
