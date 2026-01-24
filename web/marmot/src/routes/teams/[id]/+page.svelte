<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { Users, Lock, ArrowLeft, Shield, Database } from 'lucide-svelte';
	import OwnerSelector from '$components/shared/OwnerSelector.svelte';
	import Icon from '$components/ui/Icon.svelte';
	import IconifyIcon from '@iconify/svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';
	import AssetBlade from '$components/asset/AssetBlade.svelte';
	import Tags from '$components/shared/Tags.svelte';
	import MetadataView from '$components/shared/MetadataView.svelte';
	import type { Team, TeamMember } from '$lib/teams/types';
	import type { Asset } from '$lib/assets/types';
	import {
		NOTIFICATION_TYPE_LABELS,
		NOTIFICATION_TYPE_OPTIONS,
		PROVIDER_OPTIONS,
		PROVIDER_LABELS,
		type TeamWebhook,
		type CreateWebhookInput
	} from '$lib/teams/webhooks';

	interface Owner {
		id: string;
		name: string;
		type: 'user' | 'team';
		username?: string;
		email?: string;
		profile_picture?: string;
	}

	let team: Team | null = null;
	let members: TeamMember[] = [];
	let assets: Asset[] = [];
	let assetsTotal = 0;
	let assetsLimit = 20;
	let assetsOffset = 0;
	let loading = true;
	let loadingAssets = false;
	let error: string | null = null;
	let removingMemberId: string | null = null;
	let ownerSelectorRef: OwnerSelector | null = null;
	let selectedAsset: Asset | null = null;

	// Tab state (derived from URL query string)
	$: activeTab = (
		$page.url.searchParams.get('tab') === 'integrations' ? 'integrations' : 'overview'
	) as 'overview' | 'integrations';

	// Webhook state
	let webhooks: TeamWebhook[] = [];
	let loadingWebhooks = false;
	let showWebhookModal = false;
	let editingWebhook: TeamWebhook | null = null;
	let webhookForm: CreateWebhookInput = {
		name: '',
		provider: 'slack',
		webhook_url: '',
		notification_types: [],
		enabled: true
	};
	let savingWebhook = false;
	let testingWebhookId: string | null = null;
	let webhookError: string | null = null;
	let webhookSuccess: string | null = null;
	let showDeleteConfirm = false;
	let deletingWebhook: TeamWebhook | null = null;
	let showConvertConfirm = false;
	let convertingUserId: string | null = null;

	$: teamId = $page.params.id;
	$: currentUserId = auth.getCurrentUserId();
	$: memberOwners = members.map((m) => ({
		id: m.user_id,
		name: m.name,
		type: 'user' as const,
		username: m.username,
		email: m.email,
		profile_picture: m.profile_picture
	}));
	$: hasMoreAssets = assetsTotal > assetsOffset + assets.length;
	$: currentPage = Math.floor(assetsOffset / assetsLimit) + 1;
	$: totalPages = Math.ceil(assetsTotal / assetsLimit);
	$: canManageTeams = auth.hasPermission('teams', 'manage');
	$: isTeamOwner = members.some((m) => m.user_id === currentUserId && m.role === 'owner');
	$: canEditTeam = canManageTeams || isTeamOwner;

	function getIconType(asset: Asset): string {
		if (asset.providers && Array.isArray(asset.providers) && asset.providers.length === 1) {
			return asset.providers[0];
		}
		return asset.type;
	}

	function getAssetUrl(asset: Asset): string {
		if (!asset.mrn) return '#';
		const mrnParts = asset.mrn.replace('mrn://', '').split('/');
		if (mrnParts.length < 3) return '#';
		const type = mrnParts[0];
		const service = mrnParts[1];
		const fullName = mrnParts.slice(2).join('/');
		return `/discover/${encodeURIComponent(type)}/${encodeURIComponent(service)}/${encodeURIComponent(fullName)}`;
	}

	async function fetchTeam() {
		try {
			loading = true;
			const response = await fetchApi(`/teams/${teamId}`);
			team = await response.json();
			if (!team.tags) team.tags = [];
			if (!team.metadata) team.metadata = {};
		} catch (err) {
			error = err instanceof Error ? err.message : 'An error occurred';
		} finally {
			loading = false;
		}
	}

	async function fetchMembers() {
		try {
			const response = await fetchApi(`/teams/${teamId}/members`);
			const data = await response.json();
			members = data.members;
		} catch (err) {
			error = err instanceof Error ? err.message : 'An error occurred';
		}
	}

	async function fetchAssets(offset = 0) {
		try {
			loadingAssets = true;
			assetsOffset = offset;
			const response = await fetchApi(
				`/assets/search?owner_type=team&owner_id=${teamId}&limit=${assetsLimit}&offset=${offset}`
			);
			const data = await response.json();
			assets = data.assets || [];
			assetsTotal = data.total || 0;
		} catch (err) {
			console.error('Failed to fetch team assets:', err);
			assets = [];
			assetsTotal = 0;
		} finally {
			loadingAssets = false;
		}
	}

	function nextPage() {
		if (hasMoreAssets) {
			fetchAssets(assetsOffset + assetsLimit);
		}
	}

	function previousPage() {
		if (assetsOffset > 0) {
			fetchAssets(Math.max(0, assetsOffset - assetsLimit));
		}
	}

	function convertToManual(userId: string) {
		convertingUserId = userId;
		showConvertConfirm = true;
	}

	async function confirmConvertToManual() {
		if (!convertingUserId) return;
		showConvertConfirm = false;

		try {
			const response = await fetchApi(
				`/teams/${teamId}/members/${convertingUserId}/convert-to-manual`,
				{
					method: 'POST'
				}
			);

			if (response.ok) {
				await fetchMembers();
			} else {
				const errorData = await response.json();
				console.error('Failed to convert member to manual:', errorData);
			}
		} catch (err) {
			console.error('Failed to convert member to manual:', err);
		} finally {
			convertingUserId = null;
		}
	}

	async function updateMemberRole(userId: string, newRole: string) {
		try {
			const response = await fetchApi(`/teams/${teamId}/members/${userId}/role`, {
				method: 'PUT',
				body: JSON.stringify({ role: newRole })
			});

			if (response.ok) {
				await fetchMembers();
			} else {
				const errorData = await response.json();
				console.error('Failed to update member role:', errorData);
			}
		} catch (err) {
			console.error('Failed to update member role:', err);
		}
	}

	async function handleAddMembersClick(event: MouseEvent) {
		event.stopPropagation();
		if (ownerSelectorRef) {
			ownerSelectorRef.open();
		}
	}

	async function handleMembersChange(newOwners: Owner[]) {
		if (team.created_via_sso || newOwners.length === 0) return;

		try {
			const currentMemberIds = new Set(members.map((m) => m.user_id));
			const ownersToAdd = newOwners.filter((owner) => !currentMemberIds.has(owner.id));

			for (const owner of ownersToAdd) {
				const response = await fetchApi(`/teams/${teamId}/members`, {
					method: 'POST',
					body: JSON.stringify({
						user_id: owner.id,
						role: 'member'
					})
				});
				if (!response.ok) {
					const errorData = await response.json();
					console.error('Failed to add team member:', errorData);
					await fetchMembers();
					return;
				}
			}

			await fetchMembers();
		} catch (err) {
			console.error('Failed to add team members:', err);
			await fetchMembers();
		}
	}

	async function removeMemberDirect(userId: string, source: string) {
		try {
			removingMemberId = userId;
			const response = await fetchApi(`/teams/${teamId}/members/${userId}`, {
				method: 'DELETE'
			});

			if (response.ok) {
				await fetchMembers();
			} else {
				const errorData = await response.json();
				console.error('Failed to remove team member:', errorData);
			}
		} catch (err) {
			console.error('Failed to remove team member:', err);
		} finally {
			removingMemberId = null;
		}
	}

	function handleAssetClick(e: Event, asset: Asset) {
		e.preventDefault();
		selectedAsset = asset;
	}

	// Webhook functions
	async function fetchWebhooks() {
		try {
			loadingWebhooks = true;
			const response = await fetchApi(`/teams/${teamId}/webhooks`);
			if (response.ok) {
				const data = await response.json();
				webhooks = data.webhooks || [];
			} else if (response.status !== 403) {
				console.error('Failed to fetch webhooks');
			}
		} catch (err) {
			console.error('Failed to fetch webhooks:', err);
		} finally {
			loadingWebhooks = false;
		}
	}

	function openAddWebhook() {
		editingWebhook = null;
		webhookError = null;
		webhookSuccess = null;
		webhookForm = {
			name: '',
			provider: 'slack',
			webhook_url: '',
			notification_types: [],
			enabled: true
		};
		showWebhookModal = true;
	}

	async function openEditWebhook(webhook: TeamWebhook) {
		try {
			const response = await fetchApi(`/teams/${teamId}/webhooks/${webhook.id}`);
			const fullWebhook: TeamWebhook = await response.json();
			editingWebhook = fullWebhook;
			webhookForm = {
				name: fullWebhook.name,
				provider: fullWebhook.provider,
				webhook_url: fullWebhook.webhook_url,
				notification_types: [...fullWebhook.notification_types],
				enabled: fullWebhook.enabled
			};
			showWebhookModal = true;
		} catch (err) {
			console.error('Failed to fetch webhook details:', err);
		}
	}

	function closeWebhookModal() {
		showWebhookModal = false;
		editingWebhook = null;
		webhookError = null;
	}

	function toggleNotificationType(type: string) {
		if (webhookForm.notification_types.includes(type)) {
			webhookForm.notification_types = webhookForm.notification_types.filter((t) => t !== type);
		} else {
			webhookForm.notification_types = [...webhookForm.notification_types, type];
		}
	}

	async function saveWebhook() {
		if (!webhookForm.name || !webhookForm.webhook_url || !webhookForm.notification_types.length)
			return;

		webhookError = null;

		try {
			savingWebhook = true;

			if (editingWebhook) {
				const response = await fetchApi(`/teams/${teamId}/webhooks/${editingWebhook.id}`, {
					method: 'PUT',
					body: JSON.stringify({
						name: webhookForm.name,
						webhook_url: webhookForm.webhook_url,
						notification_types: webhookForm.notification_types,
						enabled: webhookForm.enabled
					})
				});
				if (!response.ok) {
					const errData = await response.json();
					webhookError = errData.error || 'Failed to update webhook';
					return;
				}
			} else {
				const response = await fetchApi(`/teams/${teamId}/webhooks`, {
					method: 'POST',
					body: JSON.stringify(webhookForm)
				});
				if (!response.ok) {
					const errData = await response.json();
					webhookError = errData.error || 'Failed to create webhook';
					return;
				}
			}

			closeWebhookModal();
			await fetchWebhooks();
		} catch (err) {
			console.error('Failed to save webhook:', err);
			webhookError = 'An error occurred while saving the webhook';
		} finally {
			savingWebhook = false;
		}
	}

	function deleteWebhook(webhook: TeamWebhook) {
		deletingWebhook = webhook;
		showDeleteConfirm = true;
	}

	async function confirmDeleteWebhook() {
		if (!deletingWebhook) return;
		showDeleteConfirm = false;

		try {
			const response = await fetchApi(`/teams/${teamId}/webhooks/${deletingWebhook.id}`, {
				method: 'DELETE'
			});
			if (response.ok || response.status === 204) {
				await fetchWebhooks();
			} else {
				const errData = await response.json();
				webhookError = errData.error || 'Failed to delete webhook';
			}
		} catch (err) {
			console.error('Failed to delete webhook:', err);
		} finally {
			deletingWebhook = null;
		}
	}

	async function toggleWebhookEnabled(webhook: TeamWebhook) {
		try {
			const response = await fetchApi(`/teams/${teamId}/webhooks/${webhook.id}`, {
				method: 'PUT',
				body: JSON.stringify({ enabled: !webhook.enabled })
			});
			if (response.ok) {
				await fetchWebhooks();
			}
		} catch (err) {
			console.error('Failed to toggle webhook:', err);
		}
	}

	async function testWebhook(webhook: TeamWebhook) {
		webhookError = null;
		webhookSuccess = null;

		try {
			testingWebhookId = webhook.id;
			const response = await fetchApi(`/teams/${teamId}/webhooks/${webhook.id}/test`, {
				method: 'POST'
			});
			if (response.ok) {
				webhookSuccess = 'Test notification sent successfully';
				setTimeout(() => {
					webhookSuccess = null;
				}, 5000);
			} else {
				const errData = await response.json();
				webhookError = errData.error || 'Failed to send test notification';
			}
		} catch (err) {
			console.error('Failed to test webhook:', err);
			webhookError = 'Failed to send test notification';
		} finally {
			testingWebhookId = null;
		}
	}

	onMount(() => {
		fetchTeam();
		fetchMembers();
		fetchAssets();
		fetchWebhooks();
	});
