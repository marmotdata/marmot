<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import RoleSelector from '$lib/components/RoleSelector.svelte';
	import { toasts } from '$lib/stores/toast';
	import {
		getServiceAccount,
		updateServiceAccount,
		deleteServiceAccount,
		listAPIKeys,
		deleteAPIKey
	} from '$lib/serviceaccounts/api';
	import { listRoles } from '$lib/roles/api';
	import type { ServiceAccount, ServiceAccountAPIKey } from '$lib/serviceaccounts/types';
	import type { Role } from '$lib/roles/types';

	const MAX_KEYS = 5;

	let sa = $state<ServiceAccount | null>(null);
	let keys = $state<ServiceAccountAPIKey[]>([]);
	let availableRoles = $state<Role[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let savingDetails = $state(false);
	let showDeleteModal = $state(false);
	let keyToDelete = $state<ServiceAccountAPIKey | null>(null);
	let showDeleteKeyModal = $state(false);

	let name = $state('');
	let description = $state('');
	let active = $state(true);
	let selectedRoleIds = $state<string[]>([]);

	let showNewKeyBanner = $derived($page.url.searchParams.get('created') === '1');

	$effect(() => {
		if ($page.params.id) {
			load($page.params.id);
		}
	});

	async function load(id: string) {
		try {
			loading = true;
			error = null;
			const [account, keyList, roles] = await Promise.all([
				getServiceAccount(id),
				listAPIKeys(id).catch(() => []),
				listRoles().catch(() => [])
			]);
			sa = account;
			keys = keyList;
			availableRoles = roles;
			name = account.name;
			description = account.description ?? '';
			active = account.active;
			selectedRoleIds = account.roles.map((r) => r.id);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load service account';
		} finally {
			loading = false;
		}
	}

	async function saveDetails() {
		if (!sa) return;
		try {
			savingDetails = true;
			const updated = await updateServiceAccount(sa.id, {
				name: name.trim(),
				description: description.trim() || undefined,
				active,
				role_ids: selectedRoleIds
			});
			sa = updated;
			toasts.success('Service account updated');
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : 'Failed to update');
		} finally {
			savingDetails = false;
		}
	}

	async function handleDelete() {
		if (!sa) return;
		try {
			await deleteServiceAccount(sa.id);
			toasts.success(`Service account "${sa.name}" deleted`);
			goto(resolve('/admin?tab=service_accounts'));
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : 'Failed to delete');
		} finally {
			showDeleteModal = false;
		}
	}

	async function handleDeleteKey() {
		if (!sa || !keyToDelete) return;
		try {
			await deleteAPIKey(sa.id, keyToDelete.id);
			toasts.success(`API key "${keyToDelete.name}" deleted`);
			keys = keys.filter((k) => k.id !== keyToDelete?.id);
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : 'Failed to delete key');
		} finally {
			showDeleteKeyModal = false;
			keyToDelete = null;
		}
	}

	function goBack() {
		goto(resolve('/admin?tab=service_accounts'));
	}

	function goNewKey() {
		if (!sa) return;
		goto(resolve(`/service-accounts/${sa.id}/api-keys/new`));
	}

	function formatDate(iso: string | undefined): string {
		if (!iso) return '';
		return iso.slice(0, 10);
	}

	function daysUntil(iso: string | undefined): number | null {
		if (!iso) return null;
		const target = new Date(iso).getTime();
		if (Number.isNaN(target)) return null;
		return Math.ceil((target - Date.now()) / (1000 * 60 * 60 * 24));
	}

	function keyStatus(key: ServiceAccountAPIKey): 'expired' | 'expiring' | 'unused' | 'ok' {
		const days = daysUntil(key.expires_at);
		if (days !== null && days <= 0) return 'expired';
		if (days !== null && days <= 14) return 'expiring';
		if (!key.last_used_at) return 'unused';
		return 'ok';
	}
</script>

