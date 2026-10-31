<script lang="ts">
	import { onMount } from 'svelte';
	import '@marmotdata/elements/drawer.js';
	import '@marmotdata/elements/multiselect.js';
	import { fetchApi } from '$lib/api';
	import { listRoles } from '$lib/roles/api';
	import { toasts, parseApiError, isLimitExceeded } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { formatList } from '$lib/utils';
	import { roleLabel } from '$lib/users/format';
	import type { Role } from '$lib/roles/types';
	import type { Team } from '$lib/teams/types';
	import type { Invitation } from '$lib/users/types';
	import { inviteMember } from '$lib/users/invites';

	// Either creates an account now with a temporary password, or invites people by email. Marmot sends no email, so an invitation is a one-time link that onInvited hands back for whoever invited them to pass on.
	let {
		onClose,
		onCreated,
		onInvited
	}: {
		onClose: () => void;
		onCreated: () => void;
		onInvited: (invitations: Invitation[]) => void;
	} = $props();

	let how = $state<'create' | 'invite'>('create');
	let emails = $state('');
	let emailsError = $state('');

	let name = $state('');
	let username = $state('');
	let password = $state('');
	let roleIds = $state<string[]>([]);
	let teamIds = $state<string[]>([]);
	let roles = $state<Role[]>([]);
	let teams = $state<Team[]>([]);
	let saving = $state(false);
	let error = $state('');
	let usernameError = $state('');
	let passwordError = $state('');
	let rolesError = $state('');

	function checkUsername(): boolean {
		const u = username.trim();
		usernameError = !u
			? m.members_error_username_required()
			: u.length < 3
				? m.users_error_username_min()
				: u.length > 255
					? m.users_error_username_too_long()
					: !/^[a-zA-Z0-9._-]+$/.test(u)
						? m.members_error_username_charset()
						: '';
		return !usernameError;
	}

	function checkPassword(): boolean {
		passwordError = password.length < 8 ? m.members_error_password_min() : '';
		return !passwordError;
	}

	// Letters and digits without look-alikes, so the password survives being read out or retyped
	function generatePassword() {
		const alphabet = 'abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789';
		const bytes = crypto.getRandomValues(new Uint32Array(16));
		password = Array.from(bytes, (b) => alphabet[b % alphabet.length]).join('');
		passwordError = '';
	}

	onMount(async () => {
		try {
			roles = await listRoles();
			// New members start with the everyday "user" role when the instance has it
			const userRole = roles.find((r) => r.name === 'user');
			if (userRole) roleIds = [userRole.id];
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.users_error_load_roles());
		}
		try {
			const res = await fetchApi('/teams?limit=100');
			if (res.ok) teams = (await res.json()).teams ?? [];
		} catch {
			teams = [];
		}
	});

	function parseEmails(): string[] | null {
		const list = [
			...new Set(
				emails
					.split(/[\s,;]+/)
					.map((x) => x.trim())
					.filter(Boolean)
			)
		];
		const bad = list.filter((x) => !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(x));
		emailsError = !list.length
			? m.members_error_emails_required()
			: bad.length
				? m.members_error_emails_invalid({ emails: formatList(bad) })
				: '';
		return emailsError ? null : list;
	}

	async function handleInvite() {
		const list = parseEmails();
		rolesError = roleIds.length ? '' : m.users_error_select_role();
		if (!list || rolesError) return;

		saving = true;
		const done: Invitation[] = [];
		const failed: string[] = [];
		try {
			for (const email of list) {
				try {
					const inv = await inviteMember({
						email,
						role_names: roles.filter((r) => roleIds.includes(r.id)).map((r) => r.name),
						team_ids: teamIds
					});
					done.push(inv);
					if (inv.failed_team_ids?.length) {
						const names = inv.failed_team_ids.map(
							(id) => teams.find((t) => t.id === id)?.name ?? id
						);
						toasts.warning(m.members_invited_team_failed({ email, teams: formatList(names) }));
					}
				} catch (err) {
					failed.push(
						`${email}: ${err instanceof Error ? err.message : m.members_invite_failed()}`
					);
				}
			}
			if (failed.length) error = failed.join(' ');
			if (done.length && !failed.length) onInvited(done);
			else if (done.length) {
				// Keep the drawer open on the failures, but the links that were made must not be lost
				toasts.success(m.members_invited_some({ count: done.length }));
				onInvited(done);
			}
		} finally {
			saving = false;
		}
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		if (how === 'invite') {
			await handleInvite();
			return;
		}
		const ok = [!!name.trim(), checkUsername(), checkPassword()].every(Boolean);
		rolesError = roleIds.length ? '' : m.users_error_select_role();
		if (!ok || rolesError) return;

		saving = true;
		try {
			const res = await fetchApi('/users', {
				method: 'POST',
				body: JSON.stringify({
					username: username.trim(),
					name: name.trim(),
					password,
					role_names: roles.filter((r) => roleIds.includes(r.id)).map((r) => r.name)
				})
			});
			if (!res.ok) {
				const info = await parseApiError(res);
				error = info.message;
				if (isLimitExceeded(info)) toasts.warning(info.message);
				return;
			}
			const user = await res.json();
			const failed: string[] = [];
			for (const id of teamIds) {
				const added = await fetchApi(`/teams/${id}/members`, {
					method: 'POST',
					body: JSON.stringify({ user_id: user.id, role: 'member' })
				}).catch(() => null);
				if (!added?.ok) failed.push(teams.find((t) => t.id === id)?.name ?? id);
			}
			if (failed.length)
				toasts.warning(m.members_created_team_failed({ teams: formatList(failed) }));
			else toasts.success(m.users_create_success({ username: username.trim() }));
			onCreated();
		} catch (err) {
			error = err instanceof Error ? err.message : m.users_error_create();
		} finally {
			saving = false;
		}
	}

	const val = (e: Event) => (e.currentTarget as unknown as { value: string }).value;
