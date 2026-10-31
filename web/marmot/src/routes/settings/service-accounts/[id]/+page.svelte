<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { html, nothing } from 'lit';
	import type { DataTableColumn } from '@marmotdata/elements/classes/data-table.js';
	import { MmConfirmDialog } from '@marmotdata/elements/classes/confirm-dialog.js';
	import { setCrumbs } from '$lib/mm';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { formatDate, formatRelativeTime } from '$lib/utils';
	import { roleLabel } from '$lib/users/format';
	import {
		getServiceAccount,
		deleteServiceAccount,
		listAPIKeys,
		deleteAPIKey,
		rotateAPIKey
	} from '$lib/serviceaccounts/api';
	import { assetHref } from '$lib/assetrules/assets';
	import type { ServiceAccount, ServiceAccountAPIKey } from '$lib/serviceaccounts/types';
	import CreateApiKeyDrawer from '$components/serviceaccount/CreateApiKeyDrawer.svelte';
	import EditServiceAccountDrawer from '$components/serviceaccount/EditServiceAccountDrawer.svelte';

	// The server refuses a sixth key, so the button is disabled before people fill in the form
	const MAX_KEYS = 5;
	// Keys this close to expiry are flagged so they can be replaced before anything breaks
	const EXPIRING_DAYS = 14;

	let sa = $state<ServiceAccount | null>(null);
	let keys = $state<ServiceAccountAPIKey[]>([]);
	let loading = $state(true);
	let error = $state('');
	let creatingKey = $state(false);
	let rotatedKey = $state<ServiceAccountAPIKey | null>(null);
	let editing = $state(false);

	let canManage = $derived(holdsAtRoot($myAccess, 'service_accounts:manage'));

	$effect(() => {
		setCrumbs([
			{ label: m.settings_title(), href: resolve('/settings') },
			{ label: m.settings_service_accounts(), href: resolve('/settings/service-accounts') },
			{ label: sa?.name ?? m.serviceaccounts_detail_crumb() }
		]);
	});

	$effect(() => {
		if (page.params.id) load(page.params.id);
	});

	// The old /api-keys/new route redirects here with ?create=1, so open the drawer once the account has loaded and drop the flag so a refresh does not reopen it
	$effect(() => {
		if (sa && page.url.searchParams.get('create') === '1') {
			creatingKey = true;
			goto(resolve('/settings/service-accounts/[id]', { id: sa.id }), {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
		}
	});

	async function load(id: string) {
		loading = true;
		error = '';
		try {
			const [account, keyList] = await Promise.all([
				getServiceAccount(id),
				listAPIKeys(id).catch(() => [])
			]);
			sa = account;
			keys = keyList;
		} catch (err) {
			error = err instanceof Error ? err.message : m.serviceaccounts_error_load();
		} finally {
			loading = false;
		}
	}

	// The most recent use across every key, which is when this identity last acted; the server works it out, and the key list covers an older server
	let lastUsed = $derived(
		sa?.last_used_at ??
			keys
				.map((k) => k.last_used_at)
				.filter((d): d is string => !!d)
				.sort()
				.at(-1)
	);
	const callsFormat = new Intl.NumberFormat();

	function daysUntil(iso: string | undefined): number | null {
		if (!iso) return null;
		const target = new Date(iso).getTime();
		if (Number.isNaN(target)) return null;
		return Math.ceil((target - Date.now()) / (1000 * 60 * 60 * 24));
	}

	async function removeAccount() {
		if (!sa) return;
		const ok = await MmConfirmDialog.confirm({
			heading: m.serviceaccounts_delete_heading({ name: sa.name }),
			body: keys.length
				? m.serviceaccounts_delete_revokes({ count: keys.length })
				: m.serviceaccounts_delete_body(),
			confirmLabel: m.serviceaccounts_delete_button(),
			confirmText: sa.name
		});
		if (!ok) return;
		try {
			await deleteServiceAccount(sa.id);
			toasts.success(m.serviceaccounts_deleted_success({ name: sa.name }));
			goto(resolve('/settings/service-accounts'));
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.serviceaccounts_error_delete());
		}
	}

	async function revokeKey(key: ServiceAccountAPIKey) {
		if (!sa) return;
		const ok = await MmConfirmDialog.confirm({
			heading: m.serviceaccounts_revoke_heading({ name: key.name }),
			body: m.serviceaccounts_delete_key_confirm(),
			confirmLabel: m.serviceaccounts_revoke_key()
		});
		if (!ok) return;
		try {
			await deleteAPIKey(sa.id, key.id);
			toasts.success(m.serviceaccounts_key_revoked({ name: key.name }));
			keys = keys.filter((k) => k.id !== key.id);
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.serviceaccounts_error_delete_key());
		}
	}

	async function rotateKey(key: ServiceAccountAPIKey) {
		if (!sa) return;
		const ok = await MmConfirmDialog.confirm({
			heading: m.apikeys_rotate_title(),
			body: m.apikeys_rotate_message(),
			confirmLabel: m.apikeys_rotate_button(),
			confirmIcon: 'sync'
		});
		if (!ok) return;
		try {
			const result = await rotateAPIKey(sa.id, key.id);
			keys = keys.map((k) => (k.id === key.id ? { ...result, key: undefined } : k));
			rotatedKey = result;
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.apikeys_rotate_error());
		}
	}

	async function onKeyCreated() {
		creatingKey = false;
		rotatedKey = null;
		if (sa) keys = await listAPIKeys(sa.id).catch(() => keys);
	}

	function keyState(key: ServiceAccountAPIKey) {
		const days = daysUntil(key.expires_at);
		if (days !== null && days <= 0)
			return html`<mm-status-pill status="danger" icon="error"
				>${m.serviceaccounts_key_expired()}</mm-status-pill
			>`;
		if (days !== null && days <= EXPIRING_DAYS)
			return html`<mm-status-pill status="warning" icon="schedule"
				>${m.serviceaccounts_detail_key_expires_in_days({ days })}</mm-status-pill
			>`;
		return m.common_active();
	}

	let keyColumns = $derived<DataTableColumn<ServiceAccountAPIKey>[]>([
		{ key: 'name', header: m.common_name(), width: 220 },
		{
			key: 'prefix',
			header: m.apikeys_column_key(),
			type: 'mono',
			width: 160,
			empty: m.apikeys_prefix_unknown()
		},
		{ key: 'created_at', header: m.serviceaccounts_col_created(), type: 'date', width: 130 },
		{
			key: 'last_used_at',
			header: m.serviceaccounts_col_last_used(),
			type: 'date',
			width: 130,
			empty: m.common_never()
		},
		{
			key: 'expires_at',
			header: m.serviceaccounts_col_expires(),
			width: 140,
			value: (k) => (k.expires_at ? formatDate(k.expires_at) : m.common_never())
		},
		{
			key: 'state',
			header: m.common_status(),
			width: 170,
			minWidth: 160,
			render: keyState
		},
		{
			key: 'actions',
			header: m.common_actions(),
			width: 180,
			minWidth: 180,
			align: 'end',
			// One visible button per row: the action the row needs most, with the rest in More
			render: (k) => {
				if (!canManage) return nothing;
				const days = daysUntil(k.expires_at);
				if (days === null || days > EXPIRING_DAYS)
					return html`<mm-button
						size="sm"
						variant="ghost"
						icon="delete"
						label=${m.serviceaccounts_revoke_key_aria({ name: k.name })}
						@click=${() => revokeKey(k)}
						>${m.serviceaccounts_revoke()}</mm-button
					>`;
				return html`<span
					class="mm-cluster"
					style="--mm-gap:var(--space-1);justify-content:flex-end;flex-wrap:nowrap"
				>
					<mm-button size="sm" icon="sync" @click=${() => rotateKey(k)}
						>${m.apikeys_rotate_button()}</mm-button
					>
					<mm-menu-button
						size="sm"
						variant="ghost"
						icon="more-horiz"
						icon-only
						label=${m.apikeys_more_actions({ name: k.name })}
					>
						<mm-menu
							label=${m.apikeys_actions_for({ name: k.name })}
							@mm-select=${() => revokeKey(k)}
						>
							<mm-menu-item icon="delete" value="revoke" danger
								>${m.serviceaccounts_revoke_key()}</mm-menu-item
							>
						</mm-menu>
					</mm-menu-button>
				</span>`;
			}
		}
	]);
