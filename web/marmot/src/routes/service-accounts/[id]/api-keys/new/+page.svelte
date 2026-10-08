<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import StepperPage from '$components/ui/StepperPage.svelte';
	import { getServiceAccount, createAPIKey } from '$lib/serviceaccounts/api';
	import { m } from '$lib/paraglide/messages';
	import type { ServiceAccount, ServiceAccountAPIKey } from '$lib/serviceaccounts/types';
	import { toasts } from '$lib/stores/toast';

	let sa = $state<ServiceAccount | null>(null);
	let name = $state('');
	let expiresPreset = $state<'never' | '30' | '90' | '365' | 'custom'>('never');
	let customDays = $state(30);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let currentStep = $state(1);
	let plaintextKey = $state<ServiceAccountAPIKey | null>(null);
	let copied = $state(false);

	const stepperSteps = [
		{ title: m.serviceaccounts_key_step_configure(), icon: 'material-symbols:settings-outline' },
		{ title: m.serviceaccounts_step_review(), icon: 'material-symbols:summarize' },
		{ title: m.serviceaccounts_key_step_save(), icon: 'material-symbols:key-outline' }
	];

	// The sample request is a shell command, so it stays out of the message catalogue
	let exampleRequest = $derived(
		`curl -H "X-API-Key: ${plaintextKey?.key ?? ''}" \\\n     \${MARMOT_HOST}/api/v1/service-accounts`
	);

	let canProceedToStep2 = $derived(name.trim().length >= 2);
	let expiryDays = $derived(
		expiresPreset === 'never'
			? 0
			: expiresPreset === 'custom'
				? Math.max(1, customDays)
				: parseInt(expiresPreset, 10)
	);

	function canNavigateToStep(step: number): boolean {
		if (plaintextKey) return step === 3;
		if (step === 2) return canProceedToStep2;
		return false;
	}

	onMount(async () => {
		const id = $page.params.id;
		if (!id) {
			error = m.serviceaccounts_error_missing_id();
			return;
		}
		try {
			sa = await getServiceAccount(id);
		} catch (err) {
			error = err instanceof Error ? err.message : m.serviceaccounts_error_load();
		}
	});

	function handleNext() {
		error = null;
		if (currentStep === 1) {
			if (!canProceedToStep2) {
				error = m.serviceaccounts_error_name_min();
				return;
			}
			currentStep = 2;
		}
	}

	async function handleCreate() {
		if (!sa) return;
		if (!name.trim()) {
			error = m.serviceaccounts_error_name_required();
			return;
		}
		try {
			saving = true;
			error = null;
			const key = await createAPIKey(sa.id, {
				name: name.trim(),
				expires_in_days: expiryDays > 0 ? expiryDays : undefined
			});
			plaintextKey = key;
			currentStep = 3;
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

	function finish() {
		if (!sa) return;
		goto(resolve(`/service-accounts/${sa.id}?created=1`));
	}

	function goBack() {
		if (plaintextKey) {
			finish();
			return;
		}
		if (!sa) {
			goto(resolve('/admin?tab=service_accounts'));
			return;
		}
		goto(resolve(`/service-accounts/${sa.id}`));
	}
</script>

<StepperPage
	title={m.serviceaccounts_create_key_button()}
	steps={stepperSteps}
	{currentStep}
	onBack={goBack}
	onCancel={goBack}
	onPrevious={currentStep === 2 && !plaintextKey ? () => (currentStep = 1) : undefined}
	onNext={currentStep === 1 ? handleNext : undefined}
	onSave={currentStep === 2 && !plaintextKey
		? handleCreate
		: currentStep === 3
			? finish
			: undefined}
	canProceed={currentStep === 1 ? canProceedToStep2 : true}
	{saving}
	saveLabel={currentStep === 3
		? m.serviceaccounts_key_done_button()
		: m.serviceaccounts_key_create_button()}
	savingLabel={m.serviceaccounts_creating_label()}
	saveIcon={currentStep === 3 ? 'material-symbols:check' : 'material-symbols:key'}
	{error}
	{canNavigateToStep}
	onStepClick={(step) => {
		if (plaintextKey) return;
		currentStep = step;
	}}
>
	{#if currentStep === 1}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
		>
			<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
				<IconifyIcon
					icon="material-symbols:settings-outline"
					class="h-5 w-5 mr-2 text-earthy-terracotta-600"
				/>
				{m.serviceaccounts_key_configure_heading()}
			</h3>

			<div class="space-y-6">
				<div>
					<label
						for="key-name"
						class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
					>
						{m.serviceaccounts_key_name_label()} <span class="text-red-500">*</span>
					</label>
					<input
						id="key-name"
						type="text"
						bind:value={name}
						placeholder={m.serviceaccounts_key_name_placeholder()}
						onkeydown={(e) => {
							if (e.key === 'Enter' && canProceedToStep2) {
								e.preventDefault();
								handleNext();
							}
						}}
						class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600"
					/>
					<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
						{m.serviceaccounts_key_name_hint()}
					</p>
				</div>

				<div>
					<div class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
						{m.apikeys_expiration_label()}
					</div>
					<div class="grid grid-cols-2 md:grid-cols-5 gap-2">
						{#each [{ id: 'never', label: m.common_never() }, { id: '30', label: m.apikeys_expiration_days_option( { count: 30 } ) }, { id: '90', label: m.apikeys_expiration_days_option( { count: 90 } ) }, { id: '365', label: m.serviceaccounts_key_expires_one_year() }, { id: 'custom', label: m.apikeys_expiration_custom() }] as opt (opt.id)}
							<button
								type="button"
								onclick={() => (expiresPreset = opt.id as typeof expiresPreset)}
								class="px-3 py-2 text-sm rounded-md border transition-colors
									{expiresPreset === opt.id
									? 'border-earthy-terracotta-500 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 text-earthy-terracotta-700 dark:text-earthy-terracotta-300'
									: 'border-gray-200 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:border-gray-300'}"
							>
								{opt.label}
							</button>
						{/each}
					</div>
					{#if expiresPreset === 'custom'}
						<div class="mt-3">
							<label
								for="custom-days"
								class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1"
							>
								{m.serviceaccounts_key_custom_days_label()}
							</label>
							<input
								id="custom-days"
								type="number"
								bind:value={customDays}
								min="1"
								class="w-32 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600"
							/>
						</div>
					{/if}
					<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
						{m.serviceaccounts_key_expiration_hint()}
					</p>
				</div>
			</div>
		</div>
	{/if}

	{#if currentStep === 2}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
		>
			<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
				<IconifyIcon
					icon="material-symbols:summarize"
					class="h-5 w-5 mr-2 text-earthy-terracotta-600"
				/>
				{m.serviceaccounts_step_review()}
			</h3>

			<dl class="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
				<div>
					<dt class="text-gray-500 dark:text-gray-400">{m.serviceaccounts_key_name_label()}</dt>
					<dd class="font-medium text-gray-900 dark:text-gray-100 font-mono">{name}</dd>
				</div>
				<div>
					<dt class="text-gray-500 dark:text-gray-400">{m.apikeys_expiration_label()}</dt>
					<dd class="font-medium text-gray-900 dark:text-gray-100">
						{expiryDays === 0
							? m.common_never()
							: m.apikeys_expiration_days_option({ count: expiryDays })}
					</dd>
				</div>
				{#if sa}
					<div class="sm:col-span-2">
						<dt class="text-gray-500 dark:text-gray-400">
							{m.serviceaccounts_key_scoped_to_label()}
						</dt>
						<dd class="font-medium text-gray-900 dark:text-gray-100">{sa.name}</dd>
					</div>
					{#if sa.roles.length > 0}
						<div class="sm:col-span-2">
							<dt class="text-gray-500 dark:text-gray-400">
								{m.serviceaccounts_key_inherits_roles_label()}
							</dt>
							<dd class="flex flex-wrap gap-1.5 mt-1">
								{#each sa.roles as role (role.id)}
									<span
										class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900 text-earthy-terracotta-700 dark:text-earthy-terracotta-100"
									>
										{role.name}
									</span>
								{/each}
							</dd>
						</div>
					{/if}
				{/if}
			</dl>

			<div
				class="mt-6 flex items-start gap-3 p-4 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg"
			>
				<IconifyIcon
					icon="material-symbols:warning-outline"
					class="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0"
				/>
				<div class="text-sm text-amber-800 dark:text-amber-200">
					<p class="font-medium">{m.serviceaccounts_key_shown_once_warning()}</p>
					<p class="mt-1">
						{m.serviceaccounts_key_copy_next_step_hint()}
					</p>
				</div>
			</div>
		</div>
	{/if}

	{#if currentStep === 3 && plaintextKey}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
		>
			<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
				<IconifyIcon
					icon="material-symbols:key-outline"
					class="h-5 w-5 mr-2 text-earthy-terracotta-600"
				/>
				{m.serviceaccounts_key_save_heading()}
			</h3>

			<div
				class="p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg mb-4"
			>
				<div class="flex items-start gap-2 mb-3">
					<IconifyIcon
						icon="material-symbols:check-circle"
						class="h-5 w-5 text-green-600 dark:text-green-400 mt-0.5"
					/>
					<div>
						<p class="text-sm font-medium text-green-800 dark:text-green-200">
							{m.serviceaccounts_key_created_named({ name: plaintextKey.name })}
						</p>
						<p class="text-xs text-green-700 dark:text-green-300 mt-0.5">
							{m.serviceaccounts_key_copy_now_hint()}
						</p>
					</div>
				</div>

				<div class="flex items-center gap-2">
					<code
						class="flex-1 text-xs bg-white dark:bg-gray-900 border border-green-200 dark:border-green-700 rounded px-3 py-3 font-mono break-all"
					>
						{plaintextKey.key}
					</code>
					<button
						type="button"
						class="shrink-0 flex items-center gap-1.5 px-3 py-2 rounded-md border border-green-300 dark:border-green-700 text-sm text-green-700 dark:text-green-400 hover:bg-green-100 dark:hover:bg-green-900/40"
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

			<div class="rounded-lg border border-gray-200 dark:border-gray-700 p-4">
				<h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">
					{m.serviceaccounts_key_use_heading()}
				</h4>
				<p class="text-xs text-gray-500 dark:text-gray-400 mb-3">
					{m.serviceaccounts_key_use_hint({ header: 'X-API-Key' })}
				</p>
				<pre
					class="text-xs bg-gray-900 dark:bg-black text-gray-100 rounded-md p-3 overflow-x-auto"><code
						>{exampleRequest}</code
					></pre>
			</div>
		</div>
	{/if}
</StepperPage>