</script>

<div class="container max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
	<!-- Back Button -->
	<div class="mb-6">
		<button
			onclick={() => window.history.back()}
			class="inline-flex items-center text-sm text-gray-600 dark:text-gray-400 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-500"
		>
			<ArrowLeft class="h-4 w-4 mr-1" />
			Back
		</button>
	</div>

	{#if loading}
		<div class="flex justify-center p-8">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"></div>
		</div>
	{:else if error}
		<div class="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
			{error}
		</div>
	{:else if team}
		<!-- Team Header -->
		<div
			class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 mb-6"
		>
			<div class="flex items-start gap-4">
				<div class="p-3 bg-blue-100 dark:bg-blue-900 rounded-lg">
					<Users class="h-8 w-8 text-blue-700 dark:text-blue-300" />
				</div>
				<div class="flex-1">
					<div class="flex items-center gap-3 mb-2">
						<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">
							{team.name}
						</h1>
						{#if team.created_via_sso}
							<span
								class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
							>
								<Lock class="h-3 w-3 mr-1" />
								SSO Managed
							</span>
						{/if}
					</div>
					{#if team.description}
						<p class="text-gray-600 dark:text-gray-400">
							{team.description}
						</p>
					{/if}
					{#if team.sso_provider}
						<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
							Provider: {team.sso_provider}
						</p>
					{/if}
				</div>
			</div>
		</div>

		<!-- Tab Bar -->
		<div class="border-b border-gray-200 dark:border-gray-700 mb-6">
			<nav class="flex gap-6" aria-label="Team tabs">
				<button
					onclick={() => goto(`?tab=overview`, { replaceState: true })}
					class="pb-3 text-sm font-medium border-b-2 transition-colors {activeTab === 'overview'
						? 'border-earthy-terracotta-700 text-earthy-terracotta-700 dark:border-earthy-terracotta-500 dark:text-earthy-terracotta-500'
						: 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300'}"
				>
					Overview
				</button>
				{#if canEditTeam}
					<button
						onclick={() => goto(`?tab=integrations`, { replaceState: true })}
						class="pb-3 text-sm font-medium border-b-2 transition-colors {activeTab ===
						'integrations'
							? 'border-earthy-terracotta-700 text-earthy-terracotta-700 dark:border-earthy-terracotta-500 dark:text-earthy-terracotta-500'
							: 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300'}"
					>
						<span class="inline-flex items-center gap-1.5">
							<IconifyIcon icon="material-symbols:webhook" class="w-4 h-4" />
							Integrations
							{#if webhooks.length > 0}
								<span
									class="inline-flex items-center justify-center w-5 h-5 text-[10px] font-medium rounded-full bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300"
								>
									{webhooks.length}
								</span>
							{/if}
						</span>
					</button>
				{/if}
			</nav>
		</div>

		<!-- Overview Tab -->
		{#if activeTab === 'overview'}
			<!-- Tags & Metadata Section -->
			<div
				class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 mb-6"
			>
				<div class="space-y-5">
					<!-- Tags -->
					<div>
						<div class="flex items-center gap-2 mb-2">
							<IconifyIcon
								icon="material-symbols:label-outline"
								class="w-4 h-4 text-gray-500 dark:text-gray-400"
							/>
							<h3
								class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider"
							>
								Tags
							</h3>
						</div>
						<Tags
							tags={team.tags ?? []}
							endpoint="/teams"
							id={team.id}
							canEdit={canEditTeam && !team.created_via_sso}
						/>
					</div>

					<!-- Metadata -->
					<div>
						<div class="flex items-center gap-2 mb-2">
							<IconifyIcon
								icon="material-symbols:database-outline"
								class="w-4 h-4 text-gray-500 dark:text-gray-400"
							/>
							<h3
								class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider"
							>
								Metadata
							</h3>
						</div>
						<MetadataView
							bind:metadata={team.metadata}
							endpoint="/teams"
							id={team.id}
							permissionResource="teams"
							permissionAction="manage"
							readOnly={team.created_via_sso}
							maxDepth={2}
						/>
					</div>
				</div>
			</div>

			<!-- Members Section -->
			<div
				class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 mb-6"
			>
				<div class="flex items-center justify-between mb-4">
					<div class="flex items-center gap-2">
						<IconifyIcon
							icon="material-symbols:group"
							class="w-5 h-5 text-gray-500 dark:text-gray-400"
						/>
						<h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">Members</h2>
						<span class="text-sm text-gray-500 dark:text-gray-400">
							({members.length})
						</span>
					</div>
					{#if !team.created_via_sso && canEditTeam}
						<button
							onclick={handleAddMembersClick}
							class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-earthy-terracotta-700 dark:text-earthy-terracotta-500 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 hover:bg-earthy-terracotta-100 dark:hover:bg-earthy-terracotta-900/30 rounded-lg transition-colors"
						>
							<IconifyIcon icon="material-symbols:add" class="w-4 h-4" />
							Add Member
						</button>
					{:else if team.created_via_sso}
						<span class="text-sm text-gray-500 dark:text-gray-400">
							Members are managed via SSO
						</span>
					{/if}
				</div>

				<!-- Hidden OwnerSelector for adding members (always starts empty) -->
				<div class="relative w-full mb-4" style="min-height: 1px;">
					<OwnerSelector
						bind:this={ownerSelectorRef}
						selectedOwners={members.map((m) => ({
							id: m.user_id,
							name: m.name,
							type: 'user' as const,
							username: m.provider_username,
							email: m.provider_email,
							profile_picture: m.profile_picture
						}))}
						onChange={handleMembersChange}
						userOnly={true}
						hideAddButton={true}
						hideSelectedOwners={true}
						placeholder="Search and add members..."
					/>
				</div>

				{#if members.length === 0}
					<div class="text-center py-8 text-gray-500 dark:text-gray-400">
						No members in this team yet
					</div>
				{:else}
					<div class="space-y-1">
						{#each members as member (member.user_id)}
							<div
								class="flex items-center justify-between p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors group"
							>
								<div class="flex items-center gap-3 flex-1 min-w-0">
									<div
										class="w-9 h-9 rounded-full bg-gradient-to-br from-earthy-terracotta-400 to-earthy-terracotta-600 flex items-center justify-center text-white text-sm font-semibold flex-shrink-0"
									>
										{member.name.charAt(0).toUpperCase()}
									</div>
									<div class="flex-1 min-w-0">
										<div class="text-sm font-medium text-gray-900 dark:text-gray-100">
											{member.name}
										</div>
										{#if member.email}
											<div class="text-xs text-gray-500 dark:text-gray-400 truncate">
												{member.email}
											</div>
										{/if}
									</div>
								</div>

								<div class="flex items-center gap-2 flex-shrink-0">
									<!-- Role Badge/Selector -->
									{#if member.source === 'manual' && !team.created_via_sso && canEditTeam && currentUserId !== member.user_id}
										<select
											value={member.role}
											onchange={(e) => updateMemberRole(member.user_id, e.currentTarget.value)}
											class="text-xs border border-gray-300 dark:border-gray-600 rounded-md px-2 py-1 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-1 focus:ring-earthy-terracotta-500"
										>
											<option value="member">Member</option>
											<option value="owner">Owner</option>
										</select>
									{:else}
										<span
											class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium {member.role ===
											'owner'
												? 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
												: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'}"
										>
											{#if member.role === 'owner'}
												<Shield class="h-3 w-3 mr-1" />
											{/if}
											{member.role}
										</span>
									{/if}

									<!-- Source Badge -->
									{#if member.source === 'sso'}
										<span
											class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
										>
											<Lock class="h-3 w-3 mr-1" />
											SSO
										</span>
									{/if}

									<!-- Actions -->
									{#if canEditTeam}
										<div
											class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity"
										>
											{#if member.source === 'sso' && currentUserId !== member.user_id}
												<button
													onclick={() => convertToManual(member.user_id)}
													class="text-xs text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300 px-2 py-1.5 rounded hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
													title="Convert to manual"
												>
													Make Permanent
												</button>
											{/if}
											{#if (member.source === 'manual' || !team.created_via_sso) && (currentUserId === member.user_id || canEditTeam)}
												<button
													onclick={() => removeMemberDirect(member.user_id, member.source)}
													disabled={removingMemberId === member.user_id}
													class="p-1.5 text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded disabled:opacity-50 transition-colors"
													title={currentUserId === member.user_id ? 'Leave team' : 'Remove member'}
												>
													<IconifyIcon icon="material-symbols:close" class="w-4 h-4" />
												</button>
											{/if}
										</div>
									{:else if currentUserId === member.user_id}
										<!-- Allow users to leave the team even without edit permissions -->
										<div
											class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity"
										>
											<button
												onclick={() => removeMemberDirect(member.user_id, member.source)}
												disabled={removingMemberId === member.user_id}
												class="p-1.5 text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded disabled:opacity-50 transition-colors"
												title="Leave team"
											>
												<IconifyIcon icon="material-symbols:close" class="w-4 h-4" />
											</button>
										</div>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Assets Section -->
			<div
				class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6"
			>
				<div class="flex items-center justify-between mb-4">
					<div class="flex items-center gap-2">
						<Database class="w-5 h-5 text-gray-500 dark:text-gray-400" />
						<h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">Assets Owned</h2>
						<span class="text-sm text-gray-500 dark:text-gray-400">
							({assetsTotal})
						</span>
					</div>
					{#if assetsTotal > assetsLimit}
						<div class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
							<button
								onclick={previousPage}
								disabled={assetsOffset === 0 || loadingAssets}
								class="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
								title="Previous page"
							>
								<IconifyIcon icon="material-symbols:chevron-left" class="w-5 h-5" />
							</button>
							<span class="min-w-[80px] text-center">
								Page {currentPage} of {totalPages}
							</span>
							<button
								onclick={nextPage}
								disabled={!hasMoreAssets || loadingAssets}
								class="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
								title="Next page"
							>
								<IconifyIcon icon="material-symbols:chevron-right" class="w-5 h-5" />
							</button>
						</div>
					{/if}
				</div>

				<div>
					{#if loadingAssets}
						<div class="flex justify-center py-8">
							<div
								class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"
							></div>
						</div>
					{:else if assets.length === 0}
						<div class="text-center py-8 text-gray-500 dark:text-gray-400">
							No assets owned by this team yet
						</div>
					{:else}
						<div class="space-y-1">
							{#each assets as asset (asset.id)}
								<a
									href={getAssetUrl(asset)}
									onclick={(e) => handleAssetClick(e, asset)}
									class="flex items-center justify-between p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors group"
								>
									<div class="flex items-center gap-3 flex-1 min-w-0">
										<div class="flex-shrink-0">
											<Icon name={getIconType(asset)} showLabel={false} size="sm" />
										</div>
										<div class="flex-1 min-w-0">
											<div
												class="text-sm font-medium text-gray-900 dark:text-gray-100 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-500 truncate"
											>
												{asset.name}
											</div>
											<div class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono">
												{asset.mrn}
											</div>
										</div>
									</div>
									<span
										class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200"
									>
										{asset.type}
									</span>
								</a>
							{/each}
						</div>
					{/if}
				</div>
			</div>
		{/if}

		<!-- Integrations Tab -->
		{#if activeTab === 'integrations'}
			<div
				class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6"
			>
				{#if showWebhookModal}
					<!-- Inline Add/Edit Form -->
					<div class="flex items-center gap-3 mb-6">
						<button
							onclick={closeWebhookModal}
							class="p-1.5 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
							title="Back to list"
						>
							<IconifyIcon icon="material-symbols:arrow-back" class="w-5 h-5" />
						</button>
						<h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">
							{editingWebhook ? 'Edit Webhook' : 'Add Webhook'}
						</h2>
					</div>

					<div class="grid grid-cols-1 md:grid-cols-2 gap-5">
						<!-- Name -->
						<div>
							<label
								for="webhook-name"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								Name
							</label>
							<input
								id="webhook-name"
								type="text"
								bind:value={webhookForm.name}
								placeholder="e.g., Schema Changes to Slack"
								class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 text-sm focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
							/>
						</div>

						<!-- Provider -->
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								Provider
							</label>
							<div class="grid grid-cols-3 gap-2">
								{#each PROVIDER_OPTIONS as provider}
									<button
										type="button"
										onclick={() => (webhookForm.provider = provider.value)}
										class="flex items-center justify-center gap-2 px-3 py-2.5 rounded-lg border-2 transition-all {webhookForm.provider ===
										provider.value
											? 'border-earthy-terracotta-500 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 text-earthy-terracotta-700 dark:text-earthy-terracotta-400'
											: 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500 text-gray-600 dark:text-gray-400'}"
									>
										<IconifyIcon icon={provider.icon} class="w-4 h-4" />
										<span class="text-sm font-medium">{provider.label}</span>
									</button>
								{/each}
							</div>
						</div>

						<!-- Webhook URL (full width) -->
						<div class="md:col-span-2">
							<label
								for="webhook-url"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								Webhook URL
							</label>
							<input
								id="webhook-url"
								type="url"
								bind:value={webhookForm.webhook_url}
								placeholder="https://hooks.slack.com/services/..."
								class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 text-sm font-mono focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
							/>
						</div>

						<!-- Notification Types (full width) -->
						<fieldset class="md:col-span-2">
							<legend class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								Notification Types
							</legend>
							<div class="grid grid-cols-2 md:grid-cols-3 gap-2">
								{#each NOTIFICATION_TYPE_OPTIONS as { type, label, icon }}
									<label
										class="flex items-center gap-2.5 px-3 py-2.5 rounded-lg border cursor-pointer transition-all {webhookForm.notification_types.includes(
											type
										)
											? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 border-earthy-terracotta-300 dark:border-earthy-terracotta-700'
											: 'border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700/50'}"
									>
										<input
											type="checkbox"
											checked={webhookForm.notification_types.includes(type)}
											onchange={() => toggleNotificationType(type)}
											class="h-4 w-4 rounded border-gray-300 text-earthy-terracotta-700 focus:ring-earthy-terracotta-600"
										/>
										<IconifyIcon
											{icon}
											class="w-4 h-4 flex-shrink-0 {webhookForm.notification_types.includes(type)
												? 'text-earthy-terracotta-600 dark:text-earthy-terracotta-400'
												: 'text-gray-400 dark:text-gray-500'}"
										/>
										<span class="text-sm text-gray-700 dark:text-gray-300">
											{label}
										</span>
									</label>
								{/each}
							</div>
						</fieldset>

						<!-- Enabled Toggle (full width) -->
						<div class="md:col-span-2 flex items-center justify-between py-1">
							<div>
								<span class="text-sm font-medium text-gray-700 dark:text-gray-300"> Enabled </span>
								<p class="text-xs text-gray-500 dark:text-gray-400">
									Webhook will receive notifications when enabled
								</p>
							</div>
							<button
								type="button"
								onclick={() => (webhookForm.enabled = !webhookForm.enabled)}
								class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors {webhookForm.enabled
									? 'bg-earthy-terracotta-600'
									: 'bg-gray-300 dark:bg-gray-600'}"
								role="switch"
								aria-checked={webhookForm.enabled}
							>
								<span
									class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {webhookForm.enabled
										? 'translate-x-6'
										: 'translate-x-1'}"
								></span>
							</button>
						</div>

						<!-- Error message -->
						{#if webhookError}
							<div
								class="md:col-span-2 flex items-center gap-2 px-4 py-3 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800"
							>
								<IconifyIcon
									icon="material-symbols:error-outline"
									class="w-4 h-4 text-red-600 dark:text-red-400 flex-shrink-0"
								/>
								<span class="text-sm text-red-700 dark:text-red-300">{webhookError}</span>
								<button
									onclick={() => (webhookError = null)}
									class="ml-auto text-red-400 hover:text-red-600 dark:hover:text-red-300"
								>
									<IconifyIcon icon="material-symbols:close" class="w-4 h-4" />
								</button>
							</div>
						{/if}

						<!-- Actions (full width) -->
						<div
							class="md:col-span-2 flex items-center gap-3 pt-4 border-t border-gray-200 dark:border-gray-700"
						>
							<button
								onclick={saveWebhook}
								disabled={savingWebhook ||
									!webhookForm.name ||
									!webhookForm.webhook_url ||
									!webhookForm.notification_types.length}
								class="px-5 py-2.5 text-sm font-medium text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
							>
								{#if savingWebhook}
									Saving...
								{:else}
									{editingWebhook ? 'Update Webhook' : 'Create Webhook'}
								{/if}
							</button>
							<button
								onclick={closeWebhookModal}
								class="px-5 py-2.5 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
							>
								Cancel
							</button>
						</div>
					</div>
				{:else}
					<!-- Webhook List -->
					<div class="flex items-center justify-between mb-4">
						<div class="flex items-center gap-2">
							<IconifyIcon
								icon="material-symbols:webhook"
								class="w-5 h-5 text-gray-500 dark:text-gray-400"
							/>
							<h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">Webhooks</h2>
							<span class="text-sm text-gray-500 dark:text-gray-400">
								({webhooks.length})
							</span>
						</div>
						<button
							onclick={openAddWebhook}
							class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-earthy-terracotta-700 dark:text-earthy-terracotta-500 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 hover:bg-earthy-terracotta-100 dark:hover:bg-earthy-terracotta-900/30 rounded-lg transition-colors"
						>
							<IconifyIcon icon="material-symbols:add" class="w-4 h-4" />
							Add Webhook
						</button>
					</div>

					<p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
						Configure webhooks to receive team notifications in external services like Slack or
						Discord.
					</p>

					{#if webhookSuccess}
						<div
							class="flex items-center gap-2 px-4 py-3 rounded-lg bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 mb-4"
						>
							<IconifyIcon
								icon="material-symbols:check-circle-outline"
								class="w-4 h-4 text-green-600 dark:text-green-400 flex-shrink-0"
							/>
							<span class="text-sm text-green-700 dark:text-green-300">{webhookSuccess}</span>
							<button
								onclick={() => (webhookSuccess = null)}
								class="ml-auto text-green-400 hover:text-green-600 dark:hover:text-green-300"
							>
								<IconifyIcon icon="material-symbols:close" class="w-4 h-4" />
							</button>
						</div>
					{/if}

					{#if webhookError}
						<div
							class="flex items-center gap-2 px-4 py-3 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 mb-4"
						>
							<IconifyIcon
								icon="material-symbols:error-outline"
								class="w-4 h-4 text-red-600 dark:text-red-400 flex-shrink-0"
							/>
							<span class="text-sm text-red-700 dark:text-red-300">{webhookError}</span>
							<button
								onclick={() => (webhookError = null)}
								class="ml-auto text-red-400 hover:text-red-600 dark:hover:text-red-300"
							>
								<IconifyIcon icon="material-symbols:close" class="w-4 h-4" />
							</button>
						</div>
					{/if}

					{#if loadingWebhooks}
						<div class="flex justify-center py-6">
							<div
								class="animate-spin rounded-full h-6 w-6 border-b-2 border-earthy-terracotta-700"
							></div>
						</div>
					{:else if webhooks.length === 0}
						<div class="text-center py-12 text-gray-500 dark:text-gray-400">
							<IconifyIcon
								icon="material-symbols:webhook"
								class="w-10 h-10 mx-auto mb-3 opacity-40"
							/>
							<p class="text-sm font-medium">No webhooks configured</p>
							<p class="text-xs mt-1 max-w-sm mx-auto">
								Add a webhook to send notifications to Slack, Discord, or other services when events
								happen for this team.
							</p>
						</div>
					{:else}
						<div class="space-y-2">
							{#each webhooks as webhook (webhook.id)}
								<div
									class="flex items-center justify-between p-4 rounded-lg border border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors group"
								>
									<div class="flex items-center gap-3 flex-1 min-w-0">
										<div
											class="flex-shrink-0 w-9 h-9 rounded-lg bg-gray-100 dark:bg-gray-700 flex items-center justify-center"
										>
											<IconifyIcon
												icon={PROVIDER_OPTIONS.find((p) => p.value === webhook.provider)?.icon ||
													'mdi:webhook'}
												class="w-5 h-5 text-gray-600 dark:text-gray-300"
											/>
										</div>
										<div class="flex-1 min-w-0">
											<div class="flex items-center gap-2">
												<span class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
													{webhook.name}
												</span>
												<span
													class="inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium {webhook.enabled
														? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300'
														: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'}"
												>
													{webhook.enabled ? 'Active' : 'Disabled'}
												</span>
												<span class="text-xs text-gray-400 dark:text-gray-500">
													{PROVIDER_LABELS[webhook.provider] || webhook.provider}
												</span>
											</div>
											<div class="flex items-center gap-1 mt-1.5 flex-wrap">
												{#each webhook.notification_types as type}
													<span
														class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300"
													>
														{NOTIFICATION_TYPE_LABELS[type] || type}
													</span>
												{/each}
											</div>
										</div>
									</div>

									<div class="flex items-center gap-1 flex-shrink-0">
										<!-- Last triggered info -->
										<div class="text-xs text-gray-400 dark:text-gray-500 mr-2 group-hover:hidden">
											{#if webhook.last_error}
												<span class="text-red-500" title={webhook.last_error}>Error</span>
											{:else if webhook.last_triggered_at}
												Last: {new Date(webhook.last_triggered_at).toLocaleDateString()}
											{:else}
												Never triggered
											{/if}
										</div>

										<!-- Actions (shown on hover) -->
										<div class="hidden group-hover:flex items-center gap-1">
											<button
												onclick={() => testWebhook(webhook)}
												disabled={testingWebhookId === webhook.id || !webhook.enabled}
												class="p-1.5 text-gray-500 hover:text-blue-600 dark:text-gray-400 dark:hover:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded transition-colors disabled:opacity-50"
												title="Send test notification"
											>
												{#if testingWebhookId === webhook.id}
													<div
														class="w-4 h-4 animate-spin rounded-full border-2 border-gray-300 border-t-blue-600"
													></div>
												{:else}
													<IconifyIcon icon="material-symbols:send-outline" class="w-4 h-4" />
												{/if}
											</button>
											<button
												onclick={() => toggleWebhookEnabled(webhook)}
												class="p-1.5 text-gray-500 hover:text-yellow-600 dark:text-gray-400 dark:hover:text-yellow-400 hover:bg-yellow-50 dark:hover:bg-yellow-900/20 rounded transition-colors"
												title={webhook.enabled ? 'Disable webhook' : 'Enable webhook'}
											>
												<IconifyIcon
													icon={webhook.enabled
														? 'material-symbols:toggle-on'
														: 'material-symbols:toggle-off'}
													class="w-5 h-5"
												/>
											</button>
											<button
												onclick={() => openEditWebhook(webhook)}
												class="p-1.5 text-gray-500 hover:text-earthy-terracotta-700 dark:text-gray-400 dark:hover:text-earthy-terracotta-500 hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/20 rounded transition-colors"
												title="Edit webhook"
											>
												<IconifyIcon icon="material-symbols:edit-outline" class="w-4 h-4" />
											</button>
											<button
												onclick={() => deleteWebhook(webhook)}
												class="p-1.5 text-gray-500 hover:text-red-600 dark:text-gray-400 dark:hover:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
												title="Delete webhook"
											>
												<IconifyIcon icon="material-symbols:delete-outline" class="w-4 h-4" />
											</button>
										</div>
									</div>
								</div>
							{/each}
						</div>
					{/if}
				{/if}
			</div>
		{/if}
	{/if}
</div>

{#if selectedAsset}
	<AssetBlade asset={selectedAsset} onClose={() => (selectedAsset = null)} />
{/if}

<ConfirmModal
	bind:show={showDeleteConfirm}
	title="Delete Webhook"
	message={`Are you sure you want to delete "${deletingWebhook?.name}"? This action cannot be undone.`}
	confirmText="Delete"
	variant="danger"
	onConfirm={confirmDeleteWebhook}
	onCancel={() => {
		showDeleteConfirm = false;
		deletingWebhook = null;
	}}
/>

<ConfirmModal
	bind:show={showConvertConfirm}
	title="Convert to Manual"
	message="Convert this member to manual? They will no longer be managed by SSO."
	confirmText="Convert"
	variant="warning"
	onConfirm={confirmConvertToManual}
	onCancel={() => {
		showConvertConfirm = false;
		convertingUserId = null;
	}}
/>
