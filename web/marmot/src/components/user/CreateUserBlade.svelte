<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import Blade from '$components/ui/Blade.svelte';
	import RoleSelector from '$lib/components/RoleSelector.svelte';
	import { listRoles } from '$lib/roles/api';
	import { fetchApi } from '$lib/api';
	import { toasts, parseApiError, isLimitExceeded } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import type { Role } from '$lib/roles/types';

	interface Props {
		onClose: () => void;
		onCreated: () => void;
	}

	let { onClose, onCreated }: Props = $props();

	const formId = 'create-user-form';
	const sectionClass =
		'text-[11px] font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400';
	const labelClass = 'block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1';
	const inputClass =
		'w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-earthy-terracotta-500 focus:ring-2 focus:ring-earthy-terracotta-500/30 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100';
	const invalidInputClass =
		'w-full rounded-lg border border-red-500 bg-white px-3 py-2 text-sm text-gray-900 focus:border-red-500 focus:ring-2 focus:ring-red-500/30 dark:border-red-500 dark:bg-gray-800 dark:text-gray-100';

	let username = $state('');
	let name = $state('');
	let password = $state('');
	let passwordConfirm = $state('');
	let showPassword = $state(false);
	let selectedRoleIds = $state<string[]>([]);
	let availableRoles = $state<Role[]>([]);
	let saving = $state(false);
	let error = $state<string | null>(null);

	let usernameError = $derived.by(() => {
		const u = username.trim();
		if (!u) return null;
		if (u.length < 3) return m.users_error_username_min();
		if (u.length > 255) return m.users_error_username_too_long();
		if (!/^[a-zA-Z0-9._-]+$/.test(u)) return m.users_error_username_charset();
		return null;
	});
	let passwordError = $derived.by(() => {
		if (!password) return null;
		if (password.length < 8) return m.users_error_password_min();
		return null;
	});
	let confirmError = $derived.by(() => {
		if (!passwordConfirm) return null;
		if (passwordConfirm !== password) return m.users_error_passwords_mismatch();
		return null;
	});

	let identityValid = $derived(
		username.trim().length >= 3 && name.trim().length > 0 && usernameError === null
	);
	let credentialsValid = $derived(password.length >= 8 && passwordConfirm === password);
	let rolesValid = $derived(selectedRoleIds.length > 0);
	let dirty = $derived(!!(username || name || password || passwordConfirm));

	let passwordStrength = $derived.by(() => {
		if (!password) return 0;
		let score = 0;
		if (password.length >= 8) score++;
		if (password.length >= 12) score++;
		if (/[A-Z]/.test(password) && /[a-z]/.test(password)) score++;
		if (/\d/.test(password) && /[^A-Za-z0-9]/.test(password)) score++;
		return score;
	});
	let strengthLabel = $derived(
		[
			'',
			m.users_strength_weak(),
			m.users_strength_fair(),
			m.users_strength_good(),
			m.users_strength_strong()
		][passwordStrength] ?? ''
	);

	let selectedRoles = $derived(availableRoles.filter((r) => selectedRoleIds.includes(r.id)));

	onMount(async () => {
		try {
			availableRoles = await listRoles();
			// Default to the "user" role when it exists, as the old form did
			const userRole = availableRoles.find((r) => r.name === 'user');
			if (userRole) selectedRoleIds = [userRole.id];
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.users_error_load_roles());
		}
	});

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = null;
		if (!identityValid) {
			error = usernameError ?? m.users_error_fill_required();
			return;
		}
		if (!credentialsValid) {
			error = passwordError ?? confirmError ?? m.users_error_password_requirements();
			return;
		}
		if (!rolesValid) {
			error = m.users_error_select_role();
			return;
		}

		try {
			saving = true;
			const res = await fetchApi('/users', {
				method: 'POST',
				body: JSON.stringify({
					username: username.trim(),
					name: name.trim(),
					password,
					role_names: selectedRoles.map((r) => r.name)
				})
			});
			if (!res.ok) {
				const info = await parseApiError(res);
				error = info.message;
				if (isLimitExceeded(info)) toasts.warning(info.message);
				return;
			}
			toasts.success(m.users_create_success({ username: username.trim() }));
			onCreated();
		} catch (err) {
			error = err instanceof Error ? err.message : m.users_error_create();
		} finally {
			saving = false;
		}
	}
</script>

