<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import Icon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import DomainPicker from '$components/domain/DomainPicker.svelte';
	import DomainRoles from '$components/domain/DomainRoles.svelte';
	import { domainOptions, type DomainOption } from '$lib/domains/options';
	import { auth } from '$lib/stores/auth';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import type { DomainNode } from '$lib/domains/types';
	import {
		capabilities,
		createDomain,
		deleteDomain,
		errorMessage,
		flatten,
		loadTree,
		moveDomain,
		updateDomain
	} from '$lib/domains/api';
	import { discoverQuery, domainName, isUnassigned } from '$lib/domains/labels';

	type Mode = 'none' | 'create' | 'edit' | 'move' | 'delete';

	const inputClass =
		'w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:bg-gray-700 dark:text-gray-100';

	// Only a global manager creates top-level domains; everything else is
	// decided per domain by the server (capabilities).
	const canManage = auth.hasPermission('domains', 'manage');

	let forest = $state<DomainNode[]>([]);
	let loading = $state(true);
	let loadFailed = $state(false);
	let expanded = $state<Record<string, boolean>>({});
	let mode = $state<Mode>('none');
	let busy = $state(false);
	let formName = $state('');
	let formDescription = $state('');
	let formParent = $state('');

	const selectedId = $derived($page.params.id ?? null);
	const entries = $derived(flatten(forest));
	const byId = $derived(new Map(entries.map((e) => [e.domain.id, e])));
	const selected = $derived(selectedId ? (byId.get(selectedId)?.domain ?? null) : null);
	const regularRoots = $derived(forest.filter((d) => !isUnassigned(d)));
	const unassignedRoot = $derived(forest.find((d) => isUnassigned(d)) ?? null);

	const ancestors = $derived.by(() => {
		if (!selected) return [];
		return selected.path
			.split('/')
			.filter((id) => id && id !== selected.id)
			.map((id) => byId.get(id)?.domain)
			.filter((d): d is DomainNode => !!d);
	});

	// A domain cannot move under itself, its subtree or Unassigned.
	const moveTargets = $derived(
		domainOptions(forest, { exclude: (d) => !!selected && d.path.startsWith(selected.path) })
	);
	const createParents = $derived(domainOptions(forest));

	// admin: edit it and create subdomains. parentAdmin: move or delete it.
	let caps = $state<{ admin: boolean; parentAdmin: boolean } | null>(null);

	$effect(() => {
		const d = selected;
		caps = null;
		if (!d) return;
		let cancelled = false;
		Promise.all([
			capabilities(d.id),
			d.parent_id ? capabilities(d.parent_id) : Promise.resolve({ admin: canManage })
		])
			.then(([own, parent]) => {
				if (!cancelled) caps = { admin: own.admin, parentAdmin: parent.admin };
			})
			.catch(() => {
				if (!cancelled) caps = { admin: false, parentAdmin: false };
			});
		return () => {
			cancelled = true;
		};
	});

	async function refresh() {
		try {
			forest = await loadTree({ fresh: true });
			loadFailed = false;
		} catch {
			loadFailed = true;
		} finally {
			loading = false;
		}
	}

	onMount(refresh);

	$effect(() => {
		// Keep the selected domain visible by expanding its ancestors.
		for (const a of ancestors) expanded[a.id] = true;
	});

	function select(id: string | null) {
		mode = 'none';
		goto(id ? resolve('/domains/[[id]]', { id }) : resolve('/domains/[[id]]', {}));
	}

	function startCreate(parentId: string) {
		formName = '';
		formDescription = '';
		formParent = parentId;
		mode = 'create';
	}

	function startEdit() {
		if (!selected) return;
		formName = selected.name;
		formDescription = selected.description ?? '';
		mode = 'edit';
	}

	function startMove() {
		if (!selected) return;
		formParent = selected.parent_id ?? '';
		mode = 'move';
	}

	async function run(action: () => Promise<unknown>, success: string) {
		busy = true;
		try {
			await action();
			toasts.success(success);
			return true;
		} catch (error) {
			toasts.error(errorMessage(error));
			return false;
		} finally {
			busy = false;
		}
	}

	async function submitCreate() {
		let createdId = '';
		const ok = await run(async () => {
			const created = await createDomain({
				name: formName,
				description: formDescription || undefined,
				parent_id: formParent || undefined
			});
			createdId = created.id;
		}, m.domains_created());
		if (!ok) return;
		await refresh();
		if (formParent) expanded[formParent] = true;
		select(createdId);
	}

	async function submitEdit() {
		if (!selected) return;
		const id = selected.id;
		if (
			await run(
				() => updateDomain(id, { name: formName, description: formDescription }),
				m.domains_updated()
			)
		) {
			mode = 'none';
			await refresh();
		}
	}

	async function submitMove() {
		if (!selected) return;
		const id = selected.id;
		if (await run(() => moveDomain(id, formParent || null), m.domains_moved())) {
			mode = 'none';
			await refresh();
		}
	}

	async function submitDelete() {
		if (!selected) return;
		const { id, parent_id } = selected;
		if (await run(() => deleteDomain(id), m.domains_deleted())) {
			await refresh();
			select(parent_id ?? null);
		}
	}
