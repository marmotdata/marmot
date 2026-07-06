<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import StepperPage from '$components/ui/StepperPage.svelte';
	import { getServiceAccount, createAPIKey } from '$lib/serviceaccounts/api';
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
		{ title: 'Configure', icon: 'material-symbols:settings-outline' },
		{ title: 'Review', icon: 'material-symbols:summarize' },
		{ title: 'Save Key', icon: 'material-symbols:key-outline' }
	];

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
			error = 'Missing service account id';
			return;
		}
		try {
			sa = await getServiceAccount(id);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load service account';
		}
	});

	function handleNext() {
		error = null;
		if (currentStep === 1) {
			if (!canProceedToStep2) {
				error = 'Name must be at least 2 characters';
				return;
			}
			currentStep = 2;
		}
	}

	async function handleCreate() {
		if (!sa) return;
		if (!name.trim()) {
			error = 'Name is required';
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
			toasts.success('API key created');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create API key';
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
			toasts.error('Could not copy to clipboard');
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
	title="Create API Key"
	steps={stepperSteps}
	{currentStep}
	onBack={goBack}
	onCancel={goBack}
	onPrevious={currentStep === 2 && !plaintextKey ? () => (currentStep = 1) : undefined}
	onNext={currentStep === 1 ? handleNext : undefined}
	onSave={currentStep === 2 && !plaintextKey ? handleCreate : currentStep === 3 ? finish : undefined}
	canProceed={currentStep === 1 ? canProceedToStep2 : true}
	{saving}
	saveLabel={currentStep === 3 ? 'Done' : 'Create Key'}
	savingLabel="Creating..."
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
				Configure Key
			</h3>

			<div class="space-y-6">
				<div>
					<label
						for="key-name"
						class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
					>
						Key name <span class="text-red-500">*</span>
					</label>
					<input
						id="key-name"
						type="text"
						bind:value={name}
						placeholder="e.g., github-actions-ci"
						onkeydown={(e) => {
							if (e.key === 'Enter' && canProceedToStep2) {
								e.preventDefault();
								handleNext();
							}
						}}
						class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600"
					/>
					<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
						A label so you can identify which system is using this key later.
					</p>
				</div>

				<div>
					<div class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
						Expiration
					</div>
					<div class="grid grid-cols-2 md:grid-cols-5 gap-2">
						{#each [{ id: 'never', label: 'Never' }, { id: '30', label: '30 days' }, { id: '90', label: '90 days' }, { id: '365', label: '1 year' }, { id: 'custom', label: 'Custom' }] as opt (opt.id)}
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
								Days
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
						Short-lived keys are safer. Long-lived keys are convenient but need rotation.
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
				Review
			</h3>

			<dl class="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
				<div>
					<dt class="text-gray-500 dark:text-gray-400">Key name</dt>
					<dd class="font-medium text-gray-900 dark:text-gray-100 font-mono">{name}</dd>
				</div>
				<div>
					<dt class="text-gray-500 dark:text-gray-400">Expiration</dt>
					<dd class="font-medium text-gray-900 dark:text-gray-100">
						{expiryDays === 0 ? 'Never' : `${expiryDays} days`}
					</dd>
				</div>
				{#if sa}
					<div class="sm:col-span-2">
						<dt class="text-gray-500 dark:text-gray-400">Scoped to service account</dt>
						<dd class="font-medium text-gray-900 dark:text-gray-100">{sa.name}</dd>
					</div>
					{#if sa.roles.length > 0}
						<div class="sm:col-span-2">
							<dt class="text-gray-500 dark:text-gray-400">Inherits roles</dt>
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
					<p class="font-medium">The plaintext key will be shown only once.</p>
					<p class="mt-1">
						On the next step, copy the key immediately and store it somewhere secure — Marmot cannot
						show it again.
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
				Save Your API Key
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
							Key <code class="font-mono">{plaintextKey.name}</code> created.
						</p>
						<p class="text-xs text-green-700 dark:text-green-300 mt-0.5">
							Copy the value below now. It will not be shown again.
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
							Copied
						{:else}
							<IconifyIcon icon="material-symbols:content-copy" class="h-4 w-4" />
							Copy
						{/if}
					</button>
				</div>
			</div>

			<div class="rounded-lg border border-gray-200 dark:border-gray-700 p-4">
				<h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">
					Use the key
				</h4>
				<p class="text-xs text-gray-500 dark:text-gray-400 mb-3">
					Pass the key in the <code>X-API-Key</code> header on requests to the Marmot API.
				</p>
				<pre class="text-xs bg-gray-900 dark:bg-black text-gray-100 rounded-md p-3 overflow-x-auto"><code
						>curl -H "X-API-Key: {plaintextKey.key}" \
     {`\${MARMOT_HOST}`}/api/v1/service-accounts</code
					></pre>
			</div>
		</div>
	{/if}
</StepperPage>
