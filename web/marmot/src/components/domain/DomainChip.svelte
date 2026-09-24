<script lang="ts">
	import Icon from '@iconify/svelte';
	import { resolve } from '$app/paths';
	import { auth } from '$lib/stores/auth';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import type { Domain, DomainKind } from '$lib/domains/types';
	import {
		assignToDomain,
		domainOf,
		domainsEnabled,
		errorMessage,
		loadTree
	} from '$lib/domains/api';
	import { domainName, isUnassigned } from '$lib/domains/labels';
	import { domainOptions, type DomainOption } from '$lib/domains/options';
	import DomainPicker from './DomainPicker.svelte';

	let {
		kind,
		entityId,
		canEdit = false,
		variant = 'compact'
	}: {
		kind: DomainKind;
		entityId: string;
		/** Whether the user may edit the entity itself; assigning also needs domains:manage. */
		canEdit?: boolean;
		variant?: 'compact' | 'section';
	} = $props();

	let current = $state<Domain | null>(null);
	let options = $state<DomainOption[] | null>(null);
	let busy = $state(false);

	const mayChange = $derived(canEdit && auth.hasPermission('domains', 'manage'));

	$effect(() => {
		const id = entityId;
		const loadOptions = mayChange;
		let cancelled = false;
		current = null;
		domainsEnabled().then(async (enabled) => {
			if (!enabled || cancelled) return;
			try {
				const [domain, forest] = await Promise.all([
					domainOf(kind, id),
					loadOptions ? loadTree() : Promise.resolve(null)
				]);
				if (cancelled) return;
				current = domain;
				options = forest ? domainOptions(forest, { includeUnassigned: true }) : null;
			} catch {
				// Without a readable domain the section stays hidden.
			}
		});
		return () => {
			cancelled = true;
		};
	});

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
		{:else}
			<a
				href={resolve('/domains/[[id]]', { id: current.id })}
				class="inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600 {isUnassigned(
					current
				)
					? 'italic'
					: ''}">{domainName(current)}</a
			>
		{/if}
	</div>
{/if}
