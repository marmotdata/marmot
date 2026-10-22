<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import Blade from '$components/ui/Blade.svelte';
	import { createAPIKey } from '$lib/serviceaccounts/api';
	import { m } from '$lib/paraglide/messages';
	import type { ServiceAccount, ServiceAccountAPIKey } from '$lib/serviceaccounts/types';
	import { toasts } from '$lib/stores/toast';

	interface Props {
		sa: ServiceAccount;
		onClose: () => void;
		onDone: () => void;
	}

	let { sa, onClose, onDone }: Props = $props();

	const formId = 'create-api-key-form';
	const sectionClass =
		'text-[11px] font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400';
	const labelClass = 'block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1';
	const inputClass =
		'w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-earthy-terracotta-500 focus:ring-2 focus:ring-earthy-terracotta-500/30 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100';

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
			<div
				class="rounded-lg border border-green-200 bg-green-50 p-4 dark:border-green-800 dark:bg-green-900/20"
			>
				<div class="mb-3 flex items-start gap-2">
					<IconifyIcon
						icon="material-symbols:check-circle"
						class="mt-0.5 h-5 w-5 text-green-600 dark:text-green-400"
					/>
					<p class="text-sm font-medium text-green-800 dark:text-green-200">
						{m.serviceaccounts_key_created_named({ name: plaintextKey.name })}
					</p>
				</div>
				<div class="flex items-center gap-2">
					<code
						class="flex-1 break-all rounded border border-green-200 bg-white px-3 py-3 font-mono text-xs dark:border-green-700 dark:bg-gray-900"
					>
						{plaintextKey.key}
					</code>
					<button
						type="button"
						class="flex shrink-0 items-center gap-1.5 rounded-md border border-green-300 px-3 py-2 text-sm text-green-700 hover:bg-green-100 dark:border-green-700 dark:text-green-400 dark:hover:bg-green-900/40"
						onclick={copyKey}
					>
						{#if copied}
							<IconifyIcon icon="material-symbols:check" class="h-4 w-4" />
							{m.serviceaccounts_key_copied_label()}
						{:else}
							<IconifyIcon icon="material-symbols:content-copy" class="h-4 w-4" />
							{m.common_copy()}
						{/if}
					</button>
				</div>
			</div>

			<div
				class="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-900/20"
			>
				<IconifyIcon
					icon="material-symbols:warning-outline"
					class="mt-0.5 h-5 w-5 flex-shrink-0 text-amber-600 dark:text-amber-400"
				/>
				<p class="text-sm text-amber-800 dark:text-amber-200">
					{m.serviceaccounts_key_copy_now_hint()}
				</p>
			</div>

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
				<div
					class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
				>
					{error}
				</div>
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
								class="rounded-md border px-3 py-2 text-sm transition-colors
									{expiresPreset === opt.id
									? 'border-earthy-terracotta-500 bg-earthy-terracotta-50 text-earthy-terracotta-700 dark:bg-earthy-terracotta-900/20 dark:text-earthy-terracotta-300'
									: 'border-gray-200 text-gray-700 hover:border-gray-300 dark:border-gray-600 dark:text-gray-300'}"
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
								<span
									class="inline-flex items-center rounded-md bg-earthy-terracotta-100 px-2 py-0.5 text-xs font-medium text-earthy-terracotta-700 dark:bg-earthy-terracotta-900 dark:text-earthy-terracotta-100"
								>
									{role.name}
								</span>
							{/each}
						</div>
					</div>
				{/if}
			</section>

			<div
				class="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-900/20"
			>
				<IconifyIcon
					icon="material-symbols:warning-outline"
					class="mt-0.5 h-5 w-5 flex-shrink-0 text-amber-600 dark:text-amber-400"
				/>
				<div class="text-sm text-amber-800 dark:text-amber-200">
					<p class="font-medium">{m.serviceaccounts_key_shown_once_warning()}</p>
					<p class="mt-1">{m.serviceaccounts_key_copy_next_step_hint()}</p>
				</div>
			</div>
		</form>
	{/if}

	{#snippet footer(close)}
		<div class="flex items-center justify-end gap-2">
			{#if plaintextKey}
				<button
					type="button"
					onclick={onDone}
					class="inline-flex items-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500 disabled:opacity-50"
				>
					<IconifyIcon icon="material-symbols:check" class="h-4 w-4" />
					{m.serviceaccounts_key_done_button()}
				</button>
			{:else}
				<button
					type="button"
					onclick={close}
					class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
				>
					{m.common_cancel()}
				</button>
				<button
					type="submit"
					form={formId}
					disabled={saving || !nameValid}
					class="inline-flex items-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500 disabled:opacity-50"
				>
					<IconifyIcon icon="material-symbols:key" class="h-4 w-4" />
					{saving ? m.serviceaccounts_creating_label() : m.serviceaccounts_key_create_button()}
				</button>
			{/if}
		</div>
	{/snippet}
</Blade>
