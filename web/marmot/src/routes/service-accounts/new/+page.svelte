<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import StepperPage from '$components/ui/StepperPage.svelte';
	import RoleSelector from '$lib/components/RoleSelector.svelte';
	import { createServiceAccount } from '$lib/serviceaccounts/api';
	import { listRoles } from '$lib/roles/api';
	import type { Role } from '$lib/roles/types';
	import { toasts } from '$lib/stores/toast';
	import { onMount } from 'svelte';

	let name = $state('');
	let description = $state('');
	let selectedRoleIds = $state<string[]>([]);
	let availableRoles = $state<Role[]>([]);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let currentStep = $state(1);

	const stepperSteps = [
		{ title: 'Basic Info', icon: 'material-symbols:info-outline' },
		{ title: 'Roles', icon: 'material-symbols:shield-outline' },
		{ title: 'Review', icon: 'material-symbols:summarize' }
	];

	let canProceedToStep2 = $derived(name.trim().length >= 2);
	let canProceedToStep3 = $derived(true); // roles are optional

	function canNavigateToStep(step: number): boolean {
		if (step === 2) return canProceedToStep2;
		if (step === 3) return canProceedToStep2;
		return false;
	}

	onMount(async () => {
		try {
			availableRoles = await listRoles();
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : 'Failed to load roles');
		}
	});

	function handleNext() {
		error = null;
		if (currentStep === 1) {
			if (!canProceedToStep2) {
				error = 'Name must be at least 2 characters';
				return;
			}
			currentStep++;
			return;
		}
		if (currentStep === 2) {
			currentStep++;
			return;
		}
	}

	async function handleSave() {
		if (!name.trim()) {
			error = 'Name is required';
			return;
		}
		try {
			saving = true;
			error = null;
			const sa = await createServiceAccount({
				name: name.trim(),
				description: description.trim() || undefined,
				role_ids: selectedRoleIds.length ? selectedRoleIds : undefined
			});
			toasts.success(`Service account "${sa.name}" created`);
			goto(resolve(`/service-accounts/${sa.id}`));
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create service account';
		} finally {
			saving = false;
		}
	}

	function goBack() {
		goto(resolve('/admin?tab=service_accounts'));
	}
</script>

<StepperPage
	title="Create Service Account"
	steps={stepperSteps}
	{currentStep}
	onBack={goBack}
	onCancel={goBack}
	onPrevious={() => currentStep--}
	onNext={handleNext}
	onSave={handleSave}
	canProceed={currentStep === 1
		? canProceedToStep2
		: currentStep === 2
			? canProceedToStep3
			: !!name.trim()}
	{saving}
	saveLabel="Create Service Account"
	savingLabel="Creating..."
	{error}
	{canNavigateToStep}
	onStepClick={(step) => (currentStep = step)}
