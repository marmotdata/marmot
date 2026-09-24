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
		flatten,
		loadTree
	} from '$lib/domains/api';
	import { domainName, isUnassigned } from '$lib/domains/labels';

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
	let editing = $state(false);
	let busy = $state(false);
	let options = $state<{ id: string; label: string }[]>([]);

	const mayChange = $derived(canEdit && auth.hasPermission('domains', 'manage'));

	$effect(() => {
		const id = entityId;
		let cancelled = false;
		current = null;
		editing = false;
		domainsEnabled().then(async (enabled) => {
			if (!enabled || cancelled) return;
			try {
				const domain = await domainOf(kind, id);
				if (!cancelled) current = domain;
			} catch {
				// Without a readable domain the section stays hidden.
			}
		});
		return () => {
			cancelled = true;
		};
	});

	async function openEditor() {
		try {
			options = flatten(await loadTree()).map((e) => ({
				id: e.domain.id,
				label: e.domain.parent_id ? e.label : domainName(e.domain)
			}));
			editing = true;
		} catch (error) {
			toasts.error(errorMessage(error));
		}
	}

	async function choose(domainId: string) {
		if (!current || domainId === current.id) {
			editing = false;
			return;
		}
		busy = true;
		try {
			await assignToDomain(domainId, kind, [entityId]);
			current = await domainOf(kind, entityId);
			toasts.success(m.domains_assigned());
			editing = false;
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

		{#if editing}
			<select
				class="rounded-md border-gray-300 py-1 text-sm dark:border-gray-600 dark:bg-gray-900"
				aria-label={m.domains_change()}
				disabled={busy}
				value={current.id}
				onchange={(e) => choose(e.currentTarget.value)}
				onkeydown={(e) => e.key === 'Escape' && (editing = false)}
			>
				{#each options as option (option.id)}
					<option value={option.id}>{option.label}</option>
				{/each}
			</select>
		{:else}
			<div class="flex items-center gap-1">
				<a
					href={resolve('/domains/[[id]]', { id: current.id })}
					class="inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600 {isUnassigned(
						current
					)
						? 'italic'
						: ''}">{domainName(current)}</a
				>
				{#if mayChange}
					<button
						type="button"
						class="rounded p-1 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
						aria-label={m.domains_change()}
						title={m.domains_change()}
						onclick={openEditor}
					>
						<Icon icon="material-symbols:edit-outline" class="h-3.5 w-3.5" />
					</button>
				{/if}
			</div>
		{/if}
	</div>
{/if}