</script>

<svelte:head>
	<title>{sa ? sa.name : m.serviceaccounts_detail_crumb()} · {m.settings_service_accounts()}</title>
</svelte:head>

{#if loading && !sa}
	<div class="mm-stack" style="--mm-gap: var(--space-5)" aria-busy="true">
		<mm-skeleton shape="text" count={1} style="width: 16rem"></mm-skeleton>
		<mm-skeleton shape="table-row" count={5}></mm-skeleton>
	</div>
{:else if error || !sa}
	<div class="mm-stack" style="--mm-gap: var(--space-5)">
		<mm-page-header heading={m.serviceaccounts_detail_crumb()}></mm-page-header>
		<mm-callout tone="danger" heading={m.serviceaccounts_error_load()}>
			{error}
			<mm-button
				slot="actions"
				size="sm"
				icon="refresh"
				onclick={() => page.params.id && load(page.params.id)}
			>
				{m.common_retry()}
			</mm-button>
		</mm-callout>
	</div>
{:else}
	<div class="mm-stack" style="--mm-gap: var(--space-6)">
		<mm-page-header
			heading={sa.name}
			icon="smart-toy"
			caption={lastUsed
				? m.serviceaccounts_meta_last_used({ date: formatRelativeTime(lastUsed) })
				: m.serviceaccounts_meta_never_used()}
			description={sa.description ?? ''}
		>
			{#if sa.used_by?.length}
				<span slot="owners" class="mm-cluster"
					><span class="mm-caption">{m.serviceaccounts_col_used_by()}</span>
					{#each sa.used_by as agent (agent.id)}
						<mm-owner-chip
							name={agent.name}
							kind="agent"
							href={assetHref(agent)}
							service-account={sa.name}
						></mm-owner-chip>
					{/each}
				</span>
			{/if}
			{#if canManage}
				<mm-menu-button
					slot="actions"
					variant="ghost"
					icon="more-horiz"
					icon-only
					label={m.serviceaccounts_more_actions({ name: sa.name })}
				>
					<mm-menu onmm-select={removeAccount}>
						<mm-menu-item icon="delete" danger>
							{m.serviceaccounts_delete_named({ name: sa.name })}
						</mm-menu-item>
					</mm-menu>
				</mm-menu-button>
				<mm-button slot="actions" icon="edit" onclick={() => (editing = true)}>
					{m.serviceaccounts_edit_button()}
				</mm-button>
			{/if}
		</mm-page-header>

		<mm-description-list layout="inline-grid" columns={4}>
			<mm-description term={m.users_header_roles()}>
				{sa.roles.length
					? sa.roles.map((r) => roleLabel(r.name)).join(', ')
					: m.serviceaccounts_no_roles()}
			</mm-description>
			{#if !sa.active}
				<mm-description term={m.common_status()}>
					{m.serviceaccounts_status_inactive()}
				</mm-description>
			{/if}
			<mm-description term={m.serviceaccounts_col_created()}
				>{formatDate(sa.created_at)}</mm-description
			>
			{#if sa.created_by_name}
				<mm-description term={m.serviceaccounts_created_by()}>{sa.created_by_name}</mm-description>
			{/if}
			<mm-description term={m.serviceaccounts_calls_7_days()}
				>{callsFormat.format(sa.calls_last_7_days ?? 0)}</mm-description
			>
		</mm-description-list>

		<mm-section
			heading={m.serviceaccounts_detail_api_keys_heading()}
			description={keys.length >= MAX_KEYS
				? m.serviceaccounts_detail_key_limit_reached({ max: MAX_KEYS })
				: m.serviceaccounts_keys_description()}
		>
			{#if canManage}
				<mm-button
					slot="actions"
					variant="primary"
					icon="key"
					disabled={keys.length >= MAX_KEYS}
					onclick={() => (creatingKey = true)}
				>
					{m.serviceaccounts_detail_create_key_button()}
				</mm-button>
			{/if}
			<mm-data-table
				label={m.serviceaccounts_keys_for({ name: sa.name })}
				columns={keyColumns}
				rows={keys}
				empty-heading={m.serviceaccounts_no_keys()}
				empty-text={m.serviceaccounts_detail_no_keys_hint()}
			></mm-data-table>
		</mm-section>
	</div>
{/if}

{#if sa && creatingKey}
	<CreateApiKeyDrawer {sa} onClose={() => (creatingKey = false)} onDone={onKeyCreated} />
{/if}

{#if sa && rotatedKey}
	<CreateApiKeyDrawer
		{sa}
		rotated={rotatedKey}
		onClose={() => (rotatedKey = null)}
		onDone={() => (rotatedKey = null)}
	/>
{/if}

{#if sa && editing}
	<EditServiceAccountDrawer
		{sa}
		onClose={() => (editing = false)}
		onSaved={(updated) => {
			sa = updated;
			editing = false;
		}}
	/>
{/if}
