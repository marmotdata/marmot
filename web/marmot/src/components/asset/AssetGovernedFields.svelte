<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import Avatar from '$components/user/Avatar.svelte';
	import { locale } from '$lib/i18n';
	import { m } from '$lib/paraglide/messages';
	import type { Asset } from '$lib/assets/types';
	import type { MetamodelField } from '$lib/metamodel/types';
	import { fetchMetamodel } from '$lib/metamodel/api';
	import { nativeMessage } from '$lib/metamodel/i18n';
	import { resolveMessage } from '$lib/metamodel/labels';
	import { lookupOwnerById, type OwnerResult } from '$lib/metamodel/owners';
	import {
		governedFields,
		isUnset,
		readMetadataValue,
		sectionLabel,
		typeIcon,
		valueClass
	} from '$lib/metamodel/values';

	let { asset }: { asset: Asset } = $props();

	// GET /api/v1/metamodel is cached by fetchMetamodel; every blade instance shares one request.
	let fields = $state<MetamodelField[]>([]);
	let schemaMessages = $state<Record<string, Record<string, string>> | undefined>();
	let defaultLocale = $state('en');
	let resolvedOwners = $state<Record<string, OwnerResult | null>>({});
	const pendingLookups: Record<string, true> = {};

	$effect(() => {
		let cancelled = false;
		fetchMetamodel()
			.then((schema) => {
				if (cancelled) return;
				fields = schema.enabled ? governedFields(schema.fields) : [];
				schemaMessages = schema.messages;
				defaultLocale = schema.defaultLocale;
			})
			.catch(() => {
				if (!cancelled) fields = [];
			});
		return () => {
			cancelled = true;
		};
	});

	const context = $derived({
		locale: $locale,
		defaultLocale,
		messages: schemaMessages,
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
			const value = readMetadataValue(asset.metadata, field.storage);
			if (typeof value === 'string' && value !== '' && !ids.includes(value)) ids.push(value);
		}
		return ids;
	});

	$effect(() => {
		for (const id of neededOwnerIds) {
			if (id in resolvedOwners || pendingLookups[id]) continue;
			pendingLookups[id] = true;
			lookupOwnerById(id)
				.then((owner) => (resolvedOwners = { ...resolvedOwners, [id]: owner }))
				.finally(() => delete pendingLookups[id]);
		}
	});

	function label(field: MetamodelField): string {
		return resolveMessage(field.presentation?.labelKey, context) ?? field.id;
	}

	function text(value: unknown): string {
		return typeof value === 'object' ? JSON.stringify(value) : String(value);
	}

	function isEmptyValue(value: unknown): boolean {
		return isUnset(value) || (Array.isArray(value) && value.length === 0);
	}
</script>

{#if fields.length > 0}
	<div
		class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800"
	>
		<h4
			class="border-b border-gray-200 px-5 py-3 text-base font-semibold text-gray-900 dark:border-gray-700 dark:text-gray-100"
		>
			{m.discover_tab_metadata()}
		</h4>
		<dl>
			{#each fields as field, i (field.id)}
				{@const value = readMetadataValue(asset.metadata, field.storage)}
				{@const section = field.presentation?.section ?? ''}
				{@const previousSection = i > 0 ? (fields[i - 1].presentation?.section ?? '') : undefined}
				{@const helpText = resolveMessage(field.presentation?.helpTextKey, context)}
				{#if hasMultipleSections && section !== previousSection}
					<p
						class="bg-gray-50/60 px-5 pt-3 pb-1.5 text-xs font-semibold tracking-wide text-gray-400 uppercase dark:bg-gray-900/40 dark:text-gray-500"
					>
						{section ? sectionLabel(section, context) : m.metamodel_other_section()}
					</p>
				{/if}
				<div
					class="border-b border-gray-100 px-5 py-2.5 last:border-0 dark:border-gray-700/60 {field.required
						? 'border-l-2 border-l-earthy-terracotta-600 bg-earthy-terracotta-50/70 dark:bg-earthy-terracotta-900/10'
						: ''}"
				>
					<dt class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
						<IconifyIcon icon={typeIcon(field)} class="h-3.5 w-3.5 flex-shrink-0" />
						<span>{label(field)}</span>
						{#if field.required}
							<span class="text-red-500" aria-hidden="true">*</span>
						{/if}
						{#if helpText}
							<span class="text-gray-400 dark:text-gray-500" title={helpText} aria-hidden="true">
								<IconifyIcon icon="mdi:information-outline" class="h-3.5 w-3.5" />
							</span>
						{/if}
					</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-gray-100">
						{#if isEmptyValue(value)}
							<span
								class="italic {field.required
									? 'text-red-600 dark:text-red-400'
									: 'text-gray-400 dark:text-gray-500'}"
							>
								{m.metamodel_not_set()}
							</span>
						{:else if field.presentation?.control === 'user' && typeof value === 'string'}
							{@const owner = resolvedOwners[value]}
							{#if owner === null}
								<span
									class="inline-flex items-center gap-1.5 italic text-gray-500 dark:text-gray-400"
								>
									<IconifyIcon
										icon="material-symbols:person-off-outline-rounded"
										class="h-3.5 w-3.5"
									/>
									{m.metamodel_unknown_user()}
								</span>
							{:else}
								<span class="inline-flex items-center gap-1.5">
									<Avatar
										name={owner?.name ?? value}
										profilePicture={owner?.profile_picture}
										size="xs"
									/>
									{owner?.name ?? value}
								</span>
							{/if}
						{:else if Array.isArray(value)}
							<div class="flex flex-wrap gap-1">
								{#each value as item, itemIndex (itemIndex)}
									<span
										class="rounded-full bg-earthy-terracotta-100 px-2 py-0.5 text-xs break-all whitespace-pre-wrap text-earthy-terracotta-700 dark:bg-earthy-terracotta-900 dark:text-earthy-terracotta-100"
									>
										{text(item)}
									</span>
								{/each}
							</div>
						{:else if typeof value === 'boolean'}
							<span class="rounded-full px-2 py-0.5 text-xs {valueClass(value)}">
								{value ? m.metamodel_yes() : m.metamodel_no()}
							</span>
						{:else}
							<span class="rounded-full px-2 py-0.5 text-xs {valueClass(value)}">
								{text(value)}
							</span>
						{/if}
					</dd>
				</div>
			{/each}
		</dl>
	</div>
{/if}