<Blade title={m.users_create_title()} {onClose} width="xl" {dirty}>
	<form id={formId} onsubmit={handleSubmit} class="space-y-6">
		{#if error}
			<div
				class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
			>
				{error}
			</div>
		{/if}

		<section class="space-y-4">
			<h3 class={sectionClass}>{m.users_basic_information_heading()}</h3>
			<div>
				<label for="user-username" class={labelClass}>
					{m.users_username_label()} <span class="text-red-500">*</span>
				</label>
				<input
					id="user-username"
					type="text"
					bind:value={username}
					placeholder={m.users_username_placeholder()}
					autocomplete="off"
					class="{usernameError ? invalidInputClass : inputClass} font-mono"
					required
				/>
				{#if usernameError}
					<p class="mt-1.5 flex items-center text-sm text-red-600 dark:text-red-400">
						<IconifyIcon icon="material-symbols:error" class="mr-1 h-4 w-4 flex-shrink-0" />
						{usernameError}
					</p>
				{:else}
					<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{m.users_username_hint()}</p>
				{/if}
			</div>
			<div>
				<label for="user-name" class={labelClass}>
					{m.users_display_name_label()} <span class="text-red-500">*</span>
				</label>
				<input
					id="user-name"
					type="text"
					bind:value={name}
					placeholder={m.users_display_name_placeholder()}
					class={inputClass}
					required
				/>
				<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
					{m.users_display_name_hint()}
				</p>
			</div>
		</section>

		<section class="space-y-4">
			<div>
				<h3 class={sectionClass}>{m.users_set_password_heading()}</h3>
				<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{m.users_password_intro()}</p>
			</div>
			<div>
				<label for="user-password" class={labelClass}>
					{m.users_password_label()} <span class="text-red-500">*</span>
				</label>
				<div class="relative">
					<input
						id="user-password"
						type={showPassword ? 'text' : 'password'}
						bind:value={password}
						autocomplete="new-password"
						placeholder={m.users_password_placeholder()}
						class="{passwordError ? invalidInputClass : inputClass} pr-10 font-mono"
					/>
					<button
						type="button"
						onclick={() => (showPassword = !showPassword)}
						class="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-600 dark:hover:text-gray-200"
						aria-label={showPassword ? m.users_hide_password_aria() : m.users_show_password_aria()}
					>
						<IconifyIcon
							icon={showPassword
								? 'material-symbols:visibility-off-outline'
								: 'material-symbols:visibility-outline'}
							class="h-4 w-4"
						/>
					</button>
				</div>

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
						<div class="mt-1.5 flex items-center justify-between">
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
								{m.users_password_char_count({ count: password.length })}
							</span>
						</div>
					</div>
				{/if}

				{#if passwordError}
					<p class="mt-2 flex items-center text-sm text-red-600 dark:text-red-400">
						<IconifyIcon icon="material-symbols:error" class="mr-1 h-4 w-4 flex-shrink-0" />
						{passwordError}
					</p>
				{/if}
			</div>
			<div>
				<label for="user-password-confirm" class={labelClass}>
					{m.users_confirm_password_label()} <span class="text-red-500">*</span>
				</label>
				<input
					id="user-password-confirm"
					type={showPassword ? 'text' : 'password'}
					bind:value={passwordConfirm}
					autocomplete="new-password"
					placeholder={m.users_confirm_password_placeholder()}
					class="{confirmError ? invalidInputClass : inputClass} font-mono"
				/>
				{#if confirmError}
					<p class="mt-2 flex items-center text-sm text-red-600 dark:text-red-400">
						<IconifyIcon icon="material-symbols:error" class="mr-1 h-4 w-4 flex-shrink-0" />
						{confirmError}
					</p>
				{:else if passwordConfirm && password && passwordConfirm === password}
					<p class="mt-2 flex items-center text-sm text-green-600 dark:text-green-400">
						<IconifyIcon
							icon="material-symbols:check-circle-outline"
							class="mr-1 h-4 w-4 flex-shrink-0"
						/>
						{m.users_passwords_match()}
					</p>
				{/if}
			</div>
		</section>

		<section class="space-y-3">
			<div>
				<h3 class={sectionClass}>
					{m.users_assign_roles_heading()} <span class="text-red-500">*</span>
				</h3>
				<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{m.users_roles_intro()}</p>
			</div>
			<RoleSelector
				roles={availableRoles}
				selectedIds={selectedRoleIds}
				onChange={(ids) => (selectedRoleIds = ids)}
				emptyMessage={m.users_no_roles_available()}
			/>
		</section>
	</form>

	{#snippet footer(close)}
		<div class="flex items-center justify-end gap-2">
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
				disabled={saving || !identityValid || !credentialsValid || !rolesValid}
				class="inline-flex items-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500 disabled:opacity-50"
			>
				{saving ? m.users_creating_label() : m.users_create_title()}
			</button>
		</div>
	{/snippet}
</Blade>
