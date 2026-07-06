<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import StepperPage from '$components/ui/StepperPage.svelte';
	import RoleSelector from '$lib/components/RoleSelector.svelte';
	import Avatar from '$components/user/Avatar.svelte';
	import { listRoles } from '$lib/roles/api';
	import { fetchApi } from '$lib/api';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import type { Role } from '$lib/roles/types';

	let username = $state('');
	let name = $state('');
	let password = $state('');
	let passwordConfirm = $state('');
	let showPassword = $state(false);
	let selectedRoleIds = $state<string[]>([]);
	let availableRoles = $state<Role[]>([]);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let currentStep = $state(1);

	const stepperSteps = [
		{ title: 'Basic Info', icon: 'material-symbols:person-outline' },
		{ title: 'Credentials', icon: 'material-symbols:lock-outline' },
		{ title: 'Roles', icon: 'material-symbols:shield-outline' },
		{ title: 'Review', icon: 'material-symbols:summarize' }
	];

	// Validation
	let usernameError = $derived.by(() => {
		const u = username.trim();
		if (!u) return null;
		if (u.length < 3) return 'Username must be at least 3 characters';
		if (u.length > 255) return 'Username is too long';
		if (!/^[a-zA-Z0-9._-]+$/.test(u))
			return 'Only letters, numbers, dots, underscores, and hyphens';
		return null;
	});
	let passwordError = $derived.by(() => {
		if (!password) return null;
		if (password.length < 8) return 'Password must be at least 8 characters';
		return null;
	});
	let confirmError = $derived.by(() => {
		if (!passwordConfirm) return null;
		if (passwordConfirm !== password) return 'Passwords do not match';
		return null;
	});

	let canProceedToStep2 = $derived(
		username.trim().length >= 3 && name.trim().length > 0 && usernameError === null
	);
	let canProceedToStep3 = $derived(password.length >= 8 && passwordConfirm === password);
	let canProceedToStep4 = $derived(selectedRoleIds.length > 0);

	function canNavigateToStep(step: number): boolean {
		if (step >= 2 && !canProceedToStep2) return false;
		if (step >= 3 && !canProceedToStep3) return false;
		if (step >= 4 && !canProceedToStep4) return false;
		return true;
	}

	// Password strength (0..4)
	let passwordStrength = $derived.by(() => {
		if (!password) return 0;
		let score = 0;
		if (password.length >= 8) score++;
		if (password.length >= 12) score++;
		if (/[A-Z]/.test(password) && /[a-z]/.test(password)) score++;
		if (/\d/.test(password) && /[^A-Za-z0-9]/.test(password)) score++;
		return score;
	});
	let strengthLabel = $derived(['', 'Weak', 'Fair', 'Good', 'Strong'][passwordStrength] ?? '');

	// Convert selected IDs to role objects and names for review + submit
	let selectedRoles = $derived(availableRoles.filter((r) => selectedRoleIds.includes(r.id)));

	onMount(async () => {
		try {
			availableRoles = await listRoles();
			// Default to "user" role if it exists (matches old form's default)
			const userRole = availableRoles.find((r) => r.name === 'user');
			if (userRole) selectedRoleIds = [userRole.id];
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : 'Failed to load roles');
		}
	});

	function handleNext() {
		error = null;
		if (currentStep === 1 && !canProceedToStep2) {
			error = usernameError ?? 'Fill in all required fields';
			return;
		}
		if (currentStep === 2 && !canProceedToStep3) {
			error = passwordError ?? confirmError ?? 'Password does not meet requirements';
			return;
		}
		if (currentStep === 3 && !canProceedToStep4) {
			error = 'Select at least one role';
			return;
		}
		currentStep = Math.min(currentStep + 1, stepperSteps.length);
	}

	async function handleSave() {
		if (!canProceedToStep2 || !canProceedToStep3 || !canProceedToStep4) {
			error = 'Please complete all required fields';
			return;
		}

		try {
			saving = true;
			error = null;

			const payload = {
				username: username.trim(),
				name: name.trim(),
				password,
				role_names: selectedRoles.map((r) => r.name)
			};

			const res = await fetchApi('/users', {
				method: 'POST',
				body: JSON.stringify(payload)
			});
			if (!res.ok) {
				error = await handleApiError(res);
				return;
			}

			toasts.success(`User "${username.trim()}" created`);
			goto(resolve('/admin?tab=users'));
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create user';
		} finally {
			saving = false;
		}
	}

	function goBack() {
		goto(resolve('/admin?tab=users'));
	}
</script>

