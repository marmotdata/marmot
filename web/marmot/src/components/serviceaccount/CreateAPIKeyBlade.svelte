<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import Blade from '$components/ui/Blade.svelte';
	import { createAPIKey } from '$lib/serviceaccounts/api';
	import { m } from '$lib/paraglide/messages';
	import { BTN_PRIMARY, BTN_SECONDARY, INPUT, LABEL } from '$lib/ui/classes';
	import Alert from '$components/ui/Alert.svelte';
	import Pill from '$components/ui/Pill.svelte';
	import type { ServiceAccount, ServiceAccountAPIKey } from '$lib/serviceaccounts/types';
	import { toasts } from '$lib/stores/toast';

	interface Props {
		sa: ServiceAccount;
		onClose: () => void;
		onDone: () => void;
	}

	let { sa, onClose, onDone }: Props = $props();

	const formId = 'create-api-key-form';
	const sectionClass = LABEL;
	const labelClass = 'mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300';
	const inputClass = INPUT;

	let name = $state('');
	let expiresPreset = $state<'never' | '30' | '90' | '365' | 'custom'>('never');
	let customDays = $state(30);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let plaintextKey = $state<ServiceAccountAPIKey | null>(null);
	let copied = $state(false);

	// The sample request is a shell command, so it stays out of the message catalog
	let exampleRequest = $derived(
		`curl -H "X-API-Key: ${plaintextKey?.key ?? ''}" \\\n     \${MARMOT_HOST}/api/v1/service-accounts`
	);

	let nameValid = $derived(name.trim().length >= 2);
	let expiryDays = $derived(
		expiresPreset === 'never'
			? 0
			: expiresPreset === 'custom'
				? Math.max(1, customDays)
				: parseInt(expiresPreset, 10)
	);
	let dirty = $derived(!plaintextKey && !!name);

	const expiryOptions = [
		{ id: 'never', label: m.common_never() },
		{ id: '30', label: m.apikeys_expiration_days_option({ count: 30 }) },
		{ id: '90', label: m.apikeys_expiration_days_option({ count: 90 }) },
		{ id: '365', label: m.serviceaccounts_key_expires_one_year() },
		{ id: 'custom', label: m.apikeys_expiration_custom() }
	] as const;

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = null;
		if (!name.trim()) {
			error = m.serviceaccounts_error_name_required();
			return;
		}
		if (!nameValid) {
			error = m.serviceaccounts_error_name_min();
			return;
		}
		try {
			saving = true;
			plaintextKey = await createAPIKey(sa.id, {
				name: name.trim(),
				expires_in_days: expiryDays > 0 ? expiryDays : undefined
			});
			toasts.success(m.serviceaccounts_key_created_banner());
		} catch (err) {
			error = err instanceof Error ? err.message : m.apikeys_create_error();
		} finally {
			saving = false;
		}
	}

	async function copyKey() {
		if (!plaintextKey?.key) return;
		try {
			await navigator.clipboard.writeText(plaintextKey.key);
			copied = true;
			setTimeout(() => (copied = false), 2000);
		} catch {
			toasts.error(m.serviceaccounts_copy_clipboard_error());
		}
	}
</script>

<!-- Once the key exists, any way of closing the blade counts as Done so the list picks up the new key -->
<Blade
	title={plaintextKey
		? m.serviceaccounts_key_save_heading()
		: m.serviceaccounts_create_key_button()}
	subtitle={sa.name}
	onClose={plaintextKey ? onDone : onClose}
	width="xl"
	{dirty}
