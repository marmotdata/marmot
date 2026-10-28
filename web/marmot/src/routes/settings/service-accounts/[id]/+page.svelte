<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import RoleSelector from '$lib/components/RoleSelector.svelte';
	import CreateAPIKeyBlade from '$components/serviceaccount/CreateAPIKeyBlade.svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import Panel from '$components/ui/Panel.svelte';
	import Pill from '$components/ui/Pill.svelte';
	import Alert from '$components/ui/Alert.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { BTN_GHOST, BTN_PRIMARY, BTN_SECONDARY, INPUT, LABEL } from '$lib/ui/classes';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import {
		getServiceAccount,
		updateServiceAccount,
		deleteServiceAccount,
		listAPIKeys,
		deleteAPIKey
	} from '$lib/serviceaccounts/api';
	import { listRoles } from '$lib/roles/api';
	import type { ServiceAccount, ServiceAccountAPIKey } from '$lib/serviceaccounts/types';
	import type { Role } from '$lib/roles/types';

	const MAX_KEYS = 5;

	let sa = $state<ServiceAccount | null>(null);
	let keys = $state<ServiceAccountAPIKey[]>([]);
	let availableRoles = $state<Role[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let savingDetails = $state(false);
	let showDeleteModal = $state(false);
	let keyToDelete = $state<ServiceAccountAPIKey | null>(null);
	let showDeleteKeyModal = $state(false);

	let name = $state('');
	let description = $state('');
	let active = $state(true);
	let selectedRoleIds = $state<string[]>([]);

	let showCreateKey = $state(false);
	let keyJustCreated = $state(false);
	let showNewKeyBanner = $derived(keyJustCreated || page.url.searchParams.get('created') === '1');

	// The old /api-keys/new route redirects here with ?create=1, so open the blade once the account has loaded and drop the flag so a refresh does not reopen it
	$effect(() => {
		if (sa && page.url.searchParams.get('create') === '1') {
			showCreateKey = true;
			goto(resolve('/settings/service-accounts/[id]', { id: sa.id }), {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
		}
	});

	$effect(() => {
		if (page.params.id) {
			load(page.params.id);
		}
	});

	async function load(id: string) {
		try {
			loading = true;
			error = null;
			const [account, keyList, roles] = await Promise.all([
				getServiceAccount(id),
				listAPIKeys(id).catch(() => []),
				listRoles().catch(() => [])
			]);
			sa = account;
			keys = keyList;
			availableRoles = roles;
			name = account.name;
			description = account.description ?? '';
			active = account.active;
			selectedRoleIds = account.roles.map((r) => r.id);
		} catch (err) {
			error = err instanceof Error ? err.message : m.serviceaccounts_error_load();
		} finally {
			loading = false;
		}
	}

	async function saveDetails() {
		if (!sa) return;
		try {
			savingDetails = true;
			const updated = await updateServiceAccount(sa.id, {
				name: name.trim(),
				description: description.trim() || undefined,
				active,
				role_ids: selectedRoleIds
			});
			sa = updated;
			toasts.success(m.serviceaccounts_updated_success());
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.serviceaccounts_error_update());
		} finally {
			savingDetails = false;
		}
	}

	async function handleDelete() {
		if (!sa) return;
		try {
			await deleteServiceAccount(sa.id);
			toasts.success(m.serviceaccounts_deleted_success({ name: sa.name }));
			goto(resolve('/settings/service-accounts'));
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.serviceaccounts_error_delete());
		} finally {
			showDeleteModal = false;
		}
	}

	async function handleDeleteKey() {
		if (!sa || !keyToDelete) return;
		try {
			await deleteAPIKey(sa.id, keyToDelete.id);
			toasts.success(m.serviceaccounts_key_deleted_success({ name: keyToDelete.name }));
			keys = keys.filter((k) => k.id !== keyToDelete?.id);
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.serviceaccounts_error_delete_key());
		} finally {
			showDeleteKeyModal = false;
			keyToDelete = null;
		}
	}

	async function handleKeyDone() {
		showCreateKey = false;
		keyJustCreated = true;
		if (!sa) return;
		keys = await listAPIKeys(sa.id).catch(() => keys);
	}

	let canManage = $derived(holdsAtRoot($myAccess, 'service_accounts:manage'));

	// The most recent use across every key, which is when this identity last acted
	let lastUsed = $derived(
		keys
			.map((k) => k.last_used_at)
			.filter((d): d is string => !!d)
			.sort()
			.at(-1)
	);

	function formatDate(iso: string | undefined): string {
		if (!iso) return '';
		return iso.slice(0, 10);
	}

	function daysUntil(iso: string | undefined): number | null {
		if (!iso) return null;
		const target = new Date(iso).getTime();
		if (Number.isNaN(target)) return null;
		return Math.ceil((target - Date.now()) / (1000 * 60 * 60 * 24));
	}

	function keyStatus(key: ServiceAccountAPIKey): 'expired' | 'expiring' | 'unused' | 'ok' {
		const days = daysUntil(key.expires_at);
		if (days !== null && days <= 0) return 'expired';
		if (days !== null && days <= 14) return 'expiring';
		if (!key.last_used_at) return 'unused';
		return 'ok';
	}
