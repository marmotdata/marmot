<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import Button from '$components/ui/Button.svelte';
	let {
		enrollmentToken = null,
		onEnrolled
	}: { enrollmentToken?: string | null; onEnrolled?: () => void } = $props();

	let status = $state<{ local: boolean; enabled: boolean; recovery_remaining: number } | null>(
		null
	);
	let setup = $state<{ secret: string; uri: string; qr: string } | null>(null);
	let password = $state('');
	let code = $state('');
	let recovery = $state<string[]>([]);
	let enrollmentAccessToken = $state('');
	let busy = $state(false);
	let required = $state(false);
	let error = $state('');
	let copied = $state<'secret' | 'recovery' | null>(null);
	let action = $state<'setup' | 'disable' | 'recovery' | null>(null);

	onMount(async () => {
		if (enrollmentToken) {
			status = { local: true, enabled: false, recovery_remaining: 0 };
			action = 'setup';
			return;
		}
		try {
			const config = await fetch('/auth-providers');
			if (!config.ok) return;
			const flags = await config.json();
			if (!flags.totp_enabled) return;
			required = !!flags.totp_required;
			const response = await fetchApi('/users/totp');
			if (!response.ok) throw new Error(m.totp_error());
			status = await response.json();
		} catch {
			error = m.totp_error();
		}
	});

	async function submit() {
		if (!action || busy) return;
		busy = true;
		error = '';
		const path = enrollmentToken
			? setup
				? '/users/login/totp/confirm'
				: '/users/login/totp/setup'
			: setup
				? '/users/totp/confirm'
				: action === 'disable'
					? '/users/totp'
					: `/users/totp/${action}`;
		try {
			const response = await fetch(`/api/v1${path}`, {
				method: action === 'disable' ? 'DELETE' : 'POST',
				headers: {
					'Content-Type': 'application/json',
					'X-Marmot-Client': 'web',
					...(enrollmentToken ? {} : { Authorization: `Bearer ${auth.getToken() || ''}` })
				},
				body: JSON.stringify({ password, code: code.trim(), mfa_token: enrollmentToken })
			});
			if (!response.ok)
				throw new Error(
					response.status === 503
						? m.totp_unavailable()
						: response.status === 429
							? m.totp_rate_limited()
							: m.totp_invalid_code()
				);
			const result = await response.json();
			if (result.secret) {
				setup = result;
			} else {
				if (result.access_token) {
					if (enrollmentToken) enrollmentAccessToken = result.access_token;
					else auth.setToken(result.access_token);
				}
				recovery = result.recovery_codes || [];
				if (status)
					status = {
						...status,
						enabled: action !== 'disable',
						recovery_remaining: recovery.length
					};
				setup = null;
				action = null;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : m.totp_error();
		} finally {
			busy = false;
			password = '';
			code = '';
		}
	}

	function cancel() {
		action = null;
		setup = null;
		copied = null;
		password = '';
		code = '';
		error = '';
	}

	async function copy(value: string, target: 'secret' | 'recovery') {
		try {
			await navigator.clipboard.writeText(value);
			copied = target;
			setTimeout(() => {
				if (copied === target) copied = null;
			}, 2000);
		} catch {
			error = m.totp_copy_error();
		}
	}
</script>

{#if error}<p role="alert" class="mt-5 text-sm text-red-700 dark:text-red-300">{error}</p>{/if}
{#if status?.local}
	<section
		class={enrollmentToken
			? ''
			: 'rounded-lg border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800'}
		aria-labelledby="totp-title"
	>
		{#if !enrollmentToken}
			<h2 id="totp-title" class="text-lg font-semibold">{m.totp_title()}</h2>
			<p class="mt-2 text-sm text-gray-600 dark:text-gray-300">{m.totp_local_help()}</p>
		{:else}
			<h2 id="totp-title" class="sr-only">{m.totp_title()}</h2>
		{/if}
		{#if recovery.length}
			<div class="mx-auto {enrollmentToken ? '' : 'mt-5'} max-w-xl space-y-4 text-center">
				<p class="text-sm font-medium">{m.totp_recovery_help()}</p>
				<div class="relative rounded-md bg-gray-50 dark:bg-gray-900">
					<button
						type="button"
						class="absolute right-2 top-2 inline-flex h-8 w-8 items-center justify-center rounded-md text-gray-500 transition hover:bg-gray-200 hover:text-gray-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-white"
						aria-label={copied === 'recovery' ? m.common_copied() : m.common_copy()}
						title={copied === 'recovery' ? m.common_copied() : m.common_copy()}
						onclick={() => void copy(recovery.join('\n'), 'recovery')}
					>
						<IconifyIcon
							icon={copied === 'recovery'
								? 'material-symbols:check'
								: 'material-symbols:content-copy-outline'}
							class="h-4 w-4"
						/>
					</button>
					<ul class="space-y-2 p-4 pr-12 text-left font-mono text-sm">
						{#each recovery as item (item)}<li class="break-all select-all">{item}</li>{/each}
					</ul>
				</div>
				<div class="flex flex-wrap justify-center gap-3">
					<Button
						text={m.totp_saved_codes()}
						variant="filled"
						click={() => {
							recovery = [];
							if (enrollmentAccessToken) auth.setToken(enrollmentAccessToken);
							onEnrolled?.();
						}}
					/>
				</div>
			</div>
		{:else if action}
			<form
				class="{enrollmentToken ? '' : 'mt-5'} space-y-4"
				onsubmit={(event) => {
					event.preventDefault();
					void submit();
				}}
			>
				{#if setup}
					<div class="mx-auto flex max-w-md flex-col items-center gap-4 text-center">
						<p class="text-sm">{m.totp_scan_help()}</p>
						{#if setup.qr}<img
								src={setup.qr}
								alt={m.totp_qr_alt()}
								width="240"
								height="240"
								class="rounded border bg-white p-2"
							/>{/if}
						<code
							class="max-w-full break-all rounded-md bg-gray-50 px-4 py-3 text-sm select-all dark:bg-gray-900"
							>{setup.secret}</code
						>
						<Button
							text={copied === 'secret' ? m.common_copied() : m.common_copy()}
							variant="clear"
							click={() => void copy(setup.secret, 'secret')}
						/>
					</div>
				{:else if !enrollmentToken}<label class="block text-sm font-medium" for="totp-password"
						>{m.login_password_label()}</label
					>
					<input
						id="totp-password"
						type="password"
						autocomplete="current-password"
						bind:value={password}
						required
						maxlength="72"
						class="block w-full max-w-md rounded-md border border-gray-300 bg-white p-2.5 dark:border-gray-600 dark:bg-gray-700"
					/>
				{/if}
				{#if setup || action !== 'setup'}
					<label class="block text-sm font-medium" for="totp-settings-code">{m.totp_code()}</label>
					<input
						id="totp-settings-code"
						autocomplete="one-time-code"
						bind:value={code}
						required
						maxlength="128"
						class="block w-full max-w-md rounded-md border border-gray-300 bg-white p-2.5 font-mono dark:border-gray-600 dark:bg-gray-700"
					/>
				{/if}
				<div class="flex gap-3">
					<Button
						type="submit"
						loading={busy}
						text={setup
							? m.totp_verify()
							: action === 'disable'
								? m.totp_disable()
								: action === 'recovery'
									? m.totp_regenerate()
									: m.totp_setup()}
						variant="filled"
					/>
					{#if !enrollmentToken}<Button
							text={m.common_cancel()}
							click={cancel}
							disabled={busy}
							variant="clear"
						/>{/if}
				</div>
			</form>
		{:else if status.enabled}
			<p class="mt-4 text-sm">{m.totp_enabled({ count: String(status.recovery_remaining) })}</p>
			<div class="mt-4 flex flex-wrap gap-3">
				<Button text={m.totp_regenerate()} click={() => (action = 'recovery')} variant="clear" />
				{#if !required}<Button
						text={m.totp_disable()}
						click={() => (action = 'disable')}
						variant="clear"
					/>{/if}
			</div>
		{:else}
			<div class="mt-4">
				<Button text={m.totp_setup()} click={() => (action = 'setup')} variant="filled" />
			</div>
		{/if}
	</section>
{/if}
