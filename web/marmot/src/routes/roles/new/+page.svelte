<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import StepperPage from '$components/ui/StepperPage.svelte';
	import PermissionEditor from '$lib/components/PermissionEditor.svelte';
	import { createRole, listPermissions } from '$lib/roles/api';
	import type { Permission } from '$lib/roles/types';
	import { toasts } from '$lib/stores/toast';

	let name = $state('');
	let description = $state('');
	let selectedPermIds = $state<string[]>([]);
	let allPermissions = $state<Permission[]>([]);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let currentStep = $state(1);

	const stepperSteps = [
		{ title: 'Basic Info', icon: 'material-symbols:info-outline' },
		{ title: 'Permissions', icon: 'material-symbols:security' },
		{ title: 'Review', icon: 'material-symbols:summarize' }
	];

	let canProceedToStep2 = $derived(name.trim().length >= 2);

	function canNavigateToStep(step: number): boolean {
		if (step === 2) return canProceedToStep2;
		if (step === 3) return canProceedToStep2;
		return false;
	}

	onMount(async () => {
		try {
			allPermissions = await listPermissions();
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : 'Failed to load permissions');
		}
	});

	// Group selected permissions by resource type for review
	let groupedSelected = $derived.by(() => {
		const selected = allPermissions.filter((p) => selectedPermIds.includes(p.id));
		return selected.reduce(
			(acc, p) => {
				if (!acc[p.resource_type]) acc[p.resource_type] = [];
				acc[p.resource_type].push(p);
				return acc;
			},
			{} as Record<string, Permission[]>
		);
	});

	function handleNext() {
		error = null;
		if (currentStep === 1) {
			if (!canProceedToStep2) {
				error = 'Name must be at least 2 characters';
				return;
			}
			currentStep = 2;
			return;
		}
		if (currentStep === 2) {
			currentStep = 3;
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
			await createRole({
				name: name.trim(),
				description: description.trim() || undefined,
				permission_ids: selectedPermIds
			});
			toasts.success(`Role "${name.trim()}" created`);
			goto(resolve('/admin?tab=roles'));
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create role';
		} finally {
			saving = false;
		}
	}

	function goBack() {
		goto(resolve('/admin?tab=roles'));
	}
</script>

<StepperPage
	title="Create Role"
	steps={stepperSteps}
	{currentStep}
	onBack={goBack}
	onCancel={goBack}
	onPrevious={() => currentStep--}
	onNext={handleNext}
	onSave={handleSave}
	canProceed={currentStep === 1 ? canProceedToStep2 : true}
	{saving}
	saveLabel="Create Role"
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
						for="role-name"
						class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
					>
						Name <span class="text-red-500">*</span>
					</label>
					<input
						id="role-name"
						type="text"
						bind:value={name}
						placeholder="e.g., data-reader, pipeline-operator"
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
						A short, memorable identifier for this role.
					</p>
				</div>

				<div>
					<label
						for="role-description"
						class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
					>
						Description
					</label>
					<textarea
						id="role-description"
						bind:value={description}
						rows="3"
						placeholder="Who should get this role and what should they be able to do?"
						class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all resize-none"
					></textarea>
					<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
						Explains the intent — future admins will thank you.
					</p>
				</div>
			</div>
		</div>
	{/if}

	{#if currentStep === 2}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
		>
			<div class="flex items-center justify-between mb-4">
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 flex items-center">
					<IconifyIcon
						icon="material-symbols:security"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					Permissions
					<span class="ml-2 text-xs font-normal text-gray-500">(Optional)</span>
				</h3>
				<span class="text-xs text-gray-500 dark:text-gray-400">
					{selectedPermIds.length} selected
				</span>
			</div>

			<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
				Pick what this role can do. You can toggle whole resource groups by clicking the section
				header.
			</p>

			<PermissionEditor
				selectedIds={selectedPermIds}
				onChange={(ids) => (selectedPermIds = ids)}
			/>
		</div>
	{/if}

	{#if currentStep === 3}
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

			<dl class="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm mb-6">
				<div>
					<dt class="text-gray-500 dark:text-gray-400">Name</dt>
					<dd class="font-medium text-gray-900 dark:text-gray-100 font-mono">{name}</dd>
				</div>
				<div>
					<dt class="text-gray-500 dark:text-gray-400">Description</dt>
					<dd class="font-medium text-gray-900 dark:text-gray-100">{description || '—'}</dd>
				</div>
			</dl>

			<div>
				<div class="flex items-center justify-between mb-3">
					<h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">
						Permissions
					</h4>
					<span class="text-xs text-gray-500 dark:text-gray-400">
						{selectedPermIds.length} of {allPermissions.length} granted
					</span>
				</div>

				{#if selectedPermIds.length === 0}
					<div
						class="p-4 rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20 text-sm text-amber-800 dark:text-amber-200 flex items-start gap-2"
					>
						<IconifyIcon
							icon="material-symbols:warning-outline"
							class="h-5 w-5 mt-0.5 flex-shrink-0"
						/>
						<div>
							<p class="font-medium">No permissions selected.</p>
							<p class="text-xs mt-1">
								This role won't grant any access. You can add permissions later by editing the role.
							</p>
						</div>
					</div>
				{:else}
					<div class="space-y-2">
						{#each Object.entries(groupedSelected) as [resourceType, perms] (resourceType)}
							<div
								class="border border-gray-200 dark:border-gray-700 rounded-md overflow-hidden bg-gray-50 dark:bg-gray-900/40"
							>
								<div
									class="flex items-center gap-2 px-3 py-2 bg-earthy-brown-100 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700"
								>
									<span
										class="text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider"
									>
										{resourceType}
									</span>
									<span class="ml-auto text-xs text-gray-500">{perms.length}</span>
								</div>
								<div class="flex flex-wrap gap-1.5 p-3">
									{#each perms as perm (perm.id)}
										<span
											class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900 text-earthy-terracotta-700 dark:text-earthy-terracotta-100"
										>
											{perm.name}
										</span>
									{/each}
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	{/if}
</StepperPage>
