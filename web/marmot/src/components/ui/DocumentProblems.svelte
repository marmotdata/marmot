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

<div class="rounded-md border border-mm-rule" aria-live="polite">
	<div class="flex items-center justify-between gap-3 border-b border-mm-rule px-3 py-2">
		<h4 class="text-label text-mm-text-2">
			{m.ui_document_problems_heading()}
		</h4>
		<span class="flex items-center gap-3 text-caption tabular-nums text-mm-text-2">
			{#if checking}
				<span class="flex items-center gap-1">
					<IconifyIcon icon="material-symbols:progress-activity" class="h-3.5 w-3.5 animate-spin" />
					{m.ui_document_checking()}
				</span>
			{/if}
			{#if errors > 0}
				<span class="flex items-center gap-1 text-mm-danger">
					<IconifyIcon icon="material-symbols:error-outline" class="h-3.5 w-3.5" />
					{m.ui_document_error_count({ count: errors })}
				</span>
			{/if}
			{#if warnings > 0}
				<span class="flex items-center gap-1 text-mm-warning-text">
					<IconifyIcon icon="material-symbols:warning-outline" class="h-3.5 w-3.5" />
					{m.ui_document_warning_count({ count: warnings })}
				</span>
			{/if}
		</span>
	</div>
	{#if diagnostics.length === 0}
		<p class="flex items-center gap-1.5 px-3 py-2.5 text-caption text-mm-text-2">
			<IconifyIcon
				icon="material-symbols:check-circle-outline"
				class="h-3.5 w-3.5 text-mm-success"
			/>
			{m.ui_document_no_problems()}
		</p>
	{:else}
		<ul class="max-h-48 divide-y divide-mm-rule overflow-y-auto">
			{#each diagnostics as diagnostic, index (index)}
				<li>
					<button
						type="button"
						class="flex w-full items-start gap-2 px-3 py-2 text-left text-caption transition-colors hover:bg-mm-sunken focus-visible:bg-mm-sunken"
						onclick={() => onSelect?.(diagnostic)}
					>
						<IconifyIcon
							icon={diagnostic.severity === 'error'
								? 'material-symbols:error-outline'
								: 'material-symbols:warning-outline'}
							class="mt-0.5 h-3.5 w-3.5 shrink-0 {diagnostic.severity === 'error'
								? 'text-mm-danger'
								: 'text-mm-warning'}"
						/>
						<span class="min-w-0 flex-1 text-mm-text">
							{diagnostic.message}
							{#if diagnostic.path}
								<code class="ml-1 font-mono text-mono-dense text-mm-text-2">{diagnostic.path}</code>
							{/if}
						</span>
						{#if diagnostic.line}
							<span class="shrink-0 tabular-nums text-mm-text-3">
								{m.ui_document_line({ line: diagnostic.line })}
							</span>
						{/if}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>
