<script lang="ts">
	import { resolve } from '$app/paths';
	import Icon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import DomainSelect from '$components/domain/DomainSelect.svelte';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { locale } from '$lib/i18n';
	import { violationMessage } from '$lib/metamodel/i18n';
	import {
		downloadSheet,
		importColumns,
		importTerms,
		type ImportColumn,
		type ImportAction,
		type ImportProblem,
		type ImportResult,
		type OnExisting,
		type SheetFormat
	} from '$lib/glossary/import';

	let columns = $state<ImportColumn[]>([]);
	let file = $state<File | null>(null);
	let onExisting = $state<OnExisting>('skip');
	// Where new rows without a domain column land; empty is Unassigned.
	let defaultDomain = $state('');
	let result = $state<ImportResult | null>(null);
	// The result belongs to this file and option; changing either asks for a new validation.
	let validatedFor = $state<{ file: File; onExisting: OnExisting; defaultDomain: string } | null>(
		null
	);
	let busy = $state<'validate' | 'apply' | 'download' | null>(null);
	let filter = $state<ImportAction | 'all'>('all');
	let dragging = $state(false);

	// Reloads when the language changes: profile labels come localized from the server.
	$effect(() => {
		const current = $locale;
		importColumns(current)
			.then((cols) => (columns = cols))
			.catch(() => (columns = []));
	});

	const nativeLabels: Record<string, () => string> = {
		name: m.glossary_import_col_name,
		definition: m.glossary_import_col_definition,
		description: m.glossary_import_col_description,
		parent: m.glossary_import_col_parent,
		owners: m.glossary_import_col_owners,
		tags: m.glossary_import_col_tags,
		domain: m.glossary_import_col_domain
	};

	function columnLabel(c: ImportColumn): string {
		return !c.profile && nativeLabels[c.id] ? nativeLabels[c.id]() : c.label;
	}

	const kindText: Record<string, () => string> = {
		string: m.glossary_import_format_string,
		integer: m.glossary_import_format_integer,
		number: m.glossary_import_format_number,
		boolean: m.glossary_import_format_boolean,
		date: m.glossary_import_format_date,
		enum: m.glossary_import_format_enum
	};

	/** How to write a cell, from the column's type, constraints and separator. */
	function columnFormat(c: ImportColumn): string {
		const sep = c.separator ?? '|';
		switch (c.format) {
			case 'name':
				return m.glossary_import_format_name();
			case 'text':
				return m.glossary_import_format_string();
			case 'term':
				return m.glossary_import_format_term();
			case 'owners':
				return m.glossary_import_format_owners({ sep });
			case 'tags':
				return m.glossary_import_format_tags({ sep });
			case 'domain':
				return m.glossary_import_format_domain();
		}
		const v = c.validation ?? {};
		const kind = c.type === 'list' ? (c.item_type ?? 'string') : c.type;
		const parts = [kindText[kind]?.() ?? kind];
		if (v.minimum !== undefined && v.maximum !== undefined) {
			parts.push(m.glossary_import_format_range({ min: v.minimum, max: v.maximum }));
		} else if (v.minimum !== undefined) {
			parts.push(m.glossary_import_format_min({ min: v.minimum }));
		} else if (v.maximum !== undefined) {
			parts.push(m.glossary_import_format_max({ max: v.maximum }));
		}
		if (v.minLength !== undefined)
			parts.push(m.glossary_import_format_min_length({ n: v.minLength }));
		if (v.maxLength !== undefined)
			parts.push(m.glossary_import_format_max_length({ n: v.maxLength }));
		const item = parts.join(', ');
		if (c.type !== 'list') return item;
		const list = [m.glossary_import_format_list({ sep, item })];
		if (v.minItems !== undefined) list.push(m.glossary_import_format_min_items({ n: v.minItems }));
		if (v.maxItems !== undefined) list.push(m.glossary_import_format_max_items({ n: v.maxItems }));
		return list.join('; ');
	}

	const stale = $derived(
		!!result &&
			(validatedFor?.file !== file ||
				validatedFor?.onExisting !== onExisting ||
				validatedFor?.defaultDomain !== defaultDomain)
	);
	const canApply = $derived(
		!!result &&
			!stale &&
			!result.applied &&
			(result.problems ?? []).length === 0 &&
			result.summary.errors === 0 &&
			result.summary.create + result.summary.update > 0
	);
	const rows = $derived(
		(result?.rows ?? []).filter((r) => filter === 'all' || r.action === filter)
	);

	async function download(what: 'template' | 'export', format: SheetFormat) {
		busy = 'download';
		try {
			await downloadSheet(what, format, $locale);
		} catch (error) {
			toasts.error(error instanceof Error ? error.message : m.glossary_import_download_failed());
		} finally {
			busy = null;
		}
	}

	function choose(files: FileList | null | undefined) {
		const picked = files?.[0];
		if (!picked) return;
		file = picked;
		result = null;
		validatedFor = null;
	}

	async function run(mode: 'validate' | 'apply') {
		if (!file) return;
		busy = mode;
		try {
			const sent = { file, onExisting, defaultDomain };
			result = await importTerms(file, mode, onExisting, defaultDomain);
			validatedFor = sent;
			filter = result.summary.errors > 0 ? 'error' : 'all';
			if (mode === 'apply' && result.applied) {
				toasts.success(
					m.glossary_import_applied({
						created: result.summary.create,
						updated: result.summary.update
					})
				);
			}
		} catch (error) {
			toasts.error(error instanceof Error ? error.message : m.glossary_import_failed());
		} finally {
			busy = null;
		}
	}

	const codeMessages: Record<string, () => string> = {
		required: m.glossary_import_code_required,
		duplicate_in_file: m.glossary_import_code_duplicate_in_file,
		ambiguous_name: m.glossary_import_code_ambiguous_name,
		parent_not_found: m.glossary_import_code_parent_not_found,
		parent_self: m.glossary_import_code_parent_self,
		ambiguous_parent: m.glossary_import_code_ambiguous_parent,
		parent_cycle: m.glossary_import_code_parent_cycle,
		owner_not_found: m.glossary_import_code_owner_not_found,
		missing: m.glossary_import_code_missing,
		matched_ignoring_case: m.glossary_import_code_matched_ignoring_case,
		domain_not_found: m.glossary_import_code_domain_not_found,
		ambiguous_domain: m.glossary_import_code_ambiguous_domain,
		domain_forbidden: m.glossary_import_code_domain_forbidden,
		unknown_column: m.glossary_import_code_unknown_column,
		missing_column: m.glossary_import_code_missing_column,
		duplicate_column: m.glossary_import_code_duplicate_column
	};
	const profileCodes = ['type', 'range', 'length', 'items', 'enum', 'date', 'not_nullable'];

	function describe(p: ImportProblem): string {
		if (codeMessages[p.code]) return codeMessages[p.code]();
		if (profileCodes.includes(p.code)) return violationMessage(p.code);
		return p.message;
	}

	const actionLabel: Record<ImportAction, () => string> = {
		create: m.glossary_import_action_create,
		update: m.glossary_import_action_update,
		skip: m.glossary_import_action_skip,
		error: m.glossary_import_action_error
	};
	const actionClass: Record<ImportAction, string> = {
		create: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-200',
		update: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-200',
		skip: 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200',
		error: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-200'
	};
	const card =
		'rounded-lg border border-gray-200 bg-white p-5 dark:border-gray-700 dark:bg-gray-800';
	const head =
		'px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400';
	const cell = 'px-3 py-2 align-top text-sm text-gray-700 dark:text-gray-300';