>
	{#if plaintextKey}
		<div class="space-y-6">
			<Alert
				tone="success"
				title={m.serviceaccounts_key_created_named({ name: plaintextKey.name })}
			>
				<div class="mt-2 flex items-center gap-2">
					<code
						class="flex-1 break-all rounded-lg border border-gray-200 bg-white px-3 py-3 font-mono text-xs text-gray-900 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
					>
						{plaintextKey.key}
					</code>
					<button type="button" class="{BTN_SECONDARY} shrink-0" onclick={copyKey}>
						{#if copied}
							<IconifyIcon icon="material-symbols:check" class="h-4 w-4" />
							{m.serviceaccounts_key_copied_label()}
						{:else}
							<IconifyIcon icon="material-symbols:content-copy" class="h-4 w-4" />
							{m.common_copy()}
						{/if}
					</button>
				</div>
			</Alert>

			<Alert tone="warning">{m.serviceaccounts_key_copy_now_hint()}</Alert>

			<section class="space-y-2">
				<h3 class={sectionClass}>{m.serviceaccounts_key_use_heading()}</h3>
				<p class="text-xs text-gray-500 dark:text-gray-400">
					{m.serviceaccounts_key_use_hint({ header: 'X-API-Key' })}
				</p>
				<pre
					class="overflow-x-auto rounded-md bg-gray-900 p-3 text-xs text-gray-100 dark:bg-black"><code
						>{exampleRequest}</code
					></pre>
			</section>
		</div>
	{:else}
		<form id={formId} onsubmit={handleSubmit} class="space-y-6">
			{#if error}
				<Alert tone="error">{error}</Alert>
			{/if}

			<section class="space-y-4">
				<h3 class={sectionClass}>{m.serviceaccounts_key_configure_heading()}</h3>
				<div>
					<label for="key-name" class={labelClass}>
						{m.serviceaccounts_key_name_label()} <span class="text-red-500">*</span>
					</label>
					<input
						id="key-name"
						type="text"
						bind:value={name}
						placeholder={m.serviceaccounts_key_name_placeholder()}
						class={inputClass}
						required
					/>
					<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
						{m.serviceaccounts_key_name_hint()}
					</p>
				</div>

				<div>
					<div class={labelClass}>{m.apikeys_expiration_label()}</div>
					<div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
						{#each expiryOptions as opt (opt.id)}
							<button
								type="button"
								onclick={() => (expiresPreset = opt.id)}
								aria-pressed={expiresPreset === opt.id}
								class="rounded-lg border px-3 py-2 text-sm font-medium transition-colors
									{expiresPreset === opt.id
									? 'border-gray-900 bg-gray-900 text-white dark:border-gray-100 dark:bg-gray-100 dark:text-gray-900'
									: 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300'}"
							>
								{opt.label}
							</button>
						{/each}
					</div>
					{#if expiresPreset === 'custom'}
						<div class="mt-3">
							<label for="custom-days" class={labelClass}>
								{m.serviceaccounts_key_custom_days_label()}
							</label>
							<input
								id="custom-days"
								type="number"
								bind:value={customDays}
								min="1"
								class="{inputClass} !w-32"
							/>
						</div>
					{/if}
					<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
						{m.serviceaccounts_key_expiration_hint()}
					</p>
				</div>
			</section>

			<section class="space-y-3">
				<h3 class={sectionClass}>{m.serviceaccounts_key_scoped_to_label()}</h3>
				<p class="text-sm font-medium text-gray-900 dark:text-gray-100">{sa.name}</p>
				{#if sa.roles.length > 0}
					<div>
						<p class="text-xs text-gray-500 dark:text-gray-400">
							{m.serviceaccounts_key_inherits_roles_label()}
						</p>
						<div class="mt-1 flex flex-wrap gap-1.5">
							{#each sa.roles as role (role.id)}
								<Pill>{role.name}</Pill>
							{/each}
						</div>
					</div>
				{/if}
			</section>

			<Alert tone="warning" title={m.serviceaccounts_key_shown_once_warning()}>
				{m.serviceaccounts_key_copy_next_step_hint()}
			</Alert>
		</form>
	{/if}

	{#snippet footer(close)}
		<div class="flex items-center justify-end gap-2">
			{#if plaintextKey}
				<button type="button" onclick={onDone} class={BTN_PRIMARY}>
					<IconifyIcon icon="material-symbols:check" class="h-4 w-4" />
					{m.serviceaccounts_key_done_button()}
				</button>
			{:else}
				<button type="button" onclick={close} class={BTN_SECONDARY}>
					{m.common_cancel()}
				</button>
				<button type="submit" form={formId} disabled={saving || !nameValid} class={BTN_PRIMARY}>
					<IconifyIcon icon="material-symbols:key" class="h-4 w-4" />
					{saving ? m.serviceaccounts_creating_label() : m.serviceaccounts_key_create_button()}
				</button>
			{/if}
		</div>
	{/snippet}
</Blade>