>
	{#if currentStep === 1}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
		>
			<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
				<IconifyIcon
					icon="material-symbols:info-outline"
					class="h-5 w-5 mr-2 text-earthy-terracotta-600"
				/>
				Basic Information
			</h3>

			<div class="space-y-6">
				<div>
					<label
						for="sa-name"
						class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
					>
						Name <span class="text-red-500">*</span>
					</label>
					<input
						id="sa-name"
						type="text"
						bind:value={name}
						placeholder="e.g., prod-airflow-ingester"
						onkeydown={(e) => {
							if (e.key === 'Enter' && canProceedToStep2) {
								e.preventDefault();
								handleNext();
							}
						}}
						class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
						required
					/>
					<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
						A unique, memorable identifier — usually the automation or system that will use it.
					</p>
				</div>

				<div>
					<label
						for="sa-description"
						class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
					>
						Description
					</label>
					<textarea
						id="sa-description"
						bind:value={description}
						rows="3"
						placeholder="What does this service account do? Who owns it?"
						class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all resize-none"
					></textarea>
					<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
						Helps other admins understand this account's purpose.
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
					icon="material-symbols:shield-outline"
					class="h-5 w-5 mr-2 text-earthy-terracotta-600"
				/>
				Assign Roles
				<span class="ml-2 text-xs font-normal text-gray-500">(Optional)</span>
			</h3>

			<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
				Grant this service account access by assigning one or more roles. You can change these
				later.
			</p>

			<RoleSelector
				roles={availableRoles}
				selectedIds={selectedRoleIds}
				onChange={(ids) => (selectedRoleIds = ids)}
				emptyMessage="No roles available."
			/>
		</div>
	{/if}

	{#if currentStep === 3}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
		>
			<!-- Header with SA identity -->
			<div
				class="flex items-center gap-4 px-6 py-5 border-b border-gray-200 dark:border-gray-700 bg-gradient-to-r from-earthy-brown-50/50 to-transparent dark:from-gray-800/60"
			>
				<div
					class="flex items-center justify-center h-12 w-12 rounded-lg bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/40 text-earthy-terracotta-700 dark:text-earthy-terracotta-300 shrink-0"
				>
					<IconifyIcon icon="material-symbols:smart-toy-outline" class="h-6 w-6" />
				</div>
				<div class="min-w-0">
					<div class="text-xs uppercase tracking-wider text-gray-500 dark:text-gray-400 mb-0.5">
						Ready to create
					</div>
					<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 font-mono truncate">
						{name || 'unnamed'}
					</h3>
				</div>
			</div>

			<!-- Field grid -->
			<div class="p-6 space-y-5">
				<div>
					<div
						class="flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5 uppercase tracking-wide"
					>
						<IconifyIcon icon="material-symbols:description-outline" class="h-3.5 w-3.5" />
						Description
					</div>
					{#if description.trim()}
						<p class="text-sm text-gray-900 dark:text-gray-100 leading-relaxed">
							{description}
						</p>
					{:else}
						<p class="text-sm text-gray-400 dark:text-gray-500 italic">Not set</p>
					{/if}
				</div>

				<div class="border-t border-gray-100 dark:border-gray-700/60 pt-5">
					<div class="flex items-center justify-between mb-2.5">
						<div
							class="flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide"
						>
							<IconifyIcon icon="material-symbols:shield-outline" class="h-3.5 w-3.5" />
							Roles
						</div>
						<span class="text-xs text-gray-400 dark:text-gray-500">
							{selectedRoleIds.length} assigned
						</span>
					</div>
					{#if selectedRoleIds.length === 0}
						<div
							class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400 px-3 py-2 rounded-md bg-gray-50 dark:bg-gray-900/40 border border-dashed border-gray-200 dark:border-gray-700"
						>
							<IconifyIcon icon="material-symbols:info-outline" class="h-4 w-4" />
							No roles — this account will have no access until you assign roles later.
						</div>
					{:else}
						<ul class="space-y-1.5">
							{#each selectedRoleIds as id (id)}
								{@const role = availableRoles.find((r) => r.id === id)}
								{#if role}
									<li
										class="flex items-start gap-3 px-3 py-2 rounded-md bg-earthy-terracotta-50/60 dark:bg-earthy-terracotta-900/15 border border-earthy-terracotta-100 dark:border-earthy-terracotta-900/30"
									>
										<IconifyIcon
											icon="material-symbols:check-circle"
											class="h-4 w-4 text-earthy-terracotta-600 dark:text-earthy-terracotta-400 mt-0.5 shrink-0"
										/>
										<div class="min-w-0 flex-1">
											<div class="flex items-center gap-2 flex-wrap">
												<span class="text-sm font-medium text-gray-900 dark:text-gray-100">
													{role.name}
												</span>
												{#if role.is_system}
													<span
														class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-200"
													>
														<IconifyIcon icon="material-symbols:lock" class="h-2.5 w-2.5" />
														system
													</span>
												{/if}
											</div>
											{#if role.description}
												<p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
													{role.description}
												</p>
											{/if}
										</div>
									</li>
								{/if}
							{/each}
						</ul>
					{/if}
				</div>
			</div>

			<!-- Next steps footer -->
			<div
				class="flex items-start gap-3 px-6 py-4 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/40"
			>
				<IconifyIcon
					icon="material-symbols:arrow-forward"
					class="h-5 w-5 text-gray-500 dark:text-gray-400 mt-0.5 flex-shrink-0"
				/>
				<div class="text-sm text-gray-700 dark:text-gray-300">
					<p class="font-medium text-gray-900 dark:text-gray-100">Next: mint an API key</p>
					<p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
						After creating this service account, you'll land on its detail page where you can create
						API keys.
					</p>
				</div>
			</div>
		</div>
	{/if}
</StepperPage>
