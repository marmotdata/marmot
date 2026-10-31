<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { locales } from '$lib/paraglide/runtime';
	import { changeLocale, locale, type Locale } from '$lib/i18n';
	import { theme } from '$lib/stores/theme';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { userProfile } from '$lib/stores/user';
	import { providerName } from './providers';

	type ThemeChoice = 'auto' | 'light' | 'dark';

	interface Permission {
		name: string;
		description: string;
		action: string;
		resource_type: string;
	}

	interface Role {
		name: string;
		permissions?: Permission[];
	}

	interface Team {
		id: string;
		name: string;
		source: string;
		sso_provider?: string;
		sso_group?: string;
	}

	interface RoleSource {
		role: string;
		provider: string;
		group: string;
	}

	interface User {
		name: string;
		username?: string;
		job_title?: string;
		teams?: Team[];
		role_sources?: RoleSource[];
		roles?: Role[];
		identities?: { provider: string; provider_email?: string }[];
		preferences?: { theme?: ThemeChoice };
	}

	let user = $state<User | null>(null);
	let error = $state('');
	let savingLanguage = $state(false);
	let jobTitle = $state('');
	let jobTitleError = $state('');
	let savingProfile = $state(false);

	const profileChanged = $derived(!!user && jobTitle.trim() !== (user.job_title ?? ''));
	const teamNames = $derived((user?.teams ?? []).map((t) => t.name).join(', '));
	// Where each role came from when an identity provider group maps to it, e.g. "Okta group marmot-admins"
	const roleFrom = $derived(
		(user?.role_sources ?? [])
			.map((src) =>
				m.account_role_from_group({
					provider: providerName(src.provider) || m.account_sso(),
					group: src.group
				})
			)
			.join(', ')
	);

	const identity = $derived(user?.identities?.[0]);
	const idpName = $derived(identity ? providerName(identity.provider) || m.account_sso() : '');
	const roleNames = $derived((user?.roles ?? []).map((r) => r.name).join(', '));

	// Every permission the caller's roles grant, once each, since two roles often share the same grant; resource_type:action is the key every check, API call and Terraform binding uses
	const permissions = $derived.by(() => {
		const seen: Record<string, { key: string; description: string }> = {};
		for (const role of user?.roles ?? []) {
			for (const p of role.permissions ?? []) {
				const key = `${p.resource_type}:${p.action}`;
				seen[key] ??= { key, description: p.description };
			}
		}
		return Object.values(seen).sort((a, b) => a.key.localeCompare(b.key));
	});

	onMount(load);

	async function load() {
		error = '';
		try {
			const response = await fetchApi('/users/me');
			if (!response.ok) throw new Error(m.profile_load_error());
			user = await response.json();
			jobTitle = user?.job_title ?? '';
			// The saved preference wins over whatever this browser last used
			const saved = user?.preferences?.theme;
			if (saved && saved !== $theme) theme.set(saved);
		} catch (err) {
			error = err instanceof Error ? err.message : m.profile_load_error();
		}
	}

	function displayName(value: string): string {
		try {
			const name = new Intl.DisplayNames([value], { type: 'language' }).of(value);
			if (!name) return value;
			return name.charAt(0).toLocaleUpperCase(value) + name.slice(1);
		} catch {
			return value;
		}
	}

	function changeTheme(e: CustomEvent<{ value: string }>) {
		const next = e.detail.value as ThemeChoice;
		theme.set(next);
		// Saved in the background; the theme has already changed on screen
		fetchApi('/users/preferences', {
			method: 'PUT',
			body: JSON.stringify({ preferences: { theme: next } })
		})
			.then((response) => {
				if (!response.ok) toasts.error(m.account_theme_save_error());
			})
			.catch(() => toasts.error(m.account_theme_save_error()));
	}

	async function saveProfile(e: Event) {
		e.preventDefault();
		if (!user || savingProfile || !profileChanged) return;
		savingProfile = true;
		jobTitleError = '';
		try {
			const response = await fetchApi('/users/me/profile', {
				method: 'PUT',
				body: JSON.stringify({ job_title: jobTitle.trim() })
			});
			if (!response.ok) {
				const message = await handleApiError(response);
				if (response.status === 400) jobTitleError = message;
				else toasts.error(message);
				return;
			}
			const saved: User = await response.json();
			user = { ...user, job_title: saved.job_title };
			jobTitle = saved.job_title ?? '';
			userProfile.patch({ job_title: saved.job_title });
			toasts.success(m.account_profile_saved());
		} catch {
			toasts.error(m.account_profile_save_error());
		} finally {
			savingProfile = false;
		}
	}

	async function changeLanguage(e: CustomEvent<{ value: string }>) {
		const next = e.detail.value as Locale;
		if (next === $locale || savingLanguage) return;
		changeLocale(next);
		savingLanguage = true;
		try {
			const response = await fetchApi('/users/preferences', {
				method: 'PUT',
				body: JSON.stringify({ preferences: { language: next } })
			});
			if (!response.ok) throw new Error(String(response.status));
		} catch {
			toasts.error(m.profile_language_save_error());
		} finally {
			savingLanguage = false;
		}
	}
