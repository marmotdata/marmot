<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import OAuthButtons from '$components/auth/OAuthButtons.svelte';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/utils';
	import { acceptInvite, InviteError, lookupInvite } from '$lib/users/invites';
	import type { InviteInfo } from '$lib/users/types';

	// Where an invited person arrives from the link they were sent: they choose a password and are signed straight in. When password sign-in is off they accept by signing in with single sign-on instead, which matches them by email.
	let token = $derived(page.params.token ?? '');

	let invite = $state<InviteInfo | null>(null);
	let loadError = $state('');
	let expired = $state(false);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state('');
	let name = $state('');
	let password = $state('');
	let confirm = $state('');

	onMount(async () => {
		try {
			invite = await lookupInvite(token);
			name = invite.name;
		} catch (err) {
			expired = err instanceof InviteError && err.status === 410;
			loadError = err instanceof Error ? err.message : m.invite_error_load();
		} finally {
			loading = false;
		}
	});

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (saving) return;
		error = '';
		if (password.length < 8) {
			error = m.login_error_password_too_short();
			return;
		}
		if (password !== confirm) {
			error = m.login_error_password_mismatch();
			return;
		}
		saving = true;
		try {
			const { access_token } = await acceptInvite(token, password, name.trim());
			auth.setToken(access_token);
			window.location.assign(resolve('/'));
		} catch (err) {
			error = err instanceof Error ? err.message : m.invite_error_accept();
		} finally {
			saving = false;
		}
	}

	function valueOf(e: Event): string {
		return (e.target as HTMLInputElement).value;
	}
</script>

<svelte:head>
	<title>{m.invite_title()}</title>
</svelte:head>

<div class="sheet">
	<section class="brand" aria-label="Marmot">
		<mm-topography seed={5} fade="end"></mm-topography>
		<mm-logo size={36} clear-space></mm-logo>
	</section>

	<main class="side">
		<div class="mm-stack form">
			{#if loading}
				<mm-loading label={m.invite_loading()}></mm-loading>
			{:else if !invite}
				<mm-heading level={1} variant="display-title">
					{expired ? m.invite_expired_heading() : m.invite_invalid_heading()}
				</mm-heading>
				<mm-text variant="body" tone="secondary" block>{loadError}</mm-text>
				<mm-button href={resolve('/login')} icon="login">{m.invite_go_to_sign_in()}</mm-button>
			{:else}
				<mm-heading level={1} variant="display-title">{m.invite_heading()}</mm-heading>
				<mm-text variant="body" tone="secondary" block>
					{m.invite_intro({ email: invite.email, date: formatDate(invite.expires_at) })}
				</mm-text>
				{#if invite.password_sign_in}
					<form class="mm-stack fields" onsubmit={handleSubmit}>
						{#if error}
							<mm-callout tone="danger" compact urgent>{error}</mm-callout>
						{/if}
						<mm-text-field
							name="name"
							label={m.common_name()}
							autocomplete="name"
							value={name}
							required
							help={m.members_name_help()}
							oninput={(e: Event) => (name = valueOf(e))}
						></mm-text-field>
						<mm-text-field
							name="username"
							label={m.login_username_label()}
							autocomplete="username"
							value={invite.email}
							readonly
						></mm-text-field>
						<mm-text-field
							name="new-password"
							type="password"
							label={m.login_new_password_label()}
							autocomplete="new-password"
							help={m.login_new_password_help()}
							value={password}
							required
							oninput={(e: Event) => (password = valueOf(e))}
						></mm-text-field>
						<mm-text-field
							name="confirm-password"
							type="password"
							label={m.login_confirm_password_label()}
							autocomplete="new-password"
							value={confirm}
							required
							oninput={(e: Event) => (confirm = valueOf(e))}
						></mm-text-field>
						<mm-button class="block" variant="primary" type="submit" loading={saving}>
							{m.invite_accept_button()}
						</mm-button>
					</form>
					<OAuthButtons />
				{:else}
					<mm-text variant="body" block>{m.invite_sso_only({ email: invite.email })}</mm-text>
					<OAuthButtons />
				{/if}
			{/if}
		</div>
	</main>
</div>

<style>
	.sheet {
		min-height: 100vh;
		background: var(--mm-paper);
		color: var(--mm-text);
		display: grid;
		grid-template-columns: minmax(0, 5fr) minmax(0, 4fr);
	}
	.brand {
		position: relative;
		isolation: isolate;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		justify-content: space-between;
		gap: var(--space-6);
		padding: var(--space-8);
		background: var(--mm-sunken);
		border-inline-end: 1px solid var(--mm-rule);
	}
	.side {
		display: grid;
		align-items: center;
		justify-items: start;
		padding: var(--space-8) var(--space-12);
	}
	.form {
		--mm-gap: var(--space-5);
		inline-size: min(100%, 24rem);
	}
	.fields {
		--mm-gap: var(--space-4);
	}
	.block {
		display: flex;
	}
	@media (max-width: 768px) {
		.sheet {
			grid-template-columns: minmax(0, 1fr);
			grid-template-rows: auto 1fr;
		}
		.brand {
			min-height: 220px;
			padding: var(--space-5) var(--space-4);
			border-inline-end: 0;
			border-block-end: 1px solid var(--mm-rule);
		}
		.side {
			align-items: start;
			padding: var(--space-6) var(--space-4) var(--space-8);
		}
	}
</style>
