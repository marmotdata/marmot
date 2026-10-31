<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { DataTableColumn } from '@marmotdata/elements/classes/data-table.js';
	import type { MmValueDetail } from '@marmotdata/elements/classes/form-field.js';
	import type { MmPageChangeDetail } from '@marmotdata/elements/classes/pagination.js';
	import { MmConfirmDialog } from '@marmotdata/elements/classes/confirm-dialog.js';
	import { setCrumbs } from '$lib/mm';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { listRoles } from '$lib/roles/api';
	import { m } from '$lib/paraglide/messages';
	import { html, nothing } from 'lit';
	import { rowMenu } from '$lib/users/cells';
	import { roleLabel, signInMethods } from '$lib/users/format';
	import { renewInvite, withdrawInvite } from '$lib/users/invites';
	import type { Invitation, MemberStatus, User } from '$lib/users/types';
	import type { Role } from '$lib/roles/types';
	import AddMemberDrawer from '$components/user/AddMemberDrawer.svelte';
	import EditMemberDrawer from '$components/user/EditMemberDrawer.svelte';
	import InviteLinksDrawer from '$components/user/InviteLinksDrawer.svelte';

	// The server filters and pages members, so the list stays quick with thousands of people
	const PAGE_SIZE = 50;

	let canManage = $derived(holdsAtRoot($myAccess, 'users:manage'));
	const currentUserId = auth.getCurrentUserId();

	let users = $state<User[]>([]);
	let total = $state(0);
	let roles = $state<Role[]>([]);
	let query = $state('');
	let roleFilter = $state('all');
	let statusFilter = $state<'all' | MemberStatus>('all');
	// The Teams page links here with ?team=none to show who belongs to no team
	let teamFilter = $state<'all' | 'none'>(
		page.url.searchParams.get('team') === 'none' ? 'none' : 'all'
	);
	let links = $state<Invitation[] | null>(null);
	let pageNumber = $state(1);
	let loading = $state(true);
	let error = $state('');
	let adding = $state(false);
	let editing = $state<User | null>(null);
	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	let seq = 0;

	let filtered = $derived(
		!!query.trim() || roleFilter !== 'all' || statusFilter !== 'all' || teamFilter !== 'all'
	);

	setCrumbs([
		{ label: m.settings_title(), href: resolve('/settings') },
		{ label: m.settings_members() }
	]);

	async function fetchUsers() {
		const mine = ++seq;
		loading = true;
		error = '';
		try {
			const entries: [string, string][] = [
				['limit', String(PAGE_SIZE)],
				['offset', String((pageNumber - 1) * PAGE_SIZE)]
			];
			if (query.trim()) entries.push(['query', query.trim()]);
			if (roleFilter !== 'all') entries.push(['role_ids', roleFilter]);
			if (statusFilter !== 'all') entries.push(['status', statusFilter]);
			if (teamFilter === 'none') entries.push(['team', 'none']);
			const response = await fetchApi(`/users?${new URLSearchParams(entries)}`);
			if (!response.ok) throw new Error(await handleApiError(response));
			const data = await response.json();
			if (mine !== seq) return;
			users = data.users ?? [];
			total = data.total ?? users.length;
		} catch (err) {
			if (mine === seq) error = err instanceof Error ? err.message : m.members_error_load();
		} finally {
			if (mine === seq) loading = false;
		}
	}

	function refilter() {
		pageNumber = 1;
		fetchUsers();
	}

	function onSearch(value: string) {
		query = value;
		clearTimeout(searchTimer);
		searchTimer = setTimeout(refilter, 300);
	}

	async function setActive(user: User, active: boolean) {
		const response = await fetchApi(`/users/${user.id}`, {
			method: 'PUT',
			body: JSON.stringify({ active })
		});
		if (!response.ok) {
			toasts.error(await handleApiError(response));
			return;
		}
		toasts.success(active ? m.members_reactivated_toast() : m.members_deactivated_toast());
		fetchUsers();
	}

	async function deleteUser(user: User) {
		const ok = await MmConfirmDialog.confirm({
			heading: m.members_delete_heading({ name: user.name || user.username }),
			body: m.members_delete_message(),
			confirmLabel: m.members_delete_button(),
			confirmText: user.username
		});
		if (!ok) return;
		try {
			const response = await fetchApi(`/users/${user.id}`, { method: 'DELETE' });
			if (!response.ok) {
				toasts.error(await handleApiError(response));
				return;
			}
			toasts.success(m.users_delete_success({ name: user.username }));
			fetchUsers();
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.users_error_delete());
		}
	}

	async function newInviteLink(user: User) {
		try {
			links = [await renewInvite(user.id)];
			fetchUsers();
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.members_invite_failed());
		}
	}

	async function withdraw(user: User) {
		const ok = await MmConfirmDialog.confirm({
			heading: m.members_withdraw_heading({ name: user.name || user.username }),
			body: m.members_withdraw_message(),
			confirmLabel: m.members_withdraw_invitation()
		});
		if (!ok) return;
		try {
			await withdrawInvite(user.id);
			toasts.success(m.members_withdrawn_toast());
			fetchUsers();
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.members_invite_failed());
		}
	}

	function statusLabel(u: User): string {
		if (u.status === 'invited') return m.members_status_invited();
		return u.active ? m.common_active() : m.members_status_deactivated();
	}

	function onRowAction(user: User, action: string) {
		if (action === 'edit') editing = user;
		else if (action === 'renew-invite') newInviteLink(user);
		else if (action === 'withdraw-invite') withdraw(user);
		else if (action === 'deactivate') setActive(user, false);
		else if (action === 'reactivate') setActive(user, true);
		else if (action === 'delete') deleteUser(user);
	}

	let columns = $derived<DataTableColumn<User>[]>([
		{
			key: 'name',
			header: m.members_col_member(),
			width: 220,
			value: (u) => u.name || u.username,
			render: (u) =>
				html`<mm-owner-chip
						name=${u.name || u.username}
						kind="person"
						src=${u.profile_picture ?? ''}
					></mm-owner-chip
					>${
						u.id === currentUserId
							? html` <span class="mm-muted">${m.members_you_badge()}</span>`
							: nothing
					}`
		},
		{ key: 'email', header: m.members_col_email() },
		{ key: 'username', header: m.members_col_username(), width: 150 },
		{
			key: 'roles',
			header: m.users_header_roles(),
			width: 150,
			value: (u) => u.roles.map((r) => roleLabel(r.name)).join(', ')
		},
		{
			key: 'active',
			header: m.common_status(),
			width: 130,
			value: statusLabel,
			// No pill: none of these states is a problem that needs attention
			render: (u) =>
				u.status === 'deactivated' || (!u.status && !u.active)
					? html`<span class="mm-muted">${m.members_status_deactivated()}</span>`
					: statusLabel(u)
		},
		{
			key: 'signin',
			header: m.members_col_signs_in_with(),
			width: 140,
			value: (u) => signInMethods(u)
		},
		{
			key: 'last_seen_at',
			header: m.members_col_last_seen(),
			type: 'date',
			width: 130,
			empty: '–'
		},
		{
			key: 'actions',
			header: m.common_actions(),
			width: 72,
			minWidth: 72,
			align: 'end',
			render: (u) => {
				if (!canManage || u.id === currentUserId) return nothing;
				const name = u.name || u.username;
				if (u.status === 'invited') {
					return rowMenu(
						m.members_actions_aria({ name }),
						[
							{ value: 'edit', label: m.members_edit_member(), icon: 'edit' },
							{ value: 'renew-invite', label: m.members_new_invite_link(), icon: 'link' },
							{
								value: 'withdraw-invite',
								label: m.members_withdraw_invitation(),
								icon: 'cancel',
								danger: true
							}
						],
						(action) => onRowAction(u, action)
					);
				}
				return rowMenu(
					m.members_actions_aria({ name }),
					[
						{ value: 'edit', label: m.members_edit_member(), icon: 'edit' },
						u.active
							? { value: 'deactivate', label: m.members_deactivate_member(), icon: 'block' }
							: { value: 'reactivate', label: m.members_reactivate_member(), icon: 'check-circle' },
						...(u.username !== 'admin'
							? [
									{
										value: 'delete',
										label: m.members_delete_button(),
										icon: 'delete',
										danger: true
									}
								]
							: [])
					],
					(action) => onRowAction(u, action)
				);
			}
		}
	]);

	onMount(() => {
		fetchUsers();
		listRoles()
			.then((list) => (roles = list))
			.catch(() => (roles = []));
		// The old /users/new route redirects here with ?create=1, so open the drawer and drop the flag so a refresh does not reopen it
		if (page.url.searchParams.get('create') === '1') {
			adding = true;
			goto(resolve('/settings/members'), { replaceState: true, noScroll: true, keepFocus: true });
		}
	});
