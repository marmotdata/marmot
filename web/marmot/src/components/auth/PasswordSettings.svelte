<script lang="ts">
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { Eye, EyeOff, KeyRound } from 'lucide-svelte';

	let currentPassword = '';
	let newPassword = '';
	let confirmPassword = '';
	let showPasswords = false;
	let busy = false;
	let error = '';
	let saved = false;
	$: validLength = newPassword.length >= 8 && newPassword.length <= 72;
	$: matches = newPassword.length > 0 && newPassword === confirmPassword;
	$: canSubmit = !!currentPassword && validLength && matches && !busy;

	async function changePassword() {
		if (!canSubmit) return;
		error = '';
		saved = false;
		busy = true;
		try {
			const response = await fetch('/api/v1/users/change-password', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${auth.getToken() || ''}`
				},
				body: JSON.stringify({ current_password: currentPassword, new_password: newPassword })
			});
			if (!response.ok)
				throw new Error(
					response.status === 401 ? m.profile_password_wrong_current() : m.profile_password_error()
				);
			const result = await response.json();
			auth.setToken(result.access_token);
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
			saved = true;
		} catch (err) {
			error = err instanceof Error ? err.message : m.profile_password_error();
		} finally {
			busy = false;
		}
	}
</script>

<section
	class="rounded-lg border border-gray-200 bg-earthy-brown-50 p-6 dark:border-gray-700 dark:bg-gray-900"
>
	<div class="flex items-start gap-3 border-b border-gray-200 pb-5 dark:border-gray-700">
		<div
			class="rounded-lg bg-earthy-terracotta-100 p-2.5 text-earthy-terracotta-700 dark:bg-earthy-terracotta-900 dark:text-earthy-terracotta-200"
		>
			<KeyRound class="h-5 w-5" />
		</div>
		<div>
			<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
				{m.profile_change_password()}
			</h2>
			<p class="mt-1 text-sm text-gray-600 dark:text-gray-300">
				{m.profile_change_password_help()}
			</p>
		</div>
	</div>

	<form
		class="mt-6 max-w-2xl space-y-6"
		onsubmit={(event) => {
			event.preventDefault();
			void changePassword();
		}}
	>
		<div>
			<label
				for="profile-current-password"
				class="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-100"
				>{m.profile_current_password()}</label
			>
			<input
				id="profile-current-password"
				type={showPasswords ? 'text' : 'password'}
				autocomplete="current-password"
				bind:value={currentPassword}
				required
				maxlength="72"
				class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2.5 text-gray-900 focus:border-earthy-terracotta-600 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-600/20 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
			/>
		</div>
		<div class="border-t border-gray-200 pt-5 dark:border-gray-700">
			<div class="mb-3 flex items-center justify-between gap-3">
				<h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
					{m.profile_new_password()}
				</h3>
				<button
					type="button"
					class="inline-flex items-center gap-1.5 text-sm text-earthy-terracotta-700 hover:underline dark:text-earthy-terracotta-300"
					aria-label={showPasswords ? m.profile_password_hide() : m.profile_password_show()}
					onclick={() => (showPasswords = !showPasswords)}
				>
					{#if showPasswords}<EyeOff class="h-4 w-4" />{m.profile_password_hide()}{:else}<Eye
							class="h-4 w-4"
						/>{m.profile_password_show()}{/if}
				</button>
			</div>
			<div class="grid gap-4 sm:grid-cols-2">
				<div>
					<label
						for="profile-new-password"
						class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
						>{m.profile_new_password()}</label
					>
					<input
						id="profile-new-password"
						type={showPasswords ? 'text' : 'password'}
						autocomplete="new-password"
						bind:value={newPassword}
						required
						minlength="8"
						maxlength="72"
						aria-describedby="profile-password-rules"
						class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2.5 text-gray-900 focus:border-earthy-terracotta-600 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-600/20 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
					/>
				</div>
				<div>
					<label
						for="profile-confirm-password"
						class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
						>{m.profile_confirm_password()}</label
					>
					<input
						id="profile-confirm-password"
						type={showPasswords ? 'text' : 'password'}
						autocomplete="new-password"
						bind:value={confirmPassword}
						required
						minlength="8"
						maxlength="72"
						class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2.5 text-gray-900 focus:border-earthy-terracotta-600 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-600/20 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
					/>
				</div>
			</div>
			<p
				id="profile-password-rules"
				class={`mt-2 text-xs ${validLength ? 'text-green-700 dark:text-green-300' : 'text-gray-500 dark:text-gray-400'}`}
			>
				{m.profile_password_length()}
			</p>
			{#if confirmPassword && !matches}<p class="mt-2 text-sm text-red-700 dark:text-red-300">
					{m.login_error_password_mismatch()}
				</p>{/if}
		</div>
		{#if error}<p role="alert" class="text-sm text-red-700 dark:text-red-300">{error}</p>{/if}
		{#if saved}<p role="status" class="text-sm text-green-700 dark:text-green-300">
				{m.profile_password_saved()}
			</p>{/if}
		<button
			type="submit"
			disabled={!canSubmit}
			class="rounded-lg bg-earthy-terracotta-700 px-5 py-2.5 text-sm font-semibold text-white hover:bg-earthy-terracotta-800 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-600 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
			>{m.profile_change_password()}</button
		>
	</form>
</section>