</script>

{#snippet node(domain: DomainNode, level: number)}
	<li>
		<div
			class="group flex items-center gap-1 rounded-md pr-2 text-sm {selectedId === domain.id
				? 'bg-earthy-terracotta-50 text-earthy-terracotta-800 dark:bg-earthy-terracotta-900/30 dark:text-earthy-terracotta-200'
				: 'text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800'}"
			style="padding-left: {level * 1}rem"
		>
			{#if domain.children.length > 0}
				<button
					type="button"
					class="rounded p-1 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
					aria-label={m.domains_toggle_children({ name: domainName(domain) })}
					aria-expanded={!!expanded[domain.id]}
					onclick={() => (expanded[domain.id] = !expanded[domain.id])}
				>
					<Icon
						icon="material-symbols:chevron-right-rounded"
						class="h-4 w-4 transition-transform {expanded[domain.id] ? 'rotate-90' : ''}"
					/>
				</button>
			{:else}
				<span class="w-6"></span>
			{/if}
			<button
				type="button"
				class="flex min-w-0 flex-1 items-center gap-2 py-1.5 text-left"
				onclick={() => select(domain.id)}
			>
				<Icon
					icon={isUnassigned(domain)
						? 'material-symbols:inbox-outline-rounded'
						: 'material-symbols:account-tree-outline-rounded'}
					class="h-4 w-4 flex-shrink-0 text-gray-400"
				/>
				<span class="truncate {isUnassigned(domain) ? 'italic' : ''}">{domainName(domain)}</span>
			</button>
		</div>
		{#if domain.children.length > 0 && expanded[domain.id]}
			<ul>
				{#each domain.children as child (child.id)}
					{@render node(child, level + 1)}
				{/each}
			</ul>
		{/if}
	</li>
{/snippet}

<div class="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
	<div class="mb-6 flex flex-wrap items-center justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">{m.domains_title()}</h1>
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{m.domains_subtitle()}</p>
		</div>
		{#if canManage}
			<Button
				icon="material-symbols:add"
				text={m.domains_new_root()}
				click={() => startCreate('')}
			/>
		{/if}
	</div>

	{#if loading}
		<div class="flex justify-center py-16">
			<div class="h-8 w-8 animate-spin rounded-full border-b-2 border-earthy-terracotta-700"></div>
		</div>
	{:else if loadFailed}
		<p class="py-16 text-center text-sm text-red-600 dark:text-red-400">
			{m.domains_load_failed()}
		</p>
	{:else}
		<div class="grid gap-6 lg:grid-cols-[18rem_1fr]">
			<nav
				class="rounded-lg border border-gray-200 bg-white p-2 dark:border-gray-700 dark:bg-gray-800"
				aria-label={m.domains_title()}
			>
				{#if regularRoots.length === 0}
					<p class="px-2 py-3 text-sm text-gray-500 dark:text-gray-400">{m.domains_empty()}</p>
				{/if}
				<ul>
					{#each regularRoots as root (root.id)}
						{@render node(root, 0)}
					{/each}
				</ul>
				{#if unassignedRoot}
					<ul class="mt-2 border-t border-gray-200 pt-2 dark:border-gray-700">
						{@render node(unassignedRoot, 0)}
					</ul>
				{/if}
			</nav>

			<section
				class="rounded-lg border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800"
			>
				{#if mode === 'create'}
					<form
						class="space-y-4"
						onsubmit={(e) => {
							e.preventDefault();
							submitCreate();
						}}
					>
						<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
							{formParent ? m.domains_new_child() : m.domains_new_root()}
						</h2>
						{@render fields()}
						{@render parentPicker(createParents)}
						{@render actions(m.domains_create())}
					</form>
				{:else if !selected}
					<p class="text-sm text-gray-500 dark:text-gray-400">{m.domains_select_hint()}</p>
				{:else if mode === 'edit'}
					<form
						class="space-y-4"
						onsubmit={(e) => {
							e.preventDefault();
							submitEdit();
						}}
					>
						<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
							{m.domains_edit()}
						</h2>
						{@render fields()}
						{@render actions(m.domains_save())}
					</form>
				{:else if mode === 'move'}
					<form
						class="space-y-4"
						onsubmit={(e) => {
							e.preventDefault();
							submitMove();
						}}
					>
						<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
							{m.domains_move()} · {domainName(selected)}
						</h2>
						{@render parentPicker(moveTargets)}
						{@render actions(m.domains_move())}
					</form>
				{:else}
					<div class="space-y-5">
						{#if ancestors.length > 0}
							<nav
								class="text-xs text-gray-500 dark:text-gray-400"
								aria-label={m.domains_location()}
							>
								{#each ancestors as a (a.id)}
									<button type="button" class="hover:underline" onclick={() => select(a.id)}
										>{domainName(a)}</button
									>
									<span class="mx-1">/</span>
								{/each}
							</nav>
						{/if}
						<div class="flex flex-wrap items-start justify-between gap-4">
							<div class="min-w-0">
								<h2 class="text-xl font-semibold text-gray-900 dark:text-gray-100">
									{domainName(selected)}
								</h2>
								{#if selected.description}
									<p class="mt-2 whitespace-pre-wrap text-sm text-gray-600 dark:text-gray-400">
										{selected.description}
									</p>
								{/if}
								{#if isUnassigned(selected)}
									<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
										{m.domains_unassigned_hint()}
									</p>
								{/if}
							</div>
							<Button
								variant="clear"
								icon="material-symbols:manage-search-rounded"
								text={m.domains_view_contents()}
								href={resolve(`/discover?q=${encodeURIComponent(discoverQuery(selected.id))}`)}
							/>
						</div>

						{#if caps && (caps.admin || caps.parentAdmin) && !isUnassigned(selected)}
							<div class="flex flex-wrap gap-2 border-t border-gray-200 pt-4 dark:border-gray-700">
								{#if caps.admin}
									<Button
										variant="clear"
										icon="material-symbols:add"
										text={m.domains_new_child()}
										click={() => selected && startCreate(selected.id)}
									/>
									<Button
										variant="clear"
										icon="material-symbols:edit-outline"
										text={m.domains_edit()}
										click={startEdit}
									/>
								{/if}
								{#if caps.parentAdmin}
									<Button
										variant="clear"
										icon="material-symbols:drive-file-move-outline"
										text={m.domains_move()}
										click={startMove}
									/>
									<Button
										variant="clear"
										icon="material-symbols:delete-outline"
										text={m.domains_delete()}
										click={() => (mode = 'delete')}
									/>
								{/if}
							</div>
						{/if}

						{#if mode === 'delete'}
							<div
								class="rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
								role="alertdialog"
							>
								<p>{m.domains_delete_confirm({ name: domainName(selected) })}</p>
								<div class="mt-3 flex gap-2">
									<Button text={m.domains_delete()} loading={busy} click={submitDelete} />
									<Button variant="clear" text={m.domains_cancel()} click={() => (mode = 'none')} />
								</div>
							</div>
						{/if}

						<DomainRoles domainId={selected.id} canAdmin={caps?.admin ?? false} />
					</div>
				{/if}
			</section>
		</div>
	{/if}
</div>

{#snippet fields()}
	<div>
		<label for="domain-name" class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
			>{m.domains_name()}</label
		>
		<input id="domain-name" bind:value={formName} required maxlength="255" class={inputClass} />
	</div>
	<div>
		<label
			for="domain-description"
			class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
			>{m.domains_description()}</label
		>
		<textarea id="domain-description" bind:value={formDescription} rows="3" class={inputClass}
		></textarea>
	</div>
{/snippet}

{#snippet parentPicker(options: DomainOption[])}
	<div>
		<span class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
			>{m.domains_parent()}</span
		>
		<DomainPicker
			bind:value={formParent}
			{options}
			label={m.domains_parent()}
			noneLabel={m.domains_root_option()}
		/>
	</div>
{/snippet}

{#snippet actions(submitLabel: string)}
	<div class="flex gap-2">
		<Button type="submit" text={submitLabel} loading={busy} disabled={busy} />
		<Button variant="clear" text={m.domains_cancel()} click={() => (mode = 'none')} />
	</div>
{/snippet}