</script>

<svelte:head>
	<title
		>{sa
			? m.serviceaccounts_page_title({ name: sa.name })
			: m.serviceaccounts_page_title_fallback()}</title
	>
</svelte:head>

{#if loading}
	<Skeleton class="h-16" />
	<div class="mt-6 grid gap-6 lg:grid-cols-3">
		<div class="lg:col-span-2"><Skeleton class="h-48" rows={2} /></div>
		<Skeleton class="h-64" />
	</div>
{:else if error}
	<PageHeader
		title={m.serviceaccounts_detail_title()}
		backHref={resolve('/settings/service-accounts')}
		backLabel={m.settings_service_accounts()}
	/>
	<Alert tone="error">{error}</Alert>
{:else if sa}
	<PageHeader
		title={sa.name}
		description={sa.description || undefined}
		backHref={resolve('/settings/service-accounts')}
		backLabel={m.settings_service_accounts()}
	>
		{#snippet meta()}
			<Pill tone={sa?.active ? 'success' : 'warning'}>
				{sa?.active ? m.common_active() : m.common_inactive()}
			</Pill>
			<span class="text-xs tabular-nums text-gray-500 dark:text-gray-400"
				>{m.serviceaccounts_meta_keys({ count: keys.length })}</span
			>
			<span class="text-xs text-gray-400 dark:text-gray-500" aria-hidden="true">·</span>
			<span class="text-xs text-gray-500 dark:text-gray-400">
				{lastUsed
					? m.serviceaccounts_meta_last_used({ date: formatDate(lastUsed) })
					: m.serviceaccounts_meta_never_used()}
			</span>
		{/snippet}
		{#snippet actions()}
			{#if canManage}
				<button type="button" class={BTN_SECONDARY} onclick={() => (showDeleteModal = true)}>
					<IconifyIcon icon="material-symbols:delete-outline" class="h-4 w-4" />
					{m.common_delete()}
				</button>
				<button
					type="button"
					class={BTN_PRIMARY}
					onclick={() => (showCreateKey = true)}
					disabled={keys.length >= MAX_KEYS}
					title={keys.length >= MAX_KEYS
						? m.serviceaccounts_detail_key_limit_reached({ max: MAX_KEYS })
						: undefined}
				>
					<IconifyIcon icon="material-symbols:key-outline" class="h-5 w-5" />
					{m.serviceaccounts_detail_create_key_button()}
				</button>
			{/if}
		{/snippet}
	</PageHeader>

	{#if showNewKeyBanner}
		<div class="mb-6 flex items-start gap-2">
			<div class="min-w-0 flex-1">
				<Alert tone="success" title={m.serviceaccounts_key_created_banner()}>
					{m.serviceaccounts_key_saved_reminder()}
				</Alert>
			</div>
			<button
				type="button"
				class="{BTN_GHOST} mt-2"
				onclick={() => {
					keyJustCreated = false;
					goto(resolve('/settings/service-accounts/[id]', { id: sa!.id }));
				}}
				aria-label={m.serviceaccounts_dismiss_aria()}
			>
				<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
			</button>
		</div>
	{/if}

	<div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
		<div class="space-y-6 lg:col-span-2">
			<Panel title={m.common_details()} icon="material-symbols:info-outline">
				<div class="space-y-4">
					<div>
						<label for="detail-name" class="{LABEL} mb-1.5 block">{m.common_name()}</label>
						<input id="detail-name" type="text" bind:value={name} class={INPUT} />
					</div>
					<div>
						<label for="detail-desc" class="{LABEL} mb-1.5 block">{m.common_description()}</label>
						<textarea id="detail-desc" bind:value={description} rows="2" class="{INPUT} resize-y"
						></textarea>
					</div>
					<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
						<input
							type="checkbox"
							bind:checked={active}
							class="rounded border-gray-300 text-earthy-terracotta-600 focus:ring-earthy-terracotta-500 dark:border-gray-600"
						/>
						{m.common_active()}
						<span class="text-xs text-gray-500 dark:text-gray-400"
							>{m.serviceaccounts_active_hint()}</span
						>
					</label>
				</div>
			</Panel>

			<Panel
				title={m.serviceaccounts_org_roles_heading()}
				description={m.serviceaccounts_org_roles_hint()}
				icon="material-symbols:shield-outline"
			>
				<RoleSelector
					roles={availableRoles}
					selectedIds={selectedRoleIds}
					onChange={(ids) => (selectedRoleIds = ids)}
					emptyMessage={m.serviceaccounts_no_roles_available()}
				/>
			</Panel>

			{#if canManage}
				<div class="flex justify-end">
					<button
						type="button"
						class={BTN_PRIMARY}
						onclick={saveDetails}
						disabled={savingDetails || !name.trim()}
					>
						<IconifyIcon icon="material-symbols:check" class="h-4 w-4" />
						{savingDetails ? m.serviceaccounts_saving_label() : m.serviceaccounts_save_changes()}
					</button>
				</div>
			{/if}
		</div>

		<!-- API keys -->
		<div class="lg:col-span-1">
			<Panel
				flush
				title={m.serviceaccounts_detail_api_keys_heading()}
				icon="material-symbols:key-outline"
			>
				{#snippet action()}
					<!-- The count is spelled out: "1 / 5" on its own made the reader guess what was being counted. -->
					<span class="text-xs tabular-nums text-gray-500 dark:text-gray-400">
						{m.serviceaccounts_key_count({ count: keys.length, max: MAX_KEYS })}
					</span>
				{/snippet}

				{#if keys.length === 0}
					<div class="p-5">
						<EmptyState
							icon="material-symbols:key-off-outline"
							title={m.serviceaccounts_no_keys()}
							description={m.serviceaccounts_detail_no_keys_hint()}
							dashed={false}
						>
							{#snippet action()}
								{#if canManage}
									<button
										type="button"
										class={BTN_SECONDARY}
										onclick={() => (showCreateKey = true)}
									>
										<IconifyIcon icon="material-symbols:add" class="h-4 w-4" />
										{m.serviceaccounts_detail_create_key_button()}
									</button>
								{/if}
							{/snippet}
						</EmptyState>
					</div>
				{:else}
					<ul class="divide-y divide-gray-100 dark:divide-gray-700/60">
						{#each keys as key (key.id)}
							{@const status = keyStatus(key)}
							{@const daysLeft = daysUntil(key.expires_at)}
							<li class="group flex items-start gap-3 px-5 py-3">
								<IconifyIcon
									icon="material-symbols:vpn-key-outline"
									class="mt-0.5 h-4 w-4 shrink-0 text-gray-400"
								/>
								<div class="min-w-0 flex-1">
									<div class="flex flex-wrap items-center gap-1.5">
										<span class="truncate text-sm font-medium text-gray-900 dark:text-gray-100"
											>{key.name}</span
										>
										<!-- A pill only when the state is worth acting on. -->
										{#if status === 'expired'}
											<Pill tone="danger">{m.serviceaccounts_key_expired()}</Pill>
										{:else if status === 'expiring' && daysLeft !== null}
											<Pill tone="warning"
												>{m.serviceaccounts_detail_key_expires_in_days({ days: daysLeft })}</Pill
											>
										{/if}
									</div>
									<p class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">
										{#if key.expires_at}{m.serviceaccounts_key_expires_date({
												date: formatDate(key.expires_at)
											})}{:else}{m.serviceaccounts_key_never_expires()}{/if}
										· {#if key.last_used_at}{m.serviceaccounts_key_used_date({
												date: formatDate(key.last_used_at)
											})}{:else}{m.serviceaccounts_key_unused()}{/if}
									</p>
								</div>
								{#if canManage}
									<button
										type="button"
										class="{BTN_GHOST} -mr-2 px-1.5 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
										onclick={() => {
											keyToDelete = key;
											showDeleteKeyModal = true;
										}}
										aria-label={m.serviceaccounts_delete_key_aria()}
										title={m.serviceaccounts_delete_key_aria()}
									>
										<IconifyIcon icon="material-symbols:delete-outline" class="h-4 w-4" />
									</button>
								{/if}
							</li>
						{/each}
					</ul>
					{#if keys.length >= MAX_KEYS}
						<p
							class="border-t border-gray-100 px-5 py-3 text-xs text-gray-500 dark:border-gray-700/60 dark:text-gray-400"
						>
							{m.serviceaccounts_detail_key_limit_reached({ max: MAX_KEYS })}
						</p>
					{/if}
				{/if}
			</Panel>
		</div>
	</div>
{/if}

{#if sa}
	<DeleteModal
		show={showDeleteModal}
		title={m.serviceaccounts_delete_title()}
		message={keys.length > 0
			? m.serviceaccounts_delete_revokes({ count: keys.length })
			: m.serviceaccounts_delete_confirm()}
		confirmText={m.common_delete()}
		resourceName={sa.name}
		requireConfirmation={true}
		onConfirm={handleDelete}
		onCancel={() => (showDeleteModal = false)}
	/>
{/if}

{#if keyToDelete}
	<DeleteModal
		show={showDeleteKeyModal}
		title={m.serviceaccounts_delete_key_title()}
		message={m.serviceaccounts_delete_key_confirm()}
		confirmText={m.common_delete()}
		resourceName={keyToDelete.name}
		requireConfirmation={false}
		onConfirm={handleDeleteKey}
		onCancel={() => {
			showDeleteKeyModal = false;
			keyToDelete = null;
		}}
	/>
{/if}

{#if sa && showCreateKey}
	<CreateAPIKeyBlade {sa} onClose={() => (showCreateKey = false)} onDone={handleKeyDone} />
{/if}
