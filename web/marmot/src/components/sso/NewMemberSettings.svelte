<script lang="ts">
	import { onMount } from 'svelte';
	import type { MmValueDetail } from '@marmotdata/elements/classes/form-field.js';
	import { getSignInSettings, saveSignInSettings } from '$lib/sso/api';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { roleLabel } from '$lib/users/format';
	import type { SignInSettings } from '$lib/sso/types';
	import type { Role } from '$lib/roles/types';
	import type { Team } from '$lib/teams/types';

	// What happens the first time someone signs in with single sign-on, and whether passwords still work. Administrators always keep password sign-in, so turning it off cannot lock everyone out.
	let {
		roles,
		teams,
		onChange
	}: { roles: Role[]; teams: Team[]; onChange?: (s: SignInSettings) => void } = $props();

	let form = $state<SignInSettings | null>(null);
	let loadError = $state('');
	let error = $state('');
	let saving = $state(false);

	async function load() {
		loadError = '';
		try {
			form = await getSignInSettings();
			onChange?.(form);
		} catch (err) {
			loadError = err instanceof Error ? err.message : m.sso_settings_load_error();
		}
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		if (!form) return;
		error = '';
		saving = true;
		try {
			form = await saveSignInSettings(form);
			onChange?.(form);
			toasts.success(m.sso_settings_saved_toast());
		} catch (err) {
			error = err instanceof Error ? err.message : m.sso_settings_save_failed();
		} finally {
			saving = false;
		}
	}

	onMount(load);
</script>

<mm-section heading={m.sso_new_members_heading()}>
	{#if loadError}
		<mm-callout tone="danger" heading={m.sso_settings_load_error()}>
			{loadError}
			<mm-button slot="actions" size="sm" icon="refresh" onclick={load}
				>{m.common_retry()}</mm-button
			>
		</mm-callout>
	{:else if !form}
		<mm-skeleton count={4}></mm-skeleton>
	{:else}
		<form class="mm-stack" style="--mm-gap: var(--space-5); max-width: 36rem" onsubmit={save}>
			{#if error}
				<mm-callout tone="danger" compact>{error}</mm-callout>
			{/if}
			<mm-switch
				label={m.sso_create_on_first_sso()}
				description={m.sso_create_on_first_sso_description()}
				checked={form.create_on_first_sso}
				state-label
				onmm-change={(e: CustomEvent<{ checked: boolean }>) =>
					form && (form.create_on_first_sso = e.detail.checked)}
			></mm-switch>
			<mm-select
				label={m.sso_default_role()}
				value={form.default_role}
				help={m.sso_default_role_help()}
				onmm-change={(e: CustomEvent<MmValueDetail>) =>
					form && (form.default_role = e.detail.value)}
			>
				{#each roles as role (role.id)}
					<mm-option value={role.name} label={roleLabel(role.name)}></mm-option>
				{/each}
			</mm-select>
			<mm-select
				label={m.sso_default_team()}
				value={form.default_team_id}
				onmm-change={(e: CustomEvent<MmValueDetail>) =>
					form && (form.default_team_id = e.detail.value)}
			>
				<mm-option value="" label={m.sso_mapping_no_team()}></mm-option>
				{#each teams as team (team.id)}
					<mm-option value={team.id} label={team.name}></mm-option>
				{/each}
			</mm-select>
			<mm-switch
				label={m.sso_allow_password()}
				description={m.sso_allow_password_description()}
				checked={form.allow_password_sign_in}
				state-label
				onmm-change={(e: CustomEvent<{ checked: boolean }>) =>
					form && (form.allow_password_sign_in = e.detail.checked)}
			></mm-switch>
			<div>
				<mm-button variant="primary" type="submit" loading={saving}>
					{m.sso_settings_save()}
				</mm-button>
			</div>
		</form>
	{/if}
</mm-section>