</script>

<mm-drawer open heading={m.members_add_button()} onmm-close={onClose}>
	<form
		id="add-member"
		class="mm-stack"
		style="--mm-gap: var(--space-6)"
		novalidate
		onsubmit={handleSubmit}
	>
		{#if error}
			<mm-callout
				tone="danger"
				heading={how === 'invite' ? m.members_invite_failed() : m.members_create_failed()}
				>{error}</mm-callout
			>
		{/if}
		<mm-segmented-control
			label={m.members_how_to_add()}
			value={how}
			onmm-change={(e: CustomEvent<{ value: string }>) => {
				how = e.detail.value as typeof how;
				error = '';
			}}
		>
			<mm-segment value="create" icon="person-add">{m.members_mode_create()}</mm-segment>
			<mm-segment value="invite" icon="mail">{m.members_mode_invite()}</mm-segment>
		</mm-segmented-control>
		{#if how === 'invite'}
			<mm-textarea
				label={m.members_emails_label()}
				name="emails"
				rows="3"
				required
				value={emails}
				help={m.members_emails_help()}
				error={emailsError}
				oninput={(e: Event) => (emails = val(e))}
			></mm-textarea>
		{:else}
			<div class="mm-stack" style="--mm-gap: var(--space-4)">
				<mm-text-field
					label={m.common_name()}
					name="name"
					required
					value={name}
					help={m.members_name_help()}
					oninput={(e: Event) => (name = val(e))}
				></mm-text-field>
				<mm-text-field
					label={m.users_username_label()}
					name="username"
					required
					autocomplete="off"
					value={username}
					help={m.members_username_help()}
					error={usernameError}
					oninput={(e: Event) => (username = val(e))}
					onmm-change={checkUsername}
				></mm-text-field>
				<div class="mm-stack" style="--mm-gap: var(--space-2)">
					<mm-text-field
						label={m.members_temporary_password()}
						name="password"
						type="password"
						required
						autocomplete="new-password"
						value={password}
						help={m.members_password_help()}
						error={passwordError}
						oninput={(e: Event) => (password = val(e))}
						onmm-change={checkPassword}
					></mm-text-field>
					<div>
						<mm-button size="sm" variant="ghost" icon="refresh" onclick={generatePassword}>
							{m.members_generate_password()}
						</mm-button>
					</div>
				</div>
			</div>
		{/if}
		<mm-multiselect
			label={m.users_header_roles()}
			name="roles"
			required
			value={roleIds}
			error={rolesError}
			onmm-change={(e: CustomEvent<{ value: string[] }>) => {
				roleIds = e.detail.value;
				rolesError = '';
			}}
		>
			{#each roles as role (role.id)}
				<mm-option value={role.id} label={roleLabel(role.name)} description={role.description}
				></mm-option>
			{/each}
		</mm-multiselect>
		{#if teams.length}
			<mm-multiselect
				label={m.settings_teams()}
				name="teams"
				optional
				value={teamIds}
				onmm-change={(e: CustomEvent<{ value: string[] }>) => (teamIds = e.detail.value)}
			>
				{#each teams as team (team.id)}
					<mm-option value={team.id} label={team.name} icon="groups"></mm-option>
				{/each}
			</mm-multiselect>
		{/if}
	</form>
	<mm-button slot="footer" data-close="cancel">{m.common_cancel()}</mm-button>
	<mm-button
		slot="footer"
		variant="primary"
		type="submit"
		form="add-member"
		icon={how === 'invite' ? 'mail' : 'person-add'}
		loading={saving}
	>
		{how === 'invite' ? m.members_create_invitations() : m.members_create_account()}
	</mm-button>
</mm-drawer>
