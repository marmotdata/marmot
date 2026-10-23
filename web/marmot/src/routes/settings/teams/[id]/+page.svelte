<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import OwnerSelector from '$components/shared/OwnerSelector.svelte';
	import Icon from '$components/ui/Icon.svelte';
	import IconifyIcon from '@iconify/svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import AssetBlade from '$components/asset/AssetBlade.svelte';
	import Tags from '$components/shared/Tags.svelte';
	import MetadataView from '$components/shared/MetadataView.svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import Panel from '$components/ui/Panel.svelte';
	import Pill from '$components/ui/Pill.svelte';
	import Alert from '$components/ui/Alert.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import {
		BTN_GHOST,
		BTN_PRIMARY,
		BTN_SECONDARY,
		INPUT,
		LABEL,
		TABLE,
		TBODY,
		TD,
		TH,
		TR
	} from '$lib/ui/classes';
	import type { Team, TeamMember } from '$lib/teams/types';
	import type { Asset } from '$lib/assets/types';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/utils';
	import {
		notificationTypeLabels,
		notificationTypeOptions,
		providerOptions,
		providerLabels,
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

	// The team and member payloads carry SSO provenance the shared types leave out
	type TeamDetail = Team & { created_via_sso?: boolean; sso_provider?: string };
	type Member = TeamMember & {
		source?: 'manual' | 'sso';
		provider_username?: string;
		provider_email?: string;
	};

	type Tab = 'overview' | 'integrations';

	let team = $state<TeamDetail | null>(null);
	let members = $state<Member[]>([]);
	let assets = $state<Asset[]>([]);
	let assetsTotal = $state(0);
	const assetsLimit = 20;
	let assetsOffset = $state(0);
	let loading = $state(true);
	let loadingAssets = $state(false);
	let error = $state<string | null>(null);
	let removingMemberId = $state<string | null>(null);
	let ownerSelectorRef = $state<OwnerSelector | null>(null);
	let selectedAsset = $state<Asset | null>(null);
	let showDeleteTeam = $state(false);

	// The tab lives in the query string so a link can open a team on its integrations
	let activeTab = $derived.by((): Tab => {
		const tab = page.url.searchParams.get('tab');
		return tab === 'integrations' ? tab : 'overview';
	});

	function setTab(tab: Tab) {
		return goto(resolve(`/settings/teams/${teamId}?tab=${tab}`), {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	}

	let canManageTeams = $derived(holdsAtRoot($myAccess, 'teams:manage'));

	// Webhook state
	let webhooks = $state<TeamWebhook[]>([]);
	let loadingWebhooks = $state(false);
	let showWebhookModal = $state(false);
	let editingWebhook = $state<TeamWebhook | null>(null);
	let webhookForm = $state<CreateWebhookInput>({
		name: '',
		provider: 'slack',
		webhook_url: '',
		notification_types: [],
		enabled: true
	});
	let savingWebhook = $state(false);
	let testingWebhookId = $state<string | null>(null);
	let webhookError = $state<string | null>(null);
	let webhookSuccess = $state<string | null>(null);
	let showDeleteConfirm = $state(false);
	let deletingWebhook = $state<TeamWebhook | null>(null);
	let showConvertConfirm = $state(false);
	let convertingUserId = $state<string | null>(null);

	let teamId = $derived(page.params.id ?? '');
	const currentUserId = auth.getCurrentUserId();
	let hasMoreAssets = $derived(assetsTotal > assetsOffset + assets.length);
	let currentPage = $derived(Math.floor(assetsOffset / assetsLimit) + 1);
	let totalPages = $derived(Math.ceil(assetsTotal / assetsLimit));
	let isTeamOwner = $derived(
		members.some((member) => member.user_id === currentUserId && member.role === 'owner')
	);
	let canEditTeam = $derived(canManageTeams || isTeamOwner);

	async function deleteTeam() {
		if (!team) return;
		try {
			const response = await fetchApi(`/teams/${team.id}`, { method: 'DELETE' });
			if (!response.ok) {
				toasts.error(await handleApiError(response));
				return;
			}
			toasts.success(m.teams_delete_success({ name: team.name }));
			goto(resolve('/settings/teams'));
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.teams_error_generic());
		} finally {
			showDeleteTeam = false;
		}
	}

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
			const loaded: TeamDetail = await response.json();
			if (!loaded.tags) loaded.tags = [];
			if (!loaded.metadata) loaded.metadata = {};
			team = loaded;
		} catch (err) {
			error = err instanceof Error ? err.message : m.teams_error_generic();
		} finally {
			loading = false;
		}
	}

	async function fetchMembers() {
		try {
			const response = await fetchApi(`/teams/${teamId}/members`);
			const data = await response.json();
			members = data.members ?? [];
		} catch (err) {
			error = err instanceof Error ? err.message : m.teams_error_generic();
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
		if (activeTab !== 'overview') {
			await setTab('overview');
			await tick();
		}
		ownerSelectorRef?.open();
	}

	async function handleMembersChange(newOwners: Owner[]) {
		if (!team || team.created_via_sso || newOwners.length === 0) return;

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

	async function removeMemberDirect(userId: string, _source: string) {
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
				// The API only returns a masked URL, so it is shown as a placeholder and only sent if replaced
				webhook_url: '',
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
		if (
			!webhookForm.name ||
			(!editingWebhook && !webhookForm.webhook_url) ||
			!webhookForm.notification_types.length
		)
			return;

		webhookError = null;

		try {
			savingWebhook = true;

			if (editingWebhook) {
				const response = await fetchApi(`/teams/${teamId}/webhooks/${editingWebhook.id}`, {
					method: 'PUT',
					body: JSON.stringify({
						name: webhookForm.name,
						webhook_url: webhookForm.webhook_url || undefined,
						notification_types: webhookForm.notification_types,
						enabled: webhookForm.enabled
					})
				});
				if (!response.ok) {
					const errData = await response.json();
					webhookError = errData.error || m.teams_webhook_error_update();
					return;
				}
			} else {
				const response = await fetchApi(`/teams/${teamId}/webhooks`, {
					method: 'POST',
					body: JSON.stringify(webhookForm)
				});
				if (!response.ok) {
					const errData = await response.json();
					webhookError = errData.error || m.teams_webhook_error_create();
					return;
				}
			}

			closeWebhookModal();
			await fetchWebhooks();
		} catch (err) {
			console.error('Failed to save webhook:', err);
			webhookError = m.teams_webhook_error_save();
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
				webhookError = errData.error || m.teams_webhook_error_delete();
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
				webhookSuccess = m.teams_webhook_test_success();
				setTimeout(() => {
					webhookSuccess = null;
				}, 5000);
			} else {
				const errData = await response.json();
				webhookError = errData.error || m.teams_webhook_test_error();
			}
		} catch (err) {
			console.error('Failed to test webhook:', err);
			webhookError = m.teams_webhook_test_error();
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

	const TABS = $derived(
		[
			{
				id: 'overview' as Tab,
				label: m.common_overview(),
				count: null as number | null,
				show: true
			},
			{
				id: 'integrations' as Tab,
				label: m.teams_integrations_heading(),
				count: webhooks.length || null,
				show: canEditTeam
			}
		].filter((tab) => tab.show)
	);

	const TILE =
		'flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg border border-gray-100 bg-gray-50 dark:border-gray-700 dark:bg-gray-900';
	const CHOICE_ON =
		'border-gray-900 bg-gray-900 text-white dark:border-gray-100 dark:bg-gray-100 dark:text-gray-900';
	const CHOICE_OFF =
		'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300';
</script>

<svelte:head>
	<title
		>{team
			? m.teams_detail_page_title({ name: team.name })
			: m.teams_detail_page_title_fallback()}</title
	>
</svelte:head>

{#if loading}
	<Skeleton class="h-16" />
	<div class="mt-6"><Skeleton class="h-40" rows={2} /></div>
{:else if error}
	<PageHeader
		title={m.teams_detail_page_title_fallback()}
		backHref={resolve('/settings/teams')}
		backLabel={m.settings_teams()}
	/>
	<Alert tone="error">{error}</Alert>
{:else if team}
	<PageHeader
		title={team.name}
		description={team.description || undefined}
		backHref={resolve('/settings/teams')}
		backLabel={m.settings_teams()}
	>
		{#snippet meta()}
			{#if team?.created_via_sso}
				<Pill tone="info">
					<IconifyIcon icon="material-symbols:lock-outline" class="h-3 w-3" />
					{m.teams_sso_managed_badge()}
				</Pill>
			{/if}
			{#if team?.sso_provider}
				<span class="text-xs text-gray-500 dark:text-gray-400"
					>{m.teams_sso_provider({ provider: team.sso_provider })}</span
				>
			{/if}
			<span class="text-xs tabular-nums text-gray-500 dark:text-gray-400"
				>{m.teams_member_count({ count: members.length })}</span
			>
		{/snippet}
		{#snippet actions()}
			{#if canManageTeams && !team?.created_via_sso}
				<button type="button" class={BTN_SECONDARY} onclick={() => (showDeleteTeam = true)}>
					<IconifyIcon icon="material-symbols:delete-outline" class="h-4 w-4" />
					{m.teams_delete_team_button()}
				</button>
			{/if}
			{#if !team?.created_via_sso && canEditTeam}
				<button type="button" class={BTN_PRIMARY} onclick={handleAddMembersClick}>
					<IconifyIcon icon="material-symbols:person-add-outline" class="h-5 w-5" />
					{m.teams_add_member()}
				</button>
			{/if}
		{/snippet}
	</PageHeader>

	{#if TABS.length > 1}
		<div
			class="mb-6 flex items-center gap-1 border-b border-gray-200 dark:border-gray-700"
			role="tablist"
			aria-label={m.teams_tabs_aria()}
		>
			{#each TABS as tab (tab.id)}
				<button
					type="button"
					role="tab"
					aria-selected={activeTab === tab.id}
					onclick={() => setTab(tab.id)}
					class="relative inline-flex h-10 items-center gap-1.5 px-2.5 text-sm font-medium transition {activeTab ===
					tab.id
						? 'text-gray-900 dark:text-gray-100'
						: 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'}"
				>
					{tab.label}
					{#if tab.count !== null}
						<span
							class="rounded-full bg-gray-100 px-1.5 text-[11px] tabular-nums text-gray-600 dark:bg-gray-800 dark:text-gray-300"
							>{tab.count}</span
						>
					{/if}
					{#if activeTab === tab.id}
						<span class="absolute inset-x-2 bottom-0 h-0.5 rounded-full bg-earthy-terracotta-500"
						></span>
					{/if}
				</button>
			{/each}
		</div>
	{/if}

	{#if activeTab === 'overview'}
		<div class="grid gap-6 lg:grid-cols-3">
			<div class="space-y-6 lg:col-span-2">
				<!-- Members -->
				<Panel flush title={m.teams_members_heading()} icon="material-symbols:group-outline">
					{#snippet action()}
						{#if team?.created_via_sso}
							<span class="text-xs text-gray-500 dark:text-gray-400"
								>{m.teams_members_sso_managed()}</span
							>
						{:else}
							<span class="text-xs tabular-nums text-gray-500 dark:text-gray-400"
								>{members.length}</span
							>
						{/if}
					{/snippet}

					<!-- The selector opens from the header action; it always starts empty and sits here so its dropdown drops over the list -->
					<div class="relative w-full px-5" style="min-height: 1px;">
						<OwnerSelector
							bind:this={ownerSelectorRef}
							selectedOwners={members.map((member) => ({
								id: member.user_id,
								name: member.name,
								type: 'user' as const,
								username: member.provider_username,
								email: member.provider_email,
								profile_picture: member.profile_picture
							}))}
							onChange={handleMembersChange}
							userOnly={true}
							hideAddButton={true}
							hideSelectedOwners={true}
							placeholder={m.teams_member_selector_placeholder()}
						/>
					</div>

					{#if members.length === 0}
						<div class="p-5">
							<EmptyState
								icon="material-symbols:person-add-outline"
								title={m.teams_no_members()}
								dashed={false}
							>
								{#snippet action()}
									{#if !team?.created_via_sso && canEditTeam}
										<button type="button" class={BTN_SECONDARY} onclick={handleAddMembersClick}>
											<IconifyIcon icon="material-symbols:add" class="h-4 w-4" />
											{m.teams_add_member()}
										</button>
									{/if}
								{/snippet}
							</EmptyState>
						</div>
					{:else}
						<div class="overflow-x-auto">
							<table class={TABLE}>
								<thead class="border-b border-gray-100 dark:border-gray-700/60">
									<tr>
										<th class="{TH} pl-5">{m.members_col_member()}</th>
										<th class={TH}>{m.teams_col_team_role()}</th>
										<th class="{TH} pr-5 text-right"
											><span class="sr-only">{m.common_actions()}</span></th
										>
									</tr>
								</thead>
								<tbody class={TBODY}>
									{#each members as member (member.user_id)}
										<tr class={TR}>
											<td class="{TD} pl-5">
												<span class="flex min-w-0 items-center gap-3">
													<span
														class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full border border-gray-100 bg-gray-50 text-xs font-semibold text-gray-600 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300"
														aria-hidden="true">{member.name.charAt(0).toUpperCase()}</span
													>
													<span class="min-w-0">
														<span
															class="block truncate font-medium text-gray-900 dark:text-gray-100"
															>{member.name}</span
														>
														{#if member.email}
															<span class="block truncate text-xs text-gray-500 dark:text-gray-400"
																>{member.email}</span
															>
														{/if}
													</span>
													{#if member.source === 'sso'}
														<Pill tone="info">
															<IconifyIcon icon="material-symbols:lock-outline" class="h-3 w-3" />
															{m.teams_sso_badge()}
														</Pill>
													{/if}
												</span>
											</td>
											<td class={TD}>
												{#if member.source === 'manual' && !team?.created_via_sso && canEditTeam && currentUserId !== member.user_id}
													<select
														value={member.role}
														onchange={(e) =>
															updateMemberRole(member.user_id, e.currentTarget.value)}
														aria-label={m.teams_col_team_role()}
														class="{INPUT} w-auto py-1 text-xs"
													>
														<option value="member">{m.teams_role_member()}</option>
														<option value="owner">{m.common_owner()}</option>
													</select>
												{:else}
													<Pill tone={member.role === 'owner' ? 'brand' : 'neutral'}>
														{#if member.role === 'owner'}
															<IconifyIcon icon="material-symbols:shield-outline" class="h-3 w-3" />
														{/if}
														{member.role === 'owner' ? m.common_owner() : m.teams_role_member()}
													</Pill>
												{/if}
											</td>
											<td class="{TD} whitespace-nowrap pr-5 text-right">
												{#if canEditTeam && member.source === 'sso' && currentUserId !== member.user_id}
													<button
														type="button"
														onclick={() => convertToManual(member.user_id)}
														class="{BTN_GHOST} text-xs"
														title={m.teams_convert_to_manual_title()}
													>
														{m.teams_make_permanent()}
													</button>
												{/if}
												{#if (canEditTeam && (member.source === 'manual' || !team?.created_via_sso)) || (!canEditTeam && currentUserId === member.user_id)}
													<button
														type="button"
														onclick={() => removeMemberDirect(member.user_id, member.source ?? '')}
														disabled={removingMemberId === member.user_id}
														class="{BTN_GHOST} hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
														title={currentUserId === member.user_id
															? m.teams_leave_team_title()
															: m.teams_remove_member_title()}
														aria-label={currentUserId === member.user_id
															? m.teams_leave_team_title()
															: m.teams_remove_member_title()}
													>
														<IconifyIcon
															icon="material-symbols:person-remove-outline"
															class="h-4 w-4"
														/>
													</button>
												{/if}
											</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{/if}
				</Panel>

				<!-- Assets -->
				<Panel
					flush
					title={m.teams_assets_owned_heading()}
					icon="material-symbols:database-outline"
				>
					{#snippet action()}
						{#if assetsTotal > assetsLimit}
							<button
								type="button"
								onclick={previousPage}
								disabled={assetsOffset === 0 || loadingAssets}
								class="{BTN_GHOST} px-1.5"
								title={m.teams_previous_page_title()}
								aria-label={m.teams_previous_page_title()}
							>
								<IconifyIcon icon="material-symbols:chevron-left" class="h-4 w-4" />
							</button>
							<span class="text-xs tabular-nums text-gray-500 dark:text-gray-400">
								{m.teams_page_of({ current: currentPage, total: totalPages })}
							</span>
							<button
								type="button"
								onclick={nextPage}
								disabled={!hasMoreAssets || loadingAssets}
								class="{BTN_GHOST} px-1.5"
								title={m.teams_next_page_title()}
								aria-label={m.teams_next_page_title()}
							>
								<IconifyIcon icon="material-symbols:chevron-right" class="h-4 w-4" />
							</button>
						{:else}
							<span class="text-xs tabular-nums text-gray-500 dark:text-gray-400"
								>{assetsTotal}</span
							>
						{/if}
					{/snippet}

					{#if loadingAssets}
						<div class="p-5"><Skeleton class="h-10" rows={3} /></div>
					{:else if assets.length === 0}
						<div class="p-5">
							<EmptyState
								icon="material-symbols:database-outline"
								title={m.teams_no_assets()}
								dashed={false}
							/>
						</div>
					{:else}
						<ul class={TBODY}>
							{#each assets as asset (asset.id)}
								<li>
									<a
										href={getAssetUrl(asset) === '#' ? '#' : resolve(getAssetUrl(asset) as '/')}
										onclick={(e) => handleAssetClick(e, asset)}
										class="group flex items-center gap-3 px-5 py-3 transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/40"
									>
										<span class={TILE}>
											<Icon name={getIconType(asset)} showLabel={false} size="sm" />
										</span>
										<span class="min-w-0 flex-1">
											<span
												class="block truncate text-sm font-medium text-gray-900 group-hover:text-earthy-terracotta-700 dark:text-gray-100 dark:group-hover:text-earthy-terracotta-400"
												>{asset.name}</span
											>
											<span
												class="block truncate font-mono text-xs text-gray-500 dark:text-gray-400"
												>{asset.mrn}</span
											>
										</span>
										<Pill>{asset.type}</Pill>
									</a>
								</li>
							{/each}
						</ul>
					{/if}
				</Panel>
			</div>

			<!-- About -->
			<div class="space-y-6">
				<Panel title={m.common_details()} icon="material-symbols:info-outline">
					<div class="space-y-5">
						<div>
							<h3 class="{LABEL} mb-2 flex items-center gap-1.5">
								<IconifyIcon icon="material-symbols:label-outline" class="h-4 w-4 text-gray-400" />
								{m.common_tags()}
							</h3>
							<Tags
								tags={team.tags ?? []}
								endpoint="/teams"
								id={team.id}
								canEdit={canEditTeam && !team.created_via_sso}
							/>
						</div>
						<div>
							<h3 class="{LABEL} mb-2 flex items-center gap-1.5">
								<IconifyIcon
									icon="material-symbols:database-outline"
									class="h-4 w-4 text-gray-400"
								/>
								{m.teams_metadata_heading()}
							</h3>
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
				</Panel>
			</div>
		</div>
	{/if}

	{#if activeTab === 'integrations' && canEditTeam}
		{#if showWebhookModal}
			<Panel
				title={editingWebhook ? m.teams_webhook_edit_heading() : m.teams_webhook_add_heading()}
				icon="material-symbols:webhook"
			>
				{#snippet action()}
					<button
						type="button"
						onclick={closeWebhookModal}
						class={BTN_GHOST}
						title={m.teams_webhook_back_to_list_title()}
						aria-label={m.teams_webhook_back_to_list_title()}
					>
						<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
					</button>
				{/snippet}

				<div class="grid grid-cols-1 gap-5 md:grid-cols-2">
					<div>
						<label for="webhook-name" class="{LABEL} mb-1.5 block">{m.common_name()}</label>
						<input
							id="webhook-name"
							type="text"
							bind:value={webhookForm.name}
							placeholder={m.teams_webhook_name_example_placeholder()}
							class={INPUT}
						/>
					</div>

					<div>
						<p class="{LABEL} mb-1.5">{m.teams_webhook_provider_label()}</p>
						<div
							class="grid grid-cols-3 gap-2"
							role="group"
							aria-label={m.teams_webhook_provider_label()}
						>
							{#each providerOptions() as provider (provider.value)}
								<button
									type="button"
									aria-pressed={webhookForm.provider === provider.value}
									onclick={() =>
										(webhookForm.provider = provider.value as CreateWebhookInput['provider'])}
									class="flex items-center justify-center gap-2 rounded-lg border px-3 py-2 text-sm font-medium transition {webhookForm.provider ===
									provider.value
										? CHOICE_ON
										: CHOICE_OFF}"
								>
									<IconifyIcon icon={provider.icon} class="h-4 w-4" />
									{provider.label}
								</button>
							{/each}
						</div>
					</div>

					<div class="md:col-span-2">
						<label for="webhook-url" class="{LABEL} mb-1.5 block"
							>{m.teams_webhook_url_label()}</label
						>
						<input
							id="webhook-url"
							type="url"
							bind:value={webhookForm.webhook_url}
							placeholder={editingWebhook?.webhook_url ?? 'https://hooks.slack.com/services/...'}
							class="{INPUT} font-mono"
						/>
					</div>

					<fieldset class="md:col-span-2">
						<legend class="{LABEL} mb-1.5">{m.teams_webhook_notification_types_label()}</legend>
						<div class="grid grid-cols-2 gap-2 md:grid-cols-3">
							{#each notificationTypeOptions() as { type, label, icon } (type)}
								{@const on = webhookForm.notification_types.includes(type)}
								<label
									class="flex cursor-pointer items-center gap-2.5 rounded-lg border px-3 py-2 transition {on
										? 'border-gray-400 bg-gray-50 dark:border-gray-500 dark:bg-gray-900'
										: 'border-gray-200 hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-700/40'}"
								>
									<input
										type="checkbox"
										checked={on}
										onchange={() => toggleNotificationType(type)}
										class="h-4 w-4 rounded border-gray-300 text-earthy-terracotta-600 focus:ring-earthy-terracotta-500 dark:border-gray-600"
									/>
									<IconifyIcon {icon} class="h-4 w-4 flex-shrink-0 text-gray-400" />
									<span class="text-sm text-gray-700 dark:text-gray-300">{label}</span>
								</label>
							{/each}
						</div>
					</fieldset>

					<div class="flex items-center justify-between py-1 md:col-span-2">
						<div>
							<span class="text-sm font-medium text-gray-900 dark:text-gray-100"
								>{m.common_enabled()}</span
							>
							<p class="text-xs text-gray-500 dark:text-gray-400">
								{m.teams_webhook_enabled_hint()}
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
							aria-label={m.common_enabled()}
						>
							<span
								class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {webhookForm.enabled
									? 'translate-x-6'
									: 'translate-x-1'}"
							></span>
						</button>
					</div>

					{#if webhookError}
						<div class="md:col-span-2"><Alert tone="error">{webhookError}</Alert></div>
					{/if}

					<div
						class="flex items-center justify-end gap-2 border-t border-gray-100 pt-4 md:col-span-2 dark:border-gray-700/60"
					>
						<button type="button" onclick={closeWebhookModal} class={BTN_SECONDARY}>
							{m.common_cancel()}
						</button>
						<button
							type="button"
							onclick={saveWebhook}
							disabled={savingWebhook ||
								!webhookForm.name ||
								(!editingWebhook && !webhookForm.webhook_url) ||
								!webhookForm.notification_types.length}
							class={BTN_PRIMARY}
						>
							{#if savingWebhook}
								{m.teams_webhook_saving()}
							{:else}
								{editingWebhook ? m.teams_webhook_update_button() : m.teams_webhook_create_button()}
							{/if}
						</button>
					</div>
				</div>
			</Panel>
		{:else}
			<Panel
				flush
				title={m.teams_webhooks_heading()}
				description={m.teams_webhooks_description()}
				icon="material-symbols:webhook"
			>
				{#snippet action()}
					<button type="button" onclick={openAddWebhook} class={BTN_SECONDARY}>
						<IconifyIcon icon="material-symbols:add" class="h-4 w-4" />
						{m.teams_webhook_add_button()}
					</button>
				{/snippet}

				{#if webhookSuccess || webhookError}
					<div class="border-b border-gray-100 px-5 py-3 dark:border-gray-700/60">
						{#if webhookSuccess}<Alert tone="success">{webhookSuccess}</Alert>{/if}
						{#if webhookError}<Alert tone="error">{webhookError}</Alert>{/if}
					</div>
				{/if}

				{#if loadingWebhooks}
					<div class="p-5"><Skeleton class="h-14" rows={2} /></div>
				{:else if webhooks.length === 0}
					<div class="p-5">
						<EmptyState
							icon="material-symbols:webhook"
							title={m.teams_no_webhooks_heading()}
							description={m.teams_no_webhooks_hint()}
							dashed={false}
						>
							{#snippet action()}
								<button type="button" onclick={openAddWebhook} class={BTN_PRIMARY}>
									<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
									{m.teams_webhook_add_button()}
								</button>
							{/snippet}
						</EmptyState>
					</div>
				{:else}
					<ul class={TBODY}>
						{#each webhooks as webhook (webhook.id)}
							<li class="flex flex-wrap items-center gap-3 px-5 py-4">
								<span class={TILE}>
									<IconifyIcon
										icon={providerOptions().find((p) => p.value === webhook.provider)?.icon ||
											'mdi:webhook'}
										class="h-5 w-5 text-gray-600 dark:text-gray-300"
									/>
								</span>
								<div class="min-w-0 flex-1">
									<div class="flex flex-wrap items-center gap-2">
										<span class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100"
											>{webhook.name}</span
										>
										<Pill tone={webhook.enabled ? 'success' : 'neutral'}>
											{webhook.enabled ? m.common_active() : m.common_disabled()}
										</Pill>
										<span class="text-xs text-gray-500 dark:text-gray-400"
											>{providerLabels()[webhook.provider] || webhook.provider}</span
										>
										<span class="text-xs text-gray-400 dark:text-gray-500" aria-hidden="true"
											>·</span
										>
										{#if webhook.last_error}
											<span
												class="text-xs text-red-600 dark:text-red-400"
												title={webhook.last_error}>{m.common_error()}</span
											>
										{:else if webhook.last_triggered_at}
											<span class="text-xs text-gray-500 dark:text-gray-400">
												{m.teams_webhook_last_triggered({
													date: formatDate(webhook.last_triggered_at)
												})}
											</span>
										{:else}
											<span class="text-xs text-gray-500 dark:text-gray-400"
												>{m.teams_webhook_never_triggered()}</span
											>
										{/if}
									</div>
									<div class="mt-1.5 flex flex-wrap items-center gap-1">
										{#each webhook.notification_types as type (type)}
											<Pill>{notificationTypeLabels()[type] || type}</Pill>
										{/each}
									</div>
								</div>
								<div class="flex flex-shrink-0 items-center gap-1">
									<button
										type="button"
										onclick={() => testWebhook(webhook)}
										disabled={testingWebhookId === webhook.id || !webhook.enabled}
										class={BTN_GHOST}
										title={m.teams_webhook_test_title()}
										aria-label={m.teams_webhook_test_title()}
									>
										<IconifyIcon
											icon={testingWebhookId === webhook.id
												? 'material-symbols:hourglass-empty'
												: 'material-symbols:send-outline'}
											class="h-4 w-4"
										/>
									</button>
									<button
										type="button"
										onclick={() => toggleWebhookEnabled(webhook)}
										class={BTN_GHOST}
										title={webhook.enabled
											? m.teams_webhook_disable_title()
											: m.teams_webhook_enable_title()}
										aria-label={webhook.enabled
											? m.teams_webhook_disable_title()
											: m.teams_webhook_enable_title()}
									>
										<IconifyIcon
											icon={webhook.enabled
												? 'material-symbols:toggle-on'
												: 'material-symbols:toggle-off'}
											class="h-5 w-5"
										/>
									</button>
									<button
										type="button"
										onclick={() => openEditWebhook(webhook)}
										class={BTN_GHOST}
										title={m.teams_webhook_edit_title()}
										aria-label={m.teams_webhook_edit_title()}
									>
										<IconifyIcon icon="material-symbols:edit-outline" class="h-4 w-4" />
									</button>
									<button
										type="button"
										onclick={() => deleteWebhook(webhook)}
										class="{BTN_GHOST} hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
										title={m.teams_webhook_delete_title()}
										aria-label={m.teams_webhook_delete_title()}
									>
										<IconifyIcon icon="material-symbols:delete-outline" class="h-4 w-4" />
									</button>
								</div>
							</li>
						{/each}
					</ul>
				{/if}
			</Panel>
		{/if}
	{/if}
{/if}

{#if selectedAsset}
	<AssetBlade asset={selectedAsset} onClose={() => (selectedAsset = null)} />
{/if}

<ConfirmModal
	bind:show={showDeleteConfirm}
	title={m.teams_webhook_delete_modal_title()}
	message={m.teams_webhook_delete_confirm_message({ name: deletingWebhook?.name ?? '' })}
	confirmText={m.common_delete()}
	variant="danger"
	onConfirm={confirmDeleteWebhook}
	onCancel={() => {
		showDeleteConfirm = false;
		deletingWebhook = null;
	}}
/>

<ConfirmModal
	bind:show={showConvertConfirm}
	title={m.teams_convert_modal_title()}
	message={m.teams_convert_confirm_message()}
	confirmText={m.teams_convert_confirm_button()}
	variant="warning"
	onConfirm={confirmConvertToManual}
	onCancel={() => {
		showConvertConfirm = false;
		convertingUserId = null;
	}}
/>

{#if team}
	<DeleteModal
		show={showDeleteTeam}
		title={m.teams_delete_title()}
		message={m.teams_delete_detail_message({ count: members.length })}
		confirmText={m.common_delete()}
		resourceName={team.name}
		requireConfirmation={true}
		onConfirm={deleteTeam}
		onCancel={() => (showDeleteTeam = false)}
	/>
{/if}