<div class="min-h-screen">
	<!-- Header -->
	<div class="border-b border-gray-200 dark:border-gray-700">
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
			<div class="flex items-center justify-between gap-4">
				<div class="flex items-center gap-4 min-w-0">
					<button
						type="button"
						onclick={goBack}
						class="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
						aria-label="Back"
					>
						<IconifyIcon
							icon="material-symbols:arrow-back"
							class="h-6 w-6 text-gray-600 dark:text-gray-400"
						/>
					</button>
					<div class="min-w-0">
						<div class="flex items-center gap-3">
							<IconifyIcon
								icon="material-symbols:smart-toy-outline"
								class="h-6 w-6 text-gray-500 dark:text-gray-400 shrink-0"
							/>
							<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100 truncate">
								{sa?.name ?? 'Service Account'}
							</h1>
							{#if sa}
								<span
									class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
										{sa.active
										? 'bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300'
										: 'bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400'}"
								>
									{sa.active ? 'Active' : 'Inactive'}
								</span>
							{/if}
						</div>
						{#if sa?.description}
							<p class="text-sm text-gray-600 dark:text-gray-400 mt-1 truncate">
								{sa.description}
							</p>
						{/if}
					</div>
				</div>
				{#if sa}
					<Button
						variant="clear"
						icon="material-symbols:delete-outline"
						text="Delete"
						click={() => (showDeleteModal = true)}
					/>
				{/if}
			</div>
		</div>
	</div>

	<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
		{#if loading}
			<div class="flex justify-center py-16">
				<div
					class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-600"
				></div>
			</div>
		{:else if error}
			<div
				class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
			>
				<p class="text-sm text-red-700 dark:text-red-300">{error}</p>
			</div>
		{:else if sa}
			{#if showNewKeyBanner}
				<div
					class="mb-6 flex items-start gap-3 p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg"
				>
					<IconifyIcon
						icon="material-symbols:check-circle"
						class="h-5 w-5 text-green-600 dark:text-green-400 mt-0.5 flex-shrink-0"
					/>
					<div class="flex-1">
						<p class="text-sm font-medium text-green-800 dark:text-green-200">API key created</p>
						<p class="text-xs text-green-700 dark:text-green-300 mt-0.5">
							Make sure you saved the plaintext key — it can't be shown again.
						</p>
					</div>
					<button
						type="button"
						class="text-green-700 dark:text-green-400 hover:text-green-900"
						onclick={() => goto(resolve(`/service-accounts/${sa!.id}`))}
						aria-label="Dismiss"
					>
						<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
					</button>
				</div>
			{/if}

			<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
				<!-- Details -->
				<div class="lg:col-span-2 space-y-6">
					<div
						class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
					>
						<h2
							class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center"
						>
							<IconifyIcon
								icon="material-symbols:info-outline"
								class="h-5 w-5 mr-2 text-earthy-terracotta-600"
							/>
							Details
						</h2>

						<div class="space-y-4">
							<div>
								<label
									for="detail-name"
									class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
									>Name</label
								>
								<input
									id="detail-name"
									type="text"
									bind:value={name}
									class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600"
								/>
							</div>
							<div>
								<label
									for="detail-desc"
									class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
									>Description</label
								>
								<textarea
									id="detail-desc"
									bind:value={description}
									rows="2"
									class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 resize-none"
								></textarea>
							</div>
							<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
								<input type="checkbox" bind:checked={active} class="rounded" />
								Active
							</label>
						</div>
					</div>

					<div
						class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
					>
						<h2
							class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center"
						>
							<IconifyIcon
								icon="material-symbols:shield-outline"
								class="h-5 w-5 mr-2 text-earthy-terracotta-600"
							/>
							Roles
						</h2>

						<RoleSelector
							roles={availableRoles}
							selectedIds={selectedRoleIds}
							onChange={(ids) => (selectedRoleIds = ids)}
							emptyMessage="No roles available."
						/>
					</div>

					<div class="flex justify-end">
						<Button
							variant="filled"
							click={saveDetails}
							text={savingDetails ? 'Saving...' : 'Save changes'}
							disabled={savingDetails || !name.trim()}
							icon="material-symbols:check"
						/>
					</div>
				</div>

				<!-- Right sidebar: API Keys -->
				<div class="lg:col-span-1">
					<div
						class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
					>
						<!-- Header -->
						<div
							class="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700"
						>
							<div class="flex items-center gap-2.5">
								<div
									class="flex items-center justify-center h-8 w-8 rounded-md bg-earthy-terracotta-100/70 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-400"
								>
									<IconifyIcon icon="material-symbols:key-outline" class="h-4 w-4" />
								</div>
								<h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">API Keys</h2>
							</div>
							<div class="flex items-center gap-1.5">
								<span class="text-xs font-medium text-gray-700 dark:text-gray-300">
									{keys.length}
								</span>
								<span class="text-xs text-gray-400">/ {MAX_KEYS}</span>
							</div>
						</div>

						<!-- Progress bar -->
						<div class="h-1 bg-gray-100 dark:bg-gray-800">
							<div
								class="h-full transition-all
									{keys.length >= MAX_KEYS
									? 'bg-amber-500'
									: keys.length >= MAX_KEYS - 1
										? 'bg-earthy-terracotta-500'
										: 'bg-earthy-terracotta-400'}"
								style="width: {(keys.length / MAX_KEYS) * 100}%;"
							></div>
						</div>

						<!-- Keys list -->
						<div class="p-5">
							{#if keys.length === 0}
								<div class="text-center py-6 mb-4">
									<div
										class="inline-flex items-center justify-center h-10 w-10 rounded-full bg-gray-100 dark:bg-gray-900/40 mb-3"
									>
										<IconifyIcon
											icon="material-symbols:key-off-outline"
											class="h-5 w-5 text-gray-400"
										/>
									</div>
									<p class="text-sm font-medium text-gray-700 dark:text-gray-300">
										No API keys yet
									</p>
									<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
										Create a key to authenticate as this account.
									</p>
								</div>
							{:else}
								<ul class="space-y-2 mb-4">
									{#each keys as key (key.id)}
										{@const status = keyStatus(key)}
										{@const daysLeft = daysUntil(key.expires_at)}
										<li
											class="group relative p-3 rounded-lg border transition-all
												{status === 'expired'
												? 'border-red-200 dark:border-red-900/50 bg-red-50/40 dark:bg-red-950/20'
												: status === 'expiring'
													? 'border-amber-200 dark:border-amber-900/50 bg-amber-50/40 dark:bg-amber-950/20'
													: 'border-gray-200 dark:border-gray-700 bg-gray-50/50 dark:bg-gray-900/30 hover:border-gray-300 dark:hover:border-gray-600'}"
										>
											<div class="flex items-start justify-between gap-2">
												<div class="flex items-start gap-2.5 min-w-0 flex-1">
													<IconifyIcon
														icon="material-symbols:vpn-key-outline"
														class="h-4 w-4 mt-0.5 shrink-0
															{status === 'expired'
															? 'text-red-500'
															: status === 'expiring'
																? 'text-amber-500'
																: 'text-gray-400 dark:text-gray-500'}"
													/>
													<div class="min-w-0 flex-1">
														<div
															class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate"
														>
															{key.name}
														</div>
														<div
															class="flex items-center gap-1.5 text-[11px] text-gray-500 dark:text-gray-400 mt-0.5"
														>
															<IconifyIcon
																icon="material-symbols:schedule-outline"
																class="h-3 w-3"
															/>
															<span>Created {formatDate(key.created_at)}</span>
														</div>
													</div>
												</div>
												<button
													type="button"
													class="opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity text-gray-400 hover:text-red-600 dark:hover:text-red-400 p-1 -m-1 shrink-0"
													onclick={() => {
														keyToDelete = key;
														showDeleteKeyModal = true;
													}}
													aria-label="Delete key"
												>
													<IconifyIcon icon="material-symbols:delete-outline" class="h-4 w-4" />
												</button>
											</div>

											<!-- Meta pills -->
											<div class="flex flex-wrap gap-1.5 mt-2 pl-6">
												{#if status === 'expired'}
													<span
														class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300"
													>
														<IconifyIcon
															icon="material-symbols:error-outline"
															class="h-2.5 w-2.5"
														/>
														Expired
													</span>
												{:else if status === 'expiring' && daysLeft !== null}
													<span
														class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-amber-100 dark:bg-amber-900/50 text-amber-800 dark:text-amber-300"
													>
														<IconifyIcon
															icon="material-symbols:warning-outline"
															class="h-2.5 w-2.5"
														/>
														Expires in {daysLeft}d
													</span>
												{:else if key.expires_at}
													<span
														class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400"
													>
														<IconifyIcon
															icon="material-symbols:event-outline"
															class="h-2.5 w-2.5"
														/>
														Expires {formatDate(key.expires_at)}
													</span>
												{:else}
													<span
														class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400"
													>
														<IconifyIcon
															icon="material-symbols:all-inclusive"
															class="h-2.5 w-2.5"
														/>
														No expiry
													</span>
												{/if}

												{#if key.last_used_at}
													<span
														class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-400"
													>
														<IconifyIcon
															icon="material-symbols:check-circle-outline"
															class="h-2.5 w-2.5"
														/>
														Used {formatDate(key.last_used_at)}
													</span>
												{:else}
													<span
														class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-gray-100 dark:bg-gray-800 text-gray-500 dark:text-gray-400"
													>
														<IconifyIcon
															icon="material-symbols:circle-outline"
															class="h-2.5 w-2.5"
														/>
														Unused
													</span>
												{/if}
											</div>
										</li>
									{/each}
								</ul>
							{/if}

							<button
								type="button"
								onclick={goNewKey}
								disabled={keys.length >= MAX_KEYS}
								class="w-full flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg border border-dashed transition-all
									{keys.length >= MAX_KEYS
									? 'border-gray-200 dark:border-gray-700 text-gray-400 dark:text-gray-500 cursor-not-allowed'
									: 'border-earthy-terracotta-300 dark:border-earthy-terracotta-800 text-earthy-terracotta-700 dark:text-earthy-terracotta-400 hover:border-earthy-terracotta-500 hover:bg-earthy-terracotta-50/60 dark:hover:bg-earthy-terracotta-900/20'}"
							>
								<IconifyIcon icon="material-symbols:add" class="h-4 w-4" />
								<span class="text-sm font-medium">Create API Key</span>
							</button>
							{#if keys.length >= MAX_KEYS}
								<div
									class="flex items-start gap-2 mt-3 p-2.5 rounded-md bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800/50"
								>
									<IconifyIcon
										icon="material-symbols:info-outline"
										class="h-4 w-4 text-amber-600 dark:text-amber-400 mt-0.5 shrink-0"
									/>
									<p class="text-xs text-amber-800 dark:text-amber-200">
										Reached the {MAX_KEYS}-key limit. Delete an existing key to add a new one.
									</p>
								</div>
							{/if}
						</div>
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>

{#if sa}
	<DeleteModal
		show={showDeleteModal}
		title="Delete Service Account"
		message="Are you sure you want to delete this service account? All associated API keys will be revoked."
		confirmText="Delete"
		resourceName={sa.name}
		requireConfirmation={true}
		onConfirm={handleDelete}
		onCancel={() => (showDeleteModal = false)}
	/>
{/if}

{#if keyToDelete}
	<DeleteModal
		show={showDeleteKeyModal}
		title="Delete API Key"
		message="Anything using this key will immediately stop working."
		confirmText="Delete"
		resourceName={keyToDelete.name}
		requireConfirmation={false}
		onConfirm={handleDeleteKey}
		onCancel={() => {
			showDeleteKeyModal = false;
			keyToDelete = null;
		}}
	/>
{/if}
