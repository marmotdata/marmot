<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { Users, Lock, ArrowLeft, Shield, Database } from 'lucide-svelte';
	import OwnerSelector from '../../../components/OwnerSelector.svelte';
	import Icon from '../../../components/Icon.svelte';
	import IconifyIcon from '@iconify/svelte';
	import AssetBlade from '../../../components/AssetBlade.svelte';

	interface Owner {
		id: string;
		name: string;
		type: 'user' | 'team';
		username?: string;
		email?: string;
	}

	let team: any = null;
	let members: any[] = [];
	let assets: any[] = [];
	let assetsTotal = 0;
	let assetsLimit = 20;
	let assetsOffset = 0;
	let loading = true;
	let loadingAssets = false;
	let error: string | null = null;
	let removingMemberId: string | null = null;
	let ownerSelectorRef: any;
	let selectedAsset: any = null;

	$: teamId = $page.params.id;
	$: currentUserId = auth.getCurrentUserId();
	$: memberOwners = members.map(m => ({
		id: m.user_id,
		name: m.name,
		type: 'user' as const,
		username: m.username,
		email: m.email
	}));
	$: hasMoreAssets = assetsTotal > assetsOffset + assets.length;
	$: currentPage = Math.floor(assetsOffset / assetsLimit) + 1;
	$: totalPages = Math.ceil(assetsTotal / assetsLimit);

	function getIconType(asset: any): string {
		if (asset.providers && Array.isArray(asset.providers) && asset.providers.length === 1) {
			return asset.providers[0];
		}
		return asset.type;
	}

	async function fetchTeam() {
		try {
			loading = true;
			const response = await fetchApi(`/teams/${teamId}`);
			team = await response.json();
		} catch (err: any) {
			error = err.message;
		} finally {
			loading = false;
		}
	}

	async function fetchMembers() {
		try {
			const response = await fetchApi(`/teams/${teamId}/members`);
			const data = await response.json();
			members = data.members;
		} catch (err: any) {
			error = err.message;
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
		} catch (err: any) {
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


	async function convertToManual(userId: string) {
		if (!confirm('Convert this member to manual? They will no longer be managed by SSO.')) {
			return;
		}

		try {
			const response = await fetchApi(`/teams/${teamId}/members/${userId}/convert-to-manual`, {
				method: 'POST'
			});

			if (response.ok) {
				await fetchMembers();
			} else {
				const data = await response.json();
			}
		} catch (err: any) {
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
				const data = await response.json();
			}
		} catch (err: any) {
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
			// Find only the newly added members (ones not in current members list)
			const currentMemberIds = new Set(members.map(m => m.user_id));
			const ownersToAdd = newOwners.filter(owner => !currentMemberIds.has(owner.id));

			// Add only the new members
			for (const owner of ownersToAdd) {
				const response = await fetchApi(`/teams/${teamId}/members`, {
					method: 'POST',
					body: JSON.stringify({
						user_id: owner.id,
						role: 'member'
					})
				});
				if (!response.ok) {
					const data = await response.json();
					await fetchMembers();
					return;
				}
			}

			await fetchMembers();
		} catch (err: any) {
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
				const data = await response.json();
			}
		} catch (err: any) {
		} finally {
			removingMemberId = null;
		}
	}

	function handleAssetClick(e: Event, asset: any) {
		e.preventDefault();
		selectedAsset = asset;
	}

	onMount(() => {
		fetchTeam();
		fetchMembers();
		fetchAssets();
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
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700" />
		</div>
	{:else if error}
		<div class="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
			{error}
		</div>
	{:else if team}
		<!-- Team Header -->
		<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 mb-6">
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
							<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
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

		<!-- Members Section -->
		<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 mb-6">
			<div class="flex items-center justify-between mb-4">
				<div class="flex items-center gap-2">
					<IconifyIcon icon="material-symbols:group" class="w-5 h-5 text-gray-500 dark:text-gray-400" />
					<h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">
						Members
					</h2>
					<span class="text-sm text-gray-500 dark:text-gray-400">
						({members.length})
					</span>
				</div>
				{#if !team.created_via_sso}
					<button
						onclick={handleAddMembersClick}
						class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-earthy-terracotta-700 dark:text-earthy-terracotta-500 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 hover:bg-earthy-terracotta-100 dark:hover:bg-earthy-terracotta-900/30 rounded-lg transition-colors"
					>
						<IconifyIcon icon="material-symbols:add" class="w-4 h-4" />
						Add Member
					</button>
				{:else}
					<span class="text-sm text-gray-500 dark:text-gray-400">
						Members are managed via SSO
					</span>
				{/if}
			</div>

			<!-- Hidden OwnerSelector for adding members (always starts empty) -->
			<div class="relative w-full mb-4" style="min-height: 1px;">
				<OwnerSelector
					bind:this={ownerSelectorRef}
					selectedOwners={members.map(m => ({ id: m.user_id, name: m.name, type: 'user' as const, username: m.provider_username, email: m.provider_email }))}
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
					{#each members as member}
						<div class="flex items-center justify-between p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors group">
							<div class="flex items-center gap-3 flex-1 min-w-0">
								<div class="w-9 h-9 rounded-full bg-gradient-to-br from-earthy-terracotta-400 to-earthy-terracotta-600 flex items-center justify-center text-white text-sm font-semibold flex-shrink-0">
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
								{#if member.source === 'manual' && !team.created_via_sso && currentUserId !== member.user_id}
									<select
										value={member.role}
										onchange={(e) => updateMemberRole(member.user_id, e.currentTarget.value)}
										class="text-xs border border-gray-300 dark:border-gray-600 rounded-md px-2 py-1 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-1 focus:ring-earthy-terracotta-500"
									>
										<option value="member">Member</option>
										<option value="owner">Owner</option>
									</select>
								{:else}
									<span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium {member.role === 'owner' ? 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200' : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'}">
										{#if member.role === 'owner'}
											<Shield class="h-3 w-3 mr-1" />
										{/if}
										{member.role}
									</span>
								{/if}

								<!-- Source Badge -->
								{#if member.source === 'sso'}
									<span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
										<Lock class="h-3 w-3 mr-1" />
										SSO
									</span>
								{/if}

								<!-- Actions -->
								<div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
									{#if member.source === 'sso'}
										<button
											onclick={() => convertToManual(member.user_id)}
											class="text-xs text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300 px-2 py-1.5 rounded hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
											title="Convert to manual"
										>
											Make Permanent
										</button>
									{/if}
									{#if (member.source === 'manual' || !team.created_via_sso) && currentUserId !== member.user_id}
										<button
											onclick={() => removeMemberDirect(member.user_id, member.source)}
											disabled={removingMemberId === member.user_id}
											class="p-1.5 text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded disabled:opacity-50 transition-colors"
											title="Remove member"
										>
											<IconifyIcon icon="material-symbols:close" class="w-4 h-4" />
										</button>
									{/if}
								</div>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Assets Section -->
		<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
			<div class="flex items-center justify-between mb-4">
				<div class="flex items-center gap-2">
					<Database class="w-5 h-5 text-gray-500 dark:text-gray-400" />
					<h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">
						Assets Owned
					</h2>
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
						<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700" />
					</div>
				{:else if assets.length === 0}
					<div class="text-center py-8 text-gray-500 dark:text-gray-400">
						No assets owned by this team yet
					</div>
				{:else}
					<div class="space-y-1">
						{#each assets as asset}
							<a
								href="/discover/{asset.type}/{encodeURIComponent(asset.name)}"
								onclick={(e) => handleAssetClick(e, asset)}
								class="flex items-center justify-between p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors group"
							>
								<div class="flex items-center gap-3 flex-1 min-w-0">
									<div class="flex-shrink-0">
										<Icon name={getIconType(asset)} showLabel={false} iconSize="sm" />
									</div>
									<div class="flex-1 min-w-0">
										<div class="text-sm font-medium text-gray-900 dark:text-gray-100 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-500 truncate">
											{asset.name}
										</div>
										<div class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono">
											{asset.mrn}
										</div>
									</div>
								</div>
								<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200">
									{asset.type}
								</span>
							</a>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>

{#if selectedAsset}
	<AssetBlade asset={selectedAsset} onClose={() => (selectedAsset = null)} />
{/if}