</script>

<div class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
	<a
		href={resolve('/glossary/[[id]]', { id: undefined })}
		class="mb-4 inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
	>
		<Icon icon="material-symbols:arrow-back" class="h-4 w-4" />
		{m.glossary_heading()}
	</a>
	<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">{m.glossary_import_title()}</h1>
	<p class="mb-6 mt-1 text-sm text-gray-500 dark:text-gray-400">{m.glossary_import_subtitle()}</p>

	<section class="{card} mb-6" aria-labelledby="import-template">
		<h2 id="import-template" class="text-base font-semibold text-gray-900 dark:text-gray-100">
			1. {m.glossary_import_template_heading()}
		</h2>
		<p class="mb-4 mt-1 text-sm text-gray-500 dark:text-gray-400">
			{m.glossary_import_template_hint()}
		</p>
		<div class="mb-5 flex flex-wrap gap-2">
			<Button
				icon="material-symbols:table-view-outline"
				text={m.glossary_import_template_xlsx()}
				disabled={busy !== null}
				click={() => download('template', 'xlsx')}
			/>
			<Button
				variant="clear"
				icon="material-symbols:csv-outline"
				text={m.glossary_import_template_csv()}
				disabled={busy !== null}
				click={() => download('template', 'csv')}
			/>
			<Button
				variant="clear"
				icon="material-symbols:download"
				text={m.glossary_import_export()}
				disabled={busy !== null}
				click={() => download('export', 'xlsx')}
			/>
		</div>
		<details class="text-sm">
			<summary class="cursor-pointer font-medium text-gray-700 dark:text-gray-300">
				{m.glossary_import_guide()}
			</summary>
			<p class="mt-2 text-gray-500 dark:text-gray-400">{m.glossary_import_guide_hint()}</p>
			<div class="mt-3 overflow-x-auto rounded-md border border-gray-200 dark:border-gray-700">
				<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
					<thead class="bg-gray-50 dark:bg-gray-900/40">
						<tr>
							<th class={head}>{m.glossary_import_guide_column()}</th>
							<th class={head}>{m.glossary_import_guide_label()}</th>
							<th class={head}>{m.glossary_import_guide_required()}</th>
							<th class={head}>{m.glossary_import_guide_format()}</th>
							<th class={head}>{m.glossary_import_guide_values()}</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-100 dark:divide-gray-700">
						{#each columns as col (col.id)}
							<tr>
								<td class="{cell} font-mono text-xs">{col.id}</td>
								<td class={cell}>
									{columnLabel(col)}
									{#if col.profile && col.help}
										<span class="block text-xs text-gray-500 dark:text-gray-400">{col.help}</span>
									{/if}
								</td>
								<td class={cell}>{col.required ? m.glossary_import_yes() : ''}</td>
								<td class="{cell} text-xs">{columnFormat(col)}</td>
								<td class="{cell} font-mono text-xs">{(col.values ?? []).join(', ')}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</details>
	</section>

	<section class="{card} mb-6" aria-labelledby="import-file">
		<h2 id="import-file" class="text-base font-semibold text-gray-900 dark:text-gray-100">
			2. {m.glossary_import_file_heading()}
		</h2>
		<label
			class="mt-4 flex cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed px-4 py-8 text-center transition-colors {dragging
				? 'border-earthy-terracotta-600 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20'
				: 'border-gray-300 hover:border-gray-400 dark:border-gray-600'}"
			ondragover={(e) => {
				e.preventDefault();
				dragging = true;
			}}
			ondragleave={() => (dragging = false)}
			ondrop={(e) => {
				e.preventDefault();
				dragging = false;
				choose(e.dataTransfer?.files);
			}}
		>
			<Icon icon="material-symbols:upload-file-outline" class="h-8 w-8 text-gray-400" />
			<span class="text-sm text-gray-700 dark:text-gray-300">
				{file ? file.name : m.glossary_import_drop()}
			</span>
			<span class="text-xs text-gray-500 dark:text-gray-400">{m.glossary_import_limits()}</span>
			<input
				type="file"
				accept=".xlsx,.csv"
				class="sr-only"
				onchange={(e) => choose((e.currentTarget as HTMLInputElement).files)}
			/>
		</label>

		<fieldset class="mt-5">
			<legend class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
				{m.glossary_import_existing()}
			</legend>
			<div class="grid gap-2 sm:grid-cols-2">
				{#each [{ value: 'skip', label: m.glossary_import_existing_skip(), hint: m.glossary_import_existing_skip_hint() }, { value: 'update', label: m.glossary_import_existing_update(), hint: m.glossary_import_existing_update_hint() }] as option (option.value)}
					<label
						class="flex cursor-pointer flex-col rounded-md border px-3 py-2 text-sm {onExisting ===
						option.value
							? 'border-earthy-terracotta-600'
							: 'border-gray-200 hover:border-gray-300 dark:border-gray-700'}"
					>
						<span class="flex items-center gap-2">
							<input
								type="radio"
								name="on-existing"
								value={option.value}
								bind:group={onExisting}
								class="text-earthy-terracotta-700 focus:ring-earthy-terracotta-600"
							/>
							<span class="font-medium text-gray-900 dark:text-gray-100">{option.label}</span>
						</span>
						<span class="mt-1 text-xs text-gray-500 dark:text-gray-400">{option.hint}</span>
					</label>
				{/each}
			</div>
		</fieldset>

		<div class="mt-5 max-w-md">
			<DomainSelect id="import-default-domain" bind:value={defaultDomain} />
			<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
				{m.glossary_import_default_domain_hint()}
			</p>
		</div>

		<div class="mt-5">
			<Button
				icon="material-symbols:fact-check-outline"
				text={m.glossary_import_validate()}
				loading={busy === 'validate'}
				disabled={!file || busy !== null}
				click={() => run('validate')}
			/>
		</div>
	</section>

	{#if result}
		<section class={card} aria-labelledby="import-result">
			<div class="flex flex-wrap items-start justify-between gap-4">
				<div>
					<h2 id="import-result" class="text-base font-semibold text-gray-900 dark:text-gray-100">
						3. {m.glossary_import_result_heading()}
					</h2>
					<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
						{#if result.applied}
							{m.glossary_import_result_applied()}
						{:else if stale}
							{m.glossary_import_result_stale()}
						{:else if canApply}
							{m.glossary_import_result_ready()}
						{:else}
							{m.glossary_import_result_blocked()}
						{/if}
					</p>
				</div>
				{#if result.applied}
					<Button
						variant="clear"
						icon="material-symbols:menu-book-outline"
						text={m.glossary_import_back()}
						href={resolve('/glossary/[[id]]', { id: undefined })}
					/>
				{:else if canApply}
					<Button
						icon="material-symbols:done-all"
						text={m.glossary_import_apply({
							count: result.summary.create + result.summary.update
						})}
						loading={busy === 'apply'}
						disabled={busy !== null}
						click={() => run('apply')}
					/>
				{/if}
			</div>

			{#each [...(result.problems ?? []).map( (p) => ({ p, kind: 'error' }) ), ...(result.warnings ?? []).map( (p) => ({ p, kind: 'warning' }) )] as item, i (i)}
				<p
					class="mt-3 rounded-md px-3 py-2 text-sm {item.kind === 'error'
						? 'bg-red-50 text-red-800 dark:bg-red-900/20 dark:text-red-200'
						: 'bg-amber-50 text-amber-800 dark:bg-amber-900/20 dark:text-amber-200'}"
				>
					{#if item.p.column}<span class="mr-1 font-mono">{item.p.column}:</span>{/if}{describe(
						item.p
					)}
				</p>
			{/each}

			<div class="mt-4 flex flex-wrap gap-2" role="tablist" aria-label={m.glossary_import_filter()}>
				{#each [{ key: 'all', count: result.rows.length, label: m.glossary_import_filter_all() }, { key: 'error', count: result.summary.errors, label: actionLabel.error() }, { key: 'create', count: result.summary.create, label: actionLabel.create() }, { key: 'update', count: result.summary.update, label: actionLabel.update() }, { key: 'skip', count: result.summary.skip, label: actionLabel.skip() }] as tab (tab.key)}
					<button
						type="button"
						role="tab"
						aria-selected={filter === tab.key}
						class="rounded-full border px-3 py-1 text-xs font-medium {filter === tab.key
							? 'border-earthy-terracotta-600 text-earthy-terracotta-800 dark:text-earthy-terracotta-300'
							: 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-gray-700 dark:text-gray-300'}"
						onclick={() => (filter = tab.key as ImportAction | 'all')}
					>
						{tab.label} ({tab.count})
					</button>
				{/each}
			</div>

			{#if rows.length > 0}
				<div class="mt-4 overflow-x-auto rounded-md border border-gray-200 dark:border-gray-700">
					<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
						<thead class="bg-gray-50 dark:bg-gray-900/40">
							<tr>
								<th class={head}>{m.glossary_import_row_line()}</th>
								<th class={head}>{m.glossary_import_col_name()}</th>
								<th class={head}>{m.glossary_import_row_action()}</th>
								<th class={head}>{m.glossary_import_row_details()}</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-100 dark:divide-gray-700">
							{#each rows as row (row.line)}
								<tr>
									<td class="{cell} font-mono text-xs">{row.line}</td>
									<td class="{cell} font-medium text-gray-900 dark:text-gray-100">{row.name}</td>
									<td class={cell}>
										<span
											class="rounded-full px-2 py-0.5 text-xs font-medium {actionClass[row.action]}"
											>{actionLabel[row.action]()}</span
										>
									</td>
									<td class={cell}>
										{#each row.errors ?? [] as p, i (i)}
											<p class="text-red-700 dark:text-red-400">
												{#if p.column}<span class="mr-1 font-mono text-xs">{p.column}:</span
													>{/if}{describe(p)}
											</p>
										{/each}
										{#each row.warnings ?? [] as p, i (i)}
											<p class="text-amber-700 dark:text-amber-400">
												{#if p.column}<span class="mr-1 font-mono text-xs">{p.column}:</span
													>{/if}{describe(p)}
											</p>
										{/each}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</section>
	{/if}
</div>
