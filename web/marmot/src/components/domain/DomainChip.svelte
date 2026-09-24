<script lang="ts">
	import Icon from '@iconify/svelte';
	import { resolve } from '$app/paths';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import type { Domain, DomainKind } from '$lib/domains/types';
	import {
		assignToDomain,
		domainOf,
		domainsEnabled,
		errorMessage,
		loadTree,
		writableDomains
	} from '$lib/domains/api';
	import { domainName, isUnassigned } from '$lib/domains/labels';
	import { domainOptions, type DomainOption } from '$lib/domains/options';
	import DomainPicker from './DomainPicker.svelte';

	let {
		kind,
		entityId,
		canEdit = false,
		variant = 'compact',
		link = true
	}: {
		kind: DomainKind;
		entityId: string;
		/** Whether the user may edit the entity; the server also checks both domains under write enforcement. */
		canEdit?: boolean;
		variant?: 'compact' | 'section';
		/** False inside another link (the Discover side panel header): plain, read-only chip. */
		link?: boolean;
	} = $props();

	let current = $state<Domain | null>(null);
	let options = $state<DomainOption[] | null>(null);
	let busy = $state(false);

	const mayChange = $derived(link && canEdit);

	$effect(() => {
		const id = entityId;
		let cancelled = false;
		current = null;
		domainsEnabled().then(async (enabled) => {
			if (!enabled || cancelled) return;
			try {
				const [domain, forest, writable] = await Promise.all([
					domainOf(kind, id),
					loadTree(),
					writableDomains()
				]);
				if (cancelled) return;
				current = domain;
				options = domainOptions(forest, { includeUnassigned: true, writable });
			} catch {
				// Without a readable domain the section stays hidden.
			}
		});
		return () => {
			cancelled = true;
		};
	});

	// The full path, as the picker shows it, so every chip reads the same.
	const label = $derived(
		current ? (options?.find((o) => o.id === current?.id)?.path ?? domainName(current)) : ''
	);

	async function choose(domainId: string) {
		if (!current || domainId === current.id) return;
		busy = true;
		try {
			await assignToDomain(domainId, kind, [entityId]);
			current = await domainOf(kind, entityId);
			toasts.success(m.domains_assigned());
		} catch (error) {
			toasts.error(errorMessage(error));
		} finally {
			busy = false;
		}
	}
</script>

{#if current}
	<div>
		{#if variant === 'section'}
			<div class="mb-2 flex items-center gap-2">
				<Icon
					icon="material-symbols:account-tree-outline-rounded"
					class="h-4 w-4 text-gray-500 dark:text-gray-400"
				/>
				<h3 class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
					{m.domains_chip_label()}
				</h3>
			</div>
		{:else}
			<div class="mb-1 flex items-center gap-1.5">
				<Icon
					icon="material-symbols:account-tree-outline-rounded"
					class="h-3.5 w-3.5 text-gray-400"
				/>
				<span class="text-xs font-medium uppercase tracking-wide text-gray-400"
					>{m.domains_chip_label()}</span
				>
			</div>
		{/if}

		{#if mayChange && options}
			<div class="flex items-center gap-1">
				<DomainPicker
					variant="chip"
					value={current.id}
					{options}
					label={m.domains_change()}
					disabled={busy}
					onSelect={choose}
				/>
				<a
					href={resolve('/domains/[[id]]', { id: current.id })}
					class="rounded p-1 text-gray-400 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-500"
					aria-label={m.domains_open({ name: domainName(current) })}
					title={m.domains_open({ name: domainName(current) })}
				>
					<Icon icon="material-symbols:open-in-new-rounded" class="h-3.5 w-3.5" />
				</a>
			</div>
		{:else if !link}
			<span
				class="inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs text-gray-700 dark:bg-gray-700 dark:text-gray-200 {isUnassigned(
					current
				)
					? 'italic'
					: ''}">{label}</span
			>
		{:else}
			<a
				href={resolve('/domains/[[id]]', { id: current.id })}
				class="inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600 {isUnassigned(
					current
				)
					? 'italic'
					: ''}">{label}</a
			>
		{/if}
	</div>
{/if}