</script>

<svelte:head>
	<title>{m.settings_members()} · {m.settings_title()}</title>
</svelte:head>

<div class="mm-stack" style="--mm-gap: var(--space-5)">
	<mm-page-header heading={m.settings_members()} compact count={error ? undefined : total}>
		{#if canManage}
			<mm-button slot="actions" variant="primary" icon="person-add" onclick={() => (adding = true)}>
				{m.members_add_button()}
			</mm-button>
		{/if}
	</mm-page-header>

	<div class="mm-cluster" style="align-items: end">
		<mm-search-field
			style="width: 20rem; max-width: 100%"
			label={m.members_search_label()}
			hide-label
			placeholder={m.members_search_placeholder()}
			value={query}
			onmm-input={(e: CustomEvent<MmValueDetail>) => onSearch(e.detail.value)}
		></mm-search-field>
		<mm-select
			style="width: 12rem"
			label={m.members_role_filter_label()}
			hide-label
			value={roleFilter}
			onmm-change={(e: CustomEvent<MmValueDetail>) => {
				roleFilter = e.detail.value;
				refilter();
			}}
		>
			<mm-option value="all" label={m.members_all_roles()}></mm-option>
			{#each roles as role (role.id)}
				<mm-option value={role.id} label={roleLabel(role.name)}></mm-option>
			{/each}
		</mm-select>
		<mm-select
			style="width: 12rem"
			label={m.common_status()}
			hide-label
			value={statusFilter}
			onmm-change={(e: CustomEvent<MmValueDetail>) => {
				statusFilter = e.detail.value as typeof statusFilter;
				refilter();
			}}
		>
			<mm-option value="all" label={m.members_any_status()}></mm-option>
			<mm-option value="active" label={m.common_active()}></mm-option>
			<mm-option value="invited" label={m.members_status_invited()}></mm-option>
			<mm-option value="deactivated" label={m.members_status_deactivated()}></mm-option>
		</mm-select>
		<mm-select
			style="width: 12rem"
			label={m.members_team_filter_label()}
			hide-label
			value={teamFilter}
			onmm-change={(e: CustomEvent<MmValueDetail>) => {
				teamFilter = e.detail.value as typeof teamFilter;
				refilter();
			}}
		>
			<mm-option value="all" label={m.members_any_team()}></mm-option>
			<mm-option value="none" label={m.members_no_team()}></mm-option>
		</mm-select>
	</div>

	<mm-data-table
		label={m.settings_members()}
		{columns}
		rows={users}
		loading={loading && users.length === 0}
		{error}
		empty-heading={filtered ? m.members_no_match_title() : m.members_empty_title()}
		empty-text={filtered ? m.members_no_match_body() : m.members_empty_body()}
		onmm-retry={fetchUsers}
	></mm-data-table>

	{#if total > PAGE_SIZE}
		<mm-pagination
			label={m.members_pagination_label()}
			page={pageNumber}
			page-size={PAGE_SIZE}
			{total}
			disabled={loading}
			onmm-page-change={(e: CustomEvent<MmPageChangeDetail>) => {
				pageNumber = e.detail.page;
				fetchUsers();
			}}
		></mm-pagination>
	{/if}
</div>

{#if adding}
	<AddMemberDrawer
		onClose={() => (adding = false)}
		onCreated={() => {
			adding = false;
			fetchUsers();
		}}
		onInvited={(invitations) => {
			adding = false;
			links = invitations;
			fetchUsers();
		}}
	/>
{/if}

{#if links}
	<InviteLinksDrawer invitations={links} onClose={() => (links = null)} />
{/if}

{#if editing}
	<EditMemberDrawer
		user={editing}
		onClose={() => (editing = null)}
		onSaved={() => {
			editing = null;
			fetchUsers();
		}}
	/>
{/if}