</script>

{#if error}
	<mm-callout tone="danger">
		{error}
		<mm-button slot="actions" size="sm" icon="refresh" onclick={load}
			>{m.account_try_again()}</mm-button
		>
	</mm-callout>
{:else if !user}
	<mm-skeleton loading shape="text" count={6}></mm-skeleton>
{:else}
	<div class="mm-split">
		<form class="mm-stack" style="--mm-gap: 0" onsubmit={saveProfile}>
			<mm-section
				heading={m.account_profile()}
				description={idpName ? m.account_profile_from_idp({ provider: idpName }) : ''}
			>
				<div class="mm-stack fields">
					<mm-text-field label={m.common_name()} value={user.name} readonly></mm-text-field>
					{#if user.username}
						<mm-text-field label={m.profile_username_label()} value={user.username} readonly
						></mm-text-field>
					{/if}
					{#if identity?.provider_email}
						<mm-text-field
							label={m.account_email()}
							type="email"
							value={identity.provider_email}
							readonly
						></mm-text-field>
					{/if}
					<mm-text-field
						label={m.account_job_title()}
						name="job-title"
						value={jobTitle}
						error={jobTitleError}
						oninput={(e: Event) => (jobTitle = (e.target as HTMLInputElement).value)}
					></mm-text-field>
				</div>
			</mm-section>
			<mm-section heading={m.account_appearance()}>
				<div class="mm-stack fields">
					<mm-segmented-control
						label={m.profile_theme_label()}
						value={$theme}
						onmm-change={changeTheme}
					>
						<mm-segment value="auto" label={m.account_theme_system()}></mm-segment>
						<mm-segment value="light" label={m.account_theme_light()}></mm-segment>
						<mm-segment value="dark" label={m.account_theme_peat()}></mm-segment>
					</mm-segmented-control>
					<mm-select
						label={m.profile_language_label()}
						value={$locale}
						disabled={(locales as readonly string[]).length < 2 || savingLanguage}
						onmm-change={changeLanguage}
					>
						{#each locales as value (value)}
							<mm-option {value} label={displayName(value)}></mm-option>
						{/each}
					</mm-select>
				</div>
			</mm-section>
			<div class="save">
				<mm-button variant="primary" type="submit" disabled={!profileChanged || savingProfile}
					>{m.account_save_profile()}</mm-button
				>
			</div>
		</form>

		<mm-section heading={m.account_roles_heading()}>
			<div class="mm-stack">
				<mm-description-list layout="stacked">
					<mm-description term={m.account_role()}
						>{roleNames || m.account_role_none()}</mm-description
					>
					{#if roleFrom}
						<mm-description term={m.account_role_from()}>{roleFrom}</mm-description>
					{/if}
					{#if teamNames}
						<mm-description term={m.account_teams()}>{teamNames}</mm-description>
					{/if}
					{#if idpName}
						<mm-description term={m.account_signs_in_with()}>{idpName}</mm-description>
					{/if}
				</mm-description-list>
				{#if permissions.length}
					<mm-disclosure summary={m.profile_permissions_heading()}>
						<mm-list dense style="--mm-list-inset: 0">
							{#each permissions as permission (permission.key)}
								<mm-list-item secondary={permission.description}>
									<span class="mm-mono">{permission.key}</span>
								</mm-list-item>
							{/each}
						</mm-list>
					</mm-disclosure>
				{/if}
			</div>
		</mm-section>
	</div>
{/if}

<style>
	.save {
		margin-block-start: var(--space-8);
	}
	.fields {
		--mm-gap: var(--space-5);
		max-width: 32rem;
	}
</style>