<StepperPage
	title="Create User"
	steps={stepperSteps}
	{currentStep}
	onBack={goBack}
	onCancel={goBack}
	onPrevious={() => currentStep--}
	onNext={currentStep < stepperSteps.length ? handleNext : undefined}
	onSave={currentStep === stepperSteps.length ? handleSave : undefined}
	canProceed={currentStep === 1
		? canProceedToStep2
		: currentStep === 2
			? canProceedToStep3
			: currentStep === 3
				? canProceedToStep4
				: true}
	{saving}
	saveLabel="Create User"
	savingLabel="Creating..."
	{error}
	{canNavigateToStep}
	onStepClick={(step) => (currentStep = step)}
>
	<!-- Step 1: Basic Info -->
	{#if currentStep === 1}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
		>
			<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
				<IconifyIcon
					icon="material-symbols:person-outline"
					class="h-5 w-5 mr-2 text-earthy-terracotta-600"
				/>
				Basic Information
			</h3>

			<div class="space-y-6">
				<div class="grid grid-cols-1 md:grid-cols-2 gap-6">
					<div>
						<label
							for="user-username"
							class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
						>
							Username <span class="text-red-500">*</span>
						</label>
						<input
							id="user-username"
							type="text"
							bind:value={username}
							placeholder="e.g., alice"
							autocomplete="off"
							class="w-full px-4 py-2.5 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all font-mono {usernameError
								? 'border-red-500 dark:border-red-500'
								: 'border-gray-300 dark:border-gray-600'}"
							required
						/>
						{#if usernameError}
							<p class="mt-1.5 text-sm text-red-600 dark:text-red-400 flex items-center">
								<IconifyIcon icon="material-symbols:error" class="h-4 w-4 mr-1 flex-shrink-0" />
								{usernameError}
							</p>
						{:else}
							<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
								Letters, numbers, dots, underscores, hyphens. 3–255 characters.
							</p>
						{/if}
					</div>

					<div>
						<label
							for="user-name"
							class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
						>
							Display name <span class="text-red-500">*</span>
						</label>
						<input
							id="user-name"
							type="text"
							bind:value={name}
							placeholder="e.g., Alice Chen"
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
							Shown across the app — usually the person's full name.
						</p>
					</div>
				</div>
			</div>
		</div>
	{/if}

	<!-- Step 2: Credentials -->
	{#if currentStep === 2}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
		>
			<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
				<IconifyIcon
					icon="material-symbols:lock-outline"
					class="h-5 w-5 mr-2 text-earthy-terracotta-600"
				/>
				Set Password
			</h3>

			<p class="text-sm text-gray-600 dark:text-gray-400 mb-6">
				The user can change this after logging in. Consider sharing it via a secure channel.
			</p>

			<div class="space-y-6">
				<div>
					<label
						for="user-password"
						class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
					>
						Password <span class="text-red-500">*</span>
					</label>
					<div class="relative">
						<input
							id="user-password"
							type={showPassword ? 'text' : 'password'}
							bind:value={password}
							autocomplete="new-password"
							placeholder="At least 8 characters"
							class="w-full px-4 py-2.5 pr-10 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all font-mono {passwordError
								? 'border-red-500 dark:border-red-500'
								: 'border-gray-300 dark:border-gray-600'}"
						/>
						<button
							type="button"
							onclick={() => (showPassword = !showPassword)}
							class="absolute right-2 top-1/2 -translate-y-1/2 p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 rounded-md hover:bg-gray-100 dark:hover:bg-gray-600"
							aria-label={showPassword ? 'Hide password' : 'Show password'}
						>
							<IconifyIcon
								icon={showPassword
									? 'material-symbols:visibility-off-outline'
									: 'material-symbols:visibility-outline'}
								class="h-4 w-4"
							/>
						</button>
					</div>

					<!-- Strength meter -->
					{#if password}
						<div class="mt-3">
							<div class="flex gap-1">
								{#each [1, 2, 3, 4] as tick (tick)}
									<div
										class="h-1 flex-1 rounded-full transition-colors
											{passwordStrength >= tick
											? passwordStrength <= 1
												? 'bg-red-500'
												: passwordStrength === 2
													? 'bg-amber-500'
													: passwordStrength === 3
														? 'bg-lime-500'
														: 'bg-green-500'
											: 'bg-gray-200 dark:bg-gray-700'}"
									></div>
								{/each}
							</div>
							<div class="flex items-center justify-between mt-1.5">
								<span
									class="text-xs
										{passwordStrength <= 1
										? 'text-red-600 dark:text-red-400'
										: passwordStrength === 2
											? 'text-amber-600 dark:text-amber-400'
											: passwordStrength === 3
												? 'text-lime-600 dark:text-lime-400'
												: 'text-green-600 dark:text-green-400'}"
								>
									{strengthLabel}
								</span>
								<span class="text-xs text-gray-400 dark:text-gray-500">
									{password.length} chars
								</span>
							</div>
						</div>
					{/if}

					{#if passwordError}
						<p class="mt-2 text-sm text-red-600 dark:text-red-400 flex items-center">
							<IconifyIcon icon="material-symbols:error" class="h-4 w-4 mr-1 flex-shrink-0" />
							{passwordError}
						</p>
					{/if}
				</div>

				<div>
					<label
						for="user-password-confirm"
						class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
					>
						Confirm password <span class="text-red-500">*</span>
					</label>
					<input
						id="user-password-confirm"
						type={showPassword ? 'text' : 'password'}
						bind:value={passwordConfirm}
						autocomplete="new-password"
						placeholder="Re-enter the password"
						onkeydown={(e) => {
							if (e.key === 'Enter' && canProceedToStep3) {
								e.preventDefault();
								handleNext();
							}
						}}
						class="w-full px-4 py-2.5 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all font-mono {confirmError
							? 'border-red-500 dark:border-red-500'
							: passwordConfirm && !confirmError
								? 'border-green-500 dark:border-green-500'
								: 'border-gray-300 dark:border-gray-600'}"
					/>
					{#if confirmError}
						<p class="mt-2 text-sm text-red-600 dark:text-red-400 flex items-center">
							<IconifyIcon icon="material-symbols:error" class="h-4 w-4 mr-1 flex-shrink-0" />
							{confirmError}
						</p>
					{:else if passwordConfirm && password && passwordConfirm === password}
						<p class="mt-2 text-sm text-green-600 dark:text-green-400 flex items-center">
							<IconifyIcon
								icon="material-symbols:check-circle-outline"
								class="h-4 w-4 mr-1 flex-shrink-0"
							/>
							Passwords match
						</p>
					{/if}
				</div>
			</div>
		</div>
	{/if}

	<!-- Step 3: Roles -->
	{#if currentStep === 3}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
		>
			<div class="flex items-center justify-between mb-4">
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 flex items-center">
					<IconifyIcon
						icon="material-symbols:shield-outline"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					Assign Roles <span class="text-red-500 ml-1">*</span>
				</h3>
			</div>

			<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
				Pick at least one role. Roles determine what this user can see and do.
			</p>

			<RoleSelector
				roles={availableRoles}
				selectedIds={selectedRoleIds}
				onChange={(ids) => (selectedRoleIds = ids)}
				emptyMessage="No roles available."
			/>
		</div>
	{/if}

	<!-- Step 4: Review -->
	{#if currentStep === 4}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
		>
			<!-- Identity header -->
			<div
				class="flex items-center gap-4 px-6 py-5 border-b border-gray-200 dark:border-gray-700 bg-gradient-to-r from-earthy-brown-50/50 to-transparent dark:from-gray-800/60"
			>
				<div class="shrink-0">
					<Avatar name={name || username || '?'} size="lg" />
				</div>
				<div class="min-w-0">
					<div class="text-xs uppercase tracking-wider text-gray-500 dark:text-gray-400 mb-0.5">
						Ready to create
					</div>
					<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 truncate">
						{name || 'unnamed'}
					</h3>
					<p class="text-sm text-gray-500 dark:text-gray-400 font-mono truncate">
						@{username || 'unset'}
					</p>
				</div>
			</div>

			<div class="p-6 space-y-5">
				<!-- Credentials -->
				<div>
					<div
						class="flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5 uppercase tracking-wide"
					>
						<IconifyIcon icon="material-symbols:lock-outline" class="h-3.5 w-3.5" />
						Password
					</div>
					<div class="flex items-center gap-2">
						<code
							class="text-sm font-mono text-gray-900 dark:text-gray-100 bg-gray-100 dark:bg-gray-900 rounded px-2 py-1"
						>
							{'•'.repeat(Math.min(password.length, 16))}
						</code>
						<span class="text-xs text-gray-500 dark:text-gray-400"
							>{password.length} chars · {strengthLabel}</span
						>
					</div>
				</div>

				<!-- Roles -->
				<div class="border-t border-gray-100 dark:border-gray-700/60 pt-5">
					<div class="flex items-center justify-between mb-2.5">
						<div
							class="flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide"
						>
							<IconifyIcon icon="material-symbols:shield-outline" class="h-3.5 w-3.5" />
							Roles
						</div>
						<span class="text-xs text-gray-400 dark:text-gray-500">
							{selectedRoles.length} assigned
						</span>
					</div>

					{#if selectedRoles.length === 0}
						<p class="text-sm text-gray-400 dark:text-gray-500 italic">None</p>
					{:else}
						<ul class="space-y-1.5">
							{#each selectedRoles as role (role.id)}
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
					<p class="font-medium text-gray-900 dark:text-gray-100">Next: share credentials</p>
					<p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
						Deliver the username and password securely. The user can change their password once
						logged in.
					</p>
				</div>
			</div>
		</div>
	{/if}
</StepperPage>
