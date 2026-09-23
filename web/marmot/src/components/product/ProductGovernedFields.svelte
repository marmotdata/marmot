<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { fetchApi } from '$lib/api';
	import { toasts } from '$lib/stores/toast';
	import { locale } from '$lib/i18n';
	import { m } from '$lib/paraglide/messages';
	import Avatar from '$components/user/Avatar.svelte';
	import { createKeyboardNavigationState } from '$lib/keyboard';
	import type { MetamodelField, MetamodelSchema } from '$lib/metamodel/types';
	import { isValidationError, MetamodelHttpError } from '$lib/metamodel/api';
	import { nativeMessage, violationMessage } from '$lib/metamodel/i18n';
	import { resolveMessage } from '$lib/metamodel/labels';
	import {
		lookupOwnerById,
		searchUsers as searchUserOwners,
		type OwnerResult
	} from '$lib/metamodel/owners';
	import {
		draftFromValue,
		isUnset,
		readMetadataValue,
		sameValue,
		sectionLabel,
		toPayload,
		typeIcon,
		typeLabel,
		valueClass,
		writeMetadataValue,
		type Draft
	} from '$lib/metamodel/values';

	// Data products have no PATCH/ETag yet (see the C1 plan): every governed edit replaces the
	// whole metadata object through the same PUT the free-metadata editor already uses.
	let {
		metadata = $bindable(),
		productId,
		schema,
		fields,
		editable = false
	}: {
		metadata: Record<string, unknown>;
		productId: string | undefined;
		schema: MetamodelSchema;
		fields: MetamodelField[];
		editable?: boolean;
	} = $props();

	let editingId = $state<string | null>(null);
	let draft = $state<Draft>('');
	let pending = $state('');
	let errorCode = $state<string | null>(null);
	let saving = $state(false);

	let listboxOpen = $state(false);
	let listboxRoot = $state<HTMLDivElement>();

	let userQuery = $state('');
	let userResults = $state<OwnerResult[]>([]);
	let userSearching = $state(false);
	let userFocusedIndex = $state(-1);
	let userSearchTimeout: ReturnType<typeof setTimeout>;

	let resolvedOwners = $state<Record<string, OwnerResult | null>>({});
	const pendingLookups: Record<string, true> = {};

	const context = $derived({
		locale: $locale,
		defaultLocale: schema.defaultLocale,
		messages: schema.messages,
		native: nativeMessage
	});

	const hasMultipleSections = $derived.by(() => {
		const seen: string[] = [];
		for (const field of fields) {
			const section = field.presentation?.section ?? '';
			if (!seen.includes(section)) seen.push(section);
		}
		return seen.length > 1;
	});

	const neededOwnerIds = $derived.by(() => {
		const ids: string[] = [];
		for (const field of fields) {
			if (field.presentation?.control !== 'user') continue;
			const value = readMetadataValue(metadata, field.storage);
			if (typeof value === 'string' && value !== '' && !ids.includes(value)) ids.push(value);
		}
		return ids;
	});

	$effect(() => {
		for (const id of neededOwnerIds) {
			if (id in resolvedOwners || pendingLookups[id]) continue;
			pendingLookups[id] = true;
			void lookupOwner(id);
		}
	});

	async function lookupOwner(id: string) {
		try {
			resolvedOwners = { ...resolvedOwners, [id]: await lookupOwnerById(id) };
		} finally {
			delete pendingLookups[id];
		}
	}

	function label(field: MetamodelField): string {
		return resolveMessage(field.presentation?.labelKey, context) ?? field.id;
	}

	function help(field: MetamodelField): string | undefined {
		return resolveMessage(field.presentation?.helpTextKey, context);
	}

	function text(value: unknown): string {
		return typeof value === 'object' ? JSON.stringify(value) : String(value);
	}

	function isEmptyValue(value: unknown): boolean {
		return isUnset(value) || (Array.isArray(value) && value.length === 0);
	}

	function focusIf(node: HTMLElement, on: boolean) {
		if (on) node.focus();
	}

	function resetUserSearch() {
		userQuery = '';
		userResults = [];
		userFocusedIndex = -1;
		userSearching = false;
		clearTimeout(userSearchTimeout);
	}

	function startEdit(field: MetamodelField, value: unknown) {
		editingId = field.id;
		draft = draftFromValue(field, value);
		pending = '';
		errorCode = null;
		listboxOpen = false;
		resetUserSearch();
	}

	function cancel() {
		editingId = null;
		errorCode = null;
		listboxOpen = false;
		resetUserSearch();
	}

	function onKey(event: KeyboardEvent, field: MetamodelField) {
		if (event.key === 'Escape') {
			event.preventDefault();
			if (listboxOpen) {
				listboxOpen = false;
				return;
			}
			cancel();
		} else if (event.key === 'Enter') {
			event.preventDefault();
			if (field.type === 'list' && field.itemType !== 'enum' && pending.trim() !== '') addPending();
			else void save(field);
		}
	}

	function singleSelectOptions(field: MetamodelField): { value: string; label: string }[] {
		const options = field.required ? [] : [{ value: '', label: m.metamodel_not_set() }];
		if (field.type === 'boolean') {
			return [
				...options,
				{ value: 'true', label: m.metamodel_yes() },
				{ value: 'false', label: m.metamodel_no() }
			];
		}
		return [...options, ...(field.values ?? []).map((value) => ({ value, label: value }))];
	}

	function toggleListbox(event: MouseEvent) {
		event.stopPropagation();
		listboxOpen = !listboxOpen;
	}

	function selectOption(value: string) {
		draft = value;
		listboxOpen = false;
		errorCode = null;
	}

	function handleWindowClick(event: MouseEvent) {
		if (listboxOpen && !listboxRoot?.contains(event.target as Node)) listboxOpen = false;
	}

	function addPending() {
		const item = pending.trim();
		const items = Array.isArray(draft) ? draft : [];
		if (item && !items.includes(item)) draft = [...items, item];
		pending = '';
		errorCode = null;
	}

	function removeItem(item: string) {
		draft = (Array.isArray(draft) ? draft : []).filter((entry) => entry !== item);
	}

	function toggleItem(item: string, on: boolean) {
		const items = Array.isArray(draft) ? draft : [];
		draft = on ? [...items, item] : items.filter((entry) => entry !== item);
		errorCode = null;
	}

	function searchUsers(query: string) {
		userQuery = query;
		clearTimeout(userSearchTimeout);
		if (query.trim().length < 2) {
			userResults = [];
			userFocusedIndex = -1;
			return;
		}
		userSearchTimeout = setTimeout(async () => {
			userSearching = true;
			try {
				userResults = await searchUserOwners(query);
			} catch {
				userResults = [];
			} finally {
				userSearching = false;
				userFocusedIndex = -1;
			}
		}, 300);
	}

	function pickUser(owner: OwnerResult) {
		draft = owner.id;
		resolvedOwners = { ...resolvedOwners, [owner.id]: owner };
		resetUserSearch();
		errorCode = null;
	}

	const userSearchNav = createKeyboardNavigationState(
		() => userResults,
		() => userFocusedIndex,
		(i) => (userFocusedIndex = i),
		{ onSelect: pickUser, onEscape: () => (userQuery ? resetUserSearch() : cancel()) }
	);

	// Local mode (no productId, e.g. the create wizard): just update the bound metadata, the
	// same way MetadataView's own free-field editor does before the product exists to PUT to.
	async function save(field: MetamodelField) {
		if (saving) return;
		if (field.type === 'list' && field.itemType !== 'enum') addPending();
		const current = readMetadataValue(metadata, field.storage);
		const emptyDraft = Array.isArray(draft) ? draft.length === 0 : (draft ?? '').trim() === '';
		if (emptyDraft && isEmptyValue(current) && !field.required) {
			cancel();
			return;
		}
		const parsed = toPayload(field, draft);
		if (!parsed.ok) {
			errorCode = parsed.code;
			return;
		}
		if (sameValue(current, parsed.value)) {
			cancel();
			return;
		}
		const updated = writeMetadataValue(metadata, field.storage, parsed.value);

		if (!productId) {
			metadata = updated;
			editingId = null;
			return;
		}

		saving = true;
		errorCode = null;
		try {
			const response = await fetchApi(`/products/${productId}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ metadata: updated })
			});
			if (!response.ok) {
				const body = await response.json().catch(() => null);
				throw new MetamodelHttpError(response.status, body, 'Failed to save governed field');
			}
			const saved = await response.json();
			metadata = saved?.metadata ?? updated;
			editingId = null;
		} catch (err) {
			if (err instanceof MetamodelHttpError && err.status === 400 && isValidationError(err.body)) {
				errorCode = err.body.fields.find((v) => v.field === field.id)?.code ?? 'unknown';
			} else {
				toasts.error(m.metamodel_save_failed());
			}
		} finally {
			saving = false;
		}
	}
</script>

{#snippet ownerChip(id: string, removable: boolean)}
	{@const owner = resolvedOwners[id]}
	<span
		class="group/chip inline-flex max-w-full items-center gap-2 rounded-full bg-gray-100 py-1 pl-1 pr-2.5 dark:bg-gray-700"
	>
		{#if owner === null}
			<span
				class="flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full bg-gray-200 text-gray-500 dark:bg-gray-600 dark:text-gray-400"
			>
				<IconifyIcon icon="material-symbols:person-off-outline-rounded" class="h-3 w-3" />
			</span>
			<span class="truncate text-sm italic text-gray-500 dark:text-gray-400">
				{m.metamodel_unknown_user()}
			</span>
		{:else}
			<Avatar name={owner?.name ?? id} profilePicture={owner?.profile_picture} size="xs" />
			<span class="truncate text-sm text-gray-900 dark:text-gray-100">{owner?.name ?? id}</span>
		{/if}
		{#if removable}
			<button
				type="button"
				onclick={() => (draft = '')}
				class="flex-shrink-0 rounded-full p-0.5 text-gray-400 opacity-0 transition-opacity group-hover/chip:opacity-100 hover:bg-gray-300 hover:text-gray-800 focus-visible:opacity-100 dark:hover:bg-gray-600 dark:hover:text-gray-100"
				aria-label={m.metamodel_list_remove({ value: owner?.name ?? id })}
			>
				<IconifyIcon icon="material-symbols:close-rounded" class="h-3.5 w-3.5" />
			</button>
		{/if}
	</span>
{/snippet}

{#snippet display(field: MetamodelField, value: unknown)}
	{#if isEmptyValue(value)}
		<span
			class="inline-flex items-center gap-1 text-sm italic {field.required
				? 'text-red-600 dark:text-red-400'
				: 'text-gray-400 dark:text-gray-500'}"
		>
			{#if field.required}
				<IconifyIcon icon="material-symbols:error-outline-rounded" class="h-3.5 w-3.5" />
			{/if}
			{m.metamodel_not_set()}
		</span>
	{:else if field.presentation?.control === 'user' && typeof value === 'string'}
		{@render ownerChip(value, false)}
	{:else if Array.isArray(value)}
		<div class="flex flex-wrap gap-1.5">
			{#each value as item, i (i)}
				<span
					class="rounded-full bg-earthy-terracotta-100 px-2 py-0.5 text-xs whitespace-pre-wrap break-all text-earthy-terracotta-700 dark:bg-earthy-terracotta-900 dark:text-earthy-terracotta-100"
				>
					{text(item)}
				</span>
			{/each}
		</div>
	{:else if typeof value === 'boolean'}
		<span class="rounded-full px-2 py-1 text-sm {valueClass(value)}">
			{value ? m.metamodel_yes() : m.metamodel_no()}
		</span>
	{:else}
		<span class="rounded-full px-2 py-1 text-sm {valueClass(value)}">{text(value)}</span>
	{/if}
{/snippet}

{#snippet userEditor(field: MetamodelField, controlId: string, described: string | undefined)}
	{@const selectedId = typeof draft === 'string' ? draft : ''}
	{#if selectedId}
		{@render ownerChip(selectedId, true)}
	{:else}
		<div class="relative">
			<input
				id={controlId}
				type="text"
				class="w-full rounded border border-earthy-terracotta-500 bg-white px-2 py-1.5 pl-8 text-sm text-gray-900 focus:ring-1 focus:ring-earthy-terracotta-600 dark:border-earthy-terracotta-700 dark:bg-gray-800 dark:text-gray-100"
				placeholder={m.owners_search_users_placeholder()}
				aria-labelledby={`governed-product-label-${field.id}`}
				aria-describedby={described}
				aria-expanded={userQuery.trim().length >= 2}
				role="combobox"
				aria-controls={`${controlId}-listbox`}
				autocomplete="off"
				value={userQuery}
				oninput={(e) => searchUsers(e.currentTarget.value)}
				onkeydown={userSearchNav.handleKeydown}
				use:focusIf={true}
			/>
			<IconifyIcon
				icon="material-symbols:search-rounded"
				class="pointer-events-none absolute top-1/2 left-2 h-4 w-4 -translate-y-1/2 text-gray-400"
			/>
			{#if userQuery.trim().length >= 2}
				<div
					id={`${controlId}-listbox`}
					role="listbox"
					class="absolute z-10 mt-1 max-h-52 w-full min-w-64 overflow-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
				>
					{#if userSearching}
						<div class="px-3 py-3 text-sm text-gray-500 dark:text-gray-400">
							{m.owners_searching()}
						</div>
					{:else if userResults.length === 0}
						<div class="px-3 py-3 text-sm text-gray-500 dark:text-gray-400">
							{m.metamodel_no_users_found()}
						</div>
					{:else}
						{#each userResults as owner, i (owner.id)}
							<button
								type="button"
								role="option"
								aria-selected={i === userFocusedIndex}
								onclick={() => pickUser(owner)}
								class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors {i ===
								userFocusedIndex
									? 'bg-gray-100 dark:bg-gray-700'
									: 'hover:bg-gray-50 dark:hover:bg-gray-700/50'}"
							>
								<Avatar name={owner.name} profilePicture={owner.profile_picture} size="xs" />
								<span class="min-w-0 flex-1 truncate text-gray-900 dark:text-gray-100"
									>{owner.name}</span
								>
								{#if owner.username}
									<span class="flex-shrink-0 text-xs text-gray-400">@{owner.username}</span>
								{/if}
							</button>
						{/each}
					{/if}
				</div>
			{/if}
		</div>
	{/if}
{/snippet}

{#snippet singleSelect(field: MetamodelField, controlId: string, described: string | undefined)}
	{@const value = typeof draft === 'string' ? draft : ''}
	{@const options = singleSelectOptions(field)}
	{@const currentLabel = options.find((o) => o.value === value)?.label ?? m.metamodel_not_set()}
	<div class="relative inline-flex w-full" bind:this={listboxRoot}>
		<button
			type="button"
			id={controlId}
			aria-haspopup="listbox"
			aria-expanded={listboxOpen}
			aria-labelledby={`governed-product-label-${field.id} ${controlId}`}
			aria-describedby={described}
			onclick={toggleListbox}
			onkeydown={(e) => onKey(e, field)}
			class="flex w-full items-center justify-between gap-2 rounded border border-earthy-terracotta-500 bg-white px-2 py-1.5 text-left text-sm text-gray-900 focus:border-transparent focus:ring-2 focus:ring-earthy-terracotta-500 focus:outline-none dark:border-earthy-terracotta-700 dark:bg-gray-800 dark:text-gray-100"
			use:focusIf={true}
		>
			<span class="min-w-0 truncate">{currentLabel}</span>
			<IconifyIcon
				icon="material-symbols:keyboard-arrow-down"
				class="h-4 w-4 shrink-0 text-gray-500 transition-transform dark:text-gray-400 {listboxOpen
					? 'rotate-180'
					: ''}"
			/>
		</button>
		{#if listboxOpen}
			<div
				role="listbox"
				aria-labelledby={`governed-product-label-${field.id}`}
				class="absolute top-full left-0 z-10 mt-1 max-h-60 w-full min-w-max overflow-y-auto overscroll-contain rounded-md border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
			>
				{#each options as option (option.value)}
					<button
						type="button"
						role="option"
						aria-selected={option.value === value}
						onclick={(e) => {
							e.stopPropagation();
							selectOption(option.value);
						}}
						class="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm transition-colors {option.value ===
						value
							? 'font-medium text-earthy-terracotta-700 dark:text-earthy-terracotta-500'
							: 'text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700'}"
					>
						<span>{option.label}</span>
						{#if option.value === value}
							<span
								class="h-1.5 w-1.5 rounded-full bg-earthy-terracotta-700 dark:bg-earthy-terracotta-500"
								aria-hidden="true"
							></span>
						{/if}
					</button>
				{/each}
			</div>
		{/if}
	</div>
{/snippet}

{#snippet editor(field: MetamodelField)}
	{@const controlId = `governed-product-${field.id}`}
	{@const rules = field.validation ?? {}}
	{@const scalar = typeof draft === 'string' ? draft : ''}
	{@const described = errorCode ? `governed-product-error-${field.id}` : undefined}
	<div class="flex items-start gap-2">
		<div class="min-w-0 flex-1">
			{#if field.presentation?.control === 'user'}
				{@render userEditor(field, controlId, described)}
			{:else if field.type === 'integer' || field.type === 'number'}
				<input
					id={controlId}
					type="number"
					step={field.type === 'integer' ? '1' : 'any'}
					min={rules.minimum}
					max={rules.maximum}
					class="w-full rounded border border-earthy-terracotta-500 bg-white px-2 py-1.5 text-sm text-gray-900 focus:ring-1 focus:ring-earthy-terracotta-600 dark:border-earthy-terracotta-700 dark:bg-gray-800 dark:text-gray-100"
					aria-labelledby={`governed-product-label-${field.id}`}
					aria-invalid={errorCode ? true : undefined}
					aria-describedby={described}
					value={scalar}
					oninput={(e) => (draft = e.currentTarget.value)}
					onkeydown={(e) => onKey(e, field)}
					use:focusIf={true}
				/>
			{:else if field.type === 'enum' || field.type === 'boolean'}
				{@render singleSelect(field, controlId, described)}
			{:else if field.type === 'date'}
				<input
					id={controlId}
					type="date"
					class="w-full rounded border border-earthy-terracotta-500 bg-white px-2 py-1.5 text-sm text-gray-900 focus:ring-1 focus:ring-earthy-terracotta-600 dark:border-earthy-terracotta-700 dark:bg-gray-800 dark:text-gray-100"
					aria-labelledby={`governed-product-label-${field.id}`}
					aria-invalid={errorCode ? true : undefined}
					aria-describedby={described}
					value={scalar}
					oninput={(e) => (draft = e.currentTarget.value)}
					onkeydown={(e) => onKey(e, field)}
					use:focusIf={true}
				/>
			{:else if field.type === 'list' && field.itemType === 'enum'}
				<div
					role="group"
					aria-labelledby={`governed-product-label-${field.id}`}
					aria-describedby={described}
					class="flex flex-wrap gap-x-4 gap-y-1"
				>
					{#each field.values ?? [] as option, i (option)}
						<label class="flex items-center gap-1.5 text-sm text-gray-700 dark:text-gray-300">
							<input
								type="checkbox"
								class="rounded border-gray-300 dark:border-gray-600"
								checked={Array.isArray(draft) && draft.includes(option)}
								onchange={(e) => toggleItem(option, e.currentTarget.checked)}
								onkeydown={(e) => onKey(e, field)}
								use:focusIf={i === 0}
							/>
							{option}
						</label>
					{/each}
				</div>
			{:else if field.type === 'list'}
				<div class="flex flex-wrap items-center gap-1.5">
					{#each Array.isArray(draft) ? draft : [] as item (item)}
						<span
							class="inline-flex items-center gap-1 rounded-full bg-earthy-terracotta-100 px-2 py-0.5 text-xs whitespace-pre-wrap break-all text-earthy-terracotta-700 dark:bg-earthy-terracotta-900 dark:text-earthy-terracotta-100"
						>
							{item}
							<button
								type="button"
								class="rounded hover:text-earthy-terracotta-900 dark:hover:text-white"
								aria-label={m.metamodel_list_remove({ value: item })}
								onclick={() => removeItem(item)}
							>
								<IconifyIcon icon="material-symbols:close-rounded" class="w-3.5 h-3.5" />
							</button>
						</span>
					{/each}
				</div>
				<input
					id={controlId}
					type={field.itemType === 'integer' || field.itemType === 'number' ? 'number' : 'text'}
					step={field.itemType === 'integer' ? '1' : 'any'}
					class="mt-1.5 w-full rounded border border-earthy-terracotta-500 bg-white px-2 py-1.5 text-sm text-gray-900 focus:ring-1 focus:ring-earthy-terracotta-600 dark:border-earthy-terracotta-700 dark:bg-gray-800 dark:text-gray-100"
					placeholder={m.metamodel_list_placeholder()}
					aria-labelledby={`governed-product-label-${field.id}`}
					aria-invalid={errorCode ? true : undefined}
					aria-describedby={described}
					bind:value={pending}
					onkeydown={(e) => onKey(e, field)}
					use:focusIf={true}
				/>
			{:else}
				<input
					id={controlId}
					type="text"
					maxlength={rules.maxLength}
					class="w-full rounded border border-earthy-terracotta-500 bg-white px-2 py-1.5 text-sm text-gray-900 focus:ring-1 focus:ring-earthy-terracotta-600 dark:border-earthy-terracotta-700 dark:bg-gray-800 dark:text-gray-100"
					aria-labelledby={`governed-product-label-${field.id}`}
					aria-invalid={errorCode ? true : undefined}
					aria-describedby={described}
					value={scalar}
					oninput={(e) => (draft = e.currentTarget.value)}
					onkeydown={(e) => onKey(e, field)}
					use:focusIf={true}
				/>
			{/if}
			{#if errorCode}
				<p
					id={`governed-product-error-${field.id}`}
					role="alert"
					class="mt-1 text-xs text-red-600 dark:text-red-400"
				>
					{violationMessage(errorCode)}
				</p>
			{/if}
		</div>
		<div class="flex flex-shrink-0 items-center gap-1">
			<button
				type="button"
				onclick={() => save(field)}
				disabled={saving}
				class="rounded p-1.5 text-green-600 transition-colors hover:bg-green-50 disabled:opacity-50 dark:text-green-500 dark:hover:bg-green-900/20"
				title={m.common_save()}
				aria-label={m.common_save()}
			>
				<IconifyIcon icon="material-symbols:check-rounded" class="h-5 w-5" />
			</button>
			<button
				type="button"
				onclick={cancel}
				disabled={saving}
				class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-100 dark:hover:bg-gray-700"
				title={m.common_cancel()}
				aria-label={m.common_cancel()}
			>
				<IconifyIcon icon="material-symbols:close-rounded" class="h-5 w-5" />
			</button>
		</div>
	</div>
{/snippet}

<svelte:window onclick={handleWindowClick} />

{#each fields as field, i (field.id)}
	{@const section = field.presentation?.section ?? ''}
	{@const previousSection = i > 0 ? (fields[i - 1].presentation?.section ?? '') : undefined}
	{@const value = readMetadataValue(metadata, field.storage)}
	{@const helpText = help(field)}
	{#if hasMultipleSections && section !== previousSection}
		<tr data-governed-section={section}>
			<td
				colspan={editable ? 3 : 2}
				class="bg-gray-50/60 px-4 pt-4 pb-1.5 text-xs font-semibold tracking-wide text-gray-400 uppercase dark:bg-gray-900/40 dark:text-gray-500"
			>
				{section ? sectionLabel(section, context) : m.metamodel_other_section()}
			</td>
		</tr>
	{/if}
	<tr
		class="group border-b border-gray-200 transition-colors dark:border-gray-700 {field.required
			? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20'
			: 'hover:bg-gray-50 dark:hover:bg-gray-700/30'}"
		data-governed-field={field.id}
	>
		<td
			class="w-64 px-4 py-3 align-top {field.required
				? 'border-l-2 border-l-earthy-terracotta-600'
				: ''}"
		>
			<div
				id={`governed-product-label-${field.id}`}
				class="flex items-baseline gap-1 text-sm font-medium text-gray-700 dark:text-gray-300"
			>
				<span>{label(field)}</span>
				{#if field.required}
					<span class="text-red-500" aria-hidden="true">*</span>
					<span class="sr-only">({m.metamodel_required()})</span>
				{/if}
				{#if helpText}
					<span class="text-gray-400 dark:text-gray-500" title={helpText} aria-hidden="true">
						<IconifyIcon icon="mdi:information-outline" class="h-3.5 w-3.5" />
					</span>
				{/if}
			</div>
			<div class="mt-0.5 flex items-center gap-1 text-xs text-gray-400 dark:text-gray-500">
				<IconifyIcon icon={typeIcon(field)} class="h-3.5 w-3.5" />
				{typeLabel(field)}
			</div>
		</td>
		<td class="px-4 py-3 text-sm align-top">
			{#if editingId === field.id}
				{@render editor(field)}
			{:else}
				<div class="inline-flex items-center gap-1.5">
					{@render display(field, value)}
					{#if editable}
						<button
							type="button"
							onclick={() => startEdit(field, value)}
							disabled={saving}
							class="flex-shrink-0 rounded p-1.5 text-gray-400 opacity-0 transition-all group-hover:opacity-100 hover:bg-gray-100 hover:text-earthy-terracotta-700 focus-visible:opacity-100 dark:hover:bg-gray-700 dark:hover:text-earthy-terracotta-500"
							title={m.common_edit()}
							aria-label={`${m.common_edit()}: ${label(field)}`}
						>
							<IconifyIcon icon="material-symbols:edit-outline-rounded" class="h-4 w-4" />
						</button>
					{/if}
				</div>
			{/if}
		</td>
	</tr>
{/each}
