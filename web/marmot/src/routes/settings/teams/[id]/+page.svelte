<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import '@marmotdata/elements/drawer.js';
	import type { MenuSelectDetail } from '@marmotdata/elements/classes/menu.js';
	import type { MmPageChangeDetail } from '@marmotdata/elements/classes/pagination.js';
	import type { MmTabChangeDetail } from '@marmotdata/elements/classes/tabs.js';
	import { MmConfirmDialog } from '@marmotdata/elements/classes/confirm-dialog.js';
	import { setCrumbs } from '$lib/mm';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/utils';
	import { assetHref } from '$lib/assetrules/assets';
	import { providerLabel } from '$lib/assetrules/plain';
	import { providerName, roleLabel } from '$lib/users/format';
	import { notificationTypeLabels, providerLabels, type TeamWebhook } from '$lib/teams/webhooks';
	import type { OwnerResult, TeamMemberRow, TeamRow } from '$lib/teams/types';
	import type { Asset } from '$lib/assets/types';
	import Tags from '$components/shared/Tags.svelte';
	import MetadataView from '$components/shared/MetadataView.svelte';
	import TeamMemberPicker from '$components/team/TeamMemberPicker.svelte';
	import WebhookDrawer from '$components/team/WebhookDrawer.svelte';
	import EditTeamDrawer from '$components/team/EditTeamDrawer.svelte';

	type Tab = 'overview' | 'integrations';

	const ASSETS_PAGE = 20;
	const MEMBERS_PAGE = 25;

	let teamId = $derived(page.params.id ?? '');
	const currentUserId = auth.getCurrentUserId();

	let team = $state<TeamRow | null>(null);
	let members = $state<TeamMemberRow[]>([]);
	let assets = $state<Asset[]>([]);
	let assetsTotal = $state(0);
	let assetsPage = $state(1);
	let membersPage = $state(1);
	let loading = $state(true);
	let loadingAssets = $state(false);
	let error = $state('');
	let webhooks = $state<TeamWebhook[]>([]);
	let loadingWebhooks = $state(false);
	let testingId = $state<string | null>(null);
	let addingPersonId = $state<string | null>(null);

	let editingTeam = $state(false);
	let addingMembers = $state(false);
	let webhookDrawer = $state<{ webhook: TeamWebhook | null } | null>(null);

	let canManageTeams = $derived(holdsAtRoot($myAccess, 'teams:manage'));
	let isTeamOwner = $derived(
		members.some((member) => member.user_id === currentUserId && member.role === 'owner')
	);
	let canEditTeam = $derived(canManageTeams || isTeamOwner);
	let sso = $derived(!!team?.created_via_sso);

	// The tab lives in the query string so a link can open a team on its integrations
	let activeTab = $derived<Tab>(
		page.url.searchParams.get('tab') === 'integrations' && canEditTeam ? 'integrations' : 'overview'
	);

	let pagedMembers = $derived(
		members.slice((membersPage - 1) * MEMBERS_PAGE, membersPage * MEMBERS_PAGE)
	);
	let failing = $derived(webhooks.filter((w) => w.last_error));

	$effect(() => {
		setCrumbs([
			{ label: m.settings_title(), href: resolve('/settings') },
			{ label: m.settings_teams(), href: resolve('/settings/teams') },
			{ label: team?.name ?? m.teams_detail_crumb() }
		]);
	});

	$effect(() => {
		if (!teamId) return;
		loadTeam();
		fetchMembers();
		fetchAssets(1);
		fetchWebhooks();
	});

	function setTab(tab: string) {
		goto(resolve(`/settings/teams/${teamId}?tab=${tab === 'integrations' ? tab : 'overview'}`), {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	}

	async function loadTeam() {
		loading = true;
		error = '';
		try {
			const response = await fetchApi(`/teams/${teamId}`);
			if (!response.ok) throw new Error(await handleApiError(response));
			const loaded: TeamRow = await response.json();
			loaded.tags ??= [];
			loaded.metadata ??= {};
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
			if (!response.ok) return;
			const data = await response.json();
			members = data.members ?? [];
		} catch {
			members = [];
		}
	}

	async function fetchAssets(pageNumber: number) {
		loadingAssets = true;
		assetsPage = pageNumber;
		try {
			const offset = (pageNumber - 1) * ASSETS_PAGE;
			const response = await fetchApi(
				`/assets/search?owner_type=team&owner_id=${teamId}&limit=${ASSETS_PAGE}&offset=${offset}`
			);
			const data = response.ok ? await response.json() : {};
			assets = data.assets ?? [];
			assetsTotal = data.total ?? 0;
		} catch {
			assets = [];
			assetsTotal = 0;
		} finally {
			loadingAssets = false;
		}
	}

	async function fetchWebhooks() {
		loadingWebhooks = true;
		try {
			const response = await fetchApi(`/teams/${teamId}/webhooks`);
			// Members who cannot manage the team get a 403 here, and simply see no integrations
			webhooks = response.ok ? ((await response.json()).webhooks ?? []) : [];
		} catch {
			webhooks = [];
		} finally {
			loadingWebhooks = false;
		}
	}

	async function deleteTeam() {
		if (!team) return;
		const ok = await MmConfirmDialog.confirm({
			heading: m.teams_delete_heading({ name: team.name }),
			body: m.teams_delete_detail_message({ count: members.length }),
			confirmLabel: m.teams_delete_team_button(),
			confirmText: team.name
		});
		if (!ok) return;
		const response = await fetchApi(`/teams/${team.id}`, { method: 'DELETE' });
		if (!response.ok) {
			toasts.error(await handleApiError(response));
			return;
		}
		toasts.success(m.teams_delete_success({ name: team.name }));
		goto(resolve('/settings/teams'));
	}

	async function addMember(person: OwnerResult) {
		addingPersonId = person.id;
		try {
			const response = await fetchApi(`/teams/${teamId}/members`, {
				method: 'POST',
				body: JSON.stringify({ user_id: person.id, role: 'member' })
			});
			if (!response.ok) {
				toasts.error(await handleApiError(response));
				return;
			}
			toasts.success(m.teams_member_added_toast({ name: person.name }));
			await fetchMembers();
		} finally {
			addingPersonId = null;
		}
	}

	async function setRole(member: TeamMemberRow, role: 'owner' | 'member') {
		const response = await fetchApi(`/teams/${teamId}/members/${member.user_id}/role`, {
			method: 'PUT',
			body: JSON.stringify({ role })
		});
		if (!response.ok) {
			toasts.error(await handleApiError(response));
			return;
		}
		await fetchMembers();
	}

	async function keepMember(member: TeamMemberRow) {
		const ok = await MmConfirmDialog.confirm({
			heading: m.teams_keep_member_heading({ name: member.name }),
			body: m.teams_keep_member_body(),
			confirmLabel: m.teams_keep_member(),
			confirmIcon: 'lock',
			tone: 'default'
		});
		if (!ok) return;
		const response = await fetchApi(
			`/teams/${teamId}/members/${member.user_id}/convert-to-manual`,
			{
				method: 'POST'
			}
		);
		if (!response.ok) {
			toasts.error(await handleApiError(response));
			return;
		}
		await fetchMembers();
	}

	async function removeMember(member: TeamMemberRow) {
		const self = member.user_id === currentUserId;
		const ok = await MmConfirmDialog.confirm({
			heading: self
				? m.teams_leave_heading({ team: team?.name ?? '' })
				: m.teams_remove_member_heading({ name: member.name }),
			body: self ? m.teams_leave_body() : m.teams_remove_member_body(),
			confirmLabel: self ? m.teams_leave_team_title() : m.teams_remove_from_team(),
			confirmIcon: 'person-remove'
		});
		if (!ok) return;
		const response = await fetchApi(`/teams/${teamId}/members/${member.user_id}`, {
			method: 'DELETE'
		});
		if (!response.ok) {
			toasts.error(await handleApiError(response));
			return;
		}
		toasts.success(m.teams_member_removed_toast({ name: member.name }));
		await fetchMembers();
	}

	function memberActions(member: TeamMemberRow) {
		const self = member.user_id === currentUserId;
		const items: { value: string; label: string; icon: string; danger?: boolean }[] = [];
		if (canEditTeam && !self && member.source !== 'sso' && !sso) {
			items.push(
				member.role === 'owner'
					? { value: 'member', label: m.teams_make_member(), icon: 'person' }
					: { value: 'owner', label: m.teams_make_owner(), icon: 'shield-person' }
			);
		}
		if (canEditTeam && !self && member.source === 'sso') {
			items.push({ value: 'keep', label: m.teams_keep_member(), icon: 'lock' });
		}
		if ((canEditTeam && (member.source === 'manual' || !sso)) || (!canEditTeam && self)) {
			items.push({
				value: 'remove',
				label: self ? m.teams_leave_team_title() : m.teams_remove_from_team(),
				icon: 'person-remove',
				danger: true
			});
		}
		return items;
	}

	function onMemberAction(member: TeamMemberRow, value: string) {
		if (value === 'owner' || value === 'member') setRole(member, value);
		else if (value === 'keep') keepMember(member);
		else if (value === 'remove') removeMember(member);
	}

	async function testWebhook(webhook: TeamWebhook) {
		testingId = webhook.id;
		try {
			const response = await fetchApi(`/teams/${teamId}/webhooks/${webhook.id}/test`, {
				method: 'POST'
			});
			if (response.ok) toasts.success(m.teams_webhook_test_success());
			else toasts.error(await handleApiError(response));
		} catch {
			toasts.error(m.teams_webhook_test_error());
		} finally {
			testingId = null;
		}
	}

	async function setWebhookEnabled(webhook: TeamWebhook, enabled: boolean) {
		const response = await fetchApi(`/teams/${teamId}/webhooks/${webhook.id}`, {
			method: 'PUT',
			body: JSON.stringify({ enabled })
		});
		if (!response.ok) {
			toasts.error(await handleApiError(response));
			return;
		}
		await fetchWebhooks();
	}

	async function editWebhook(webhook: TeamWebhook) {
		// The list carries a summary, so the full record is fetched for the form
		const response = await fetchApi(`/teams/${teamId}/webhooks/${webhook.id}`);
		webhookDrawer = { webhook: response.ok ? await response.json() : webhook };
	}

	async function deleteWebhook(webhook: TeamWebhook) {
		const ok = await MmConfirmDialog.confirm({
			heading: m.teams_webhook_delete_heading({ name: webhook.name }),
			body: m.teams_webhook_delete_body(),
			confirmLabel: m.teams_webhook_delete_title()
		});
		if (!ok) return;
		const response = await fetchApi(`/teams/${teamId}/webhooks/${webhook.id}`, {
			method: 'DELETE'
		});
		if (!response.ok && response.status !== 204) {
			toasts.error(await handleApiError(response));
			return;
		}
		await fetchWebhooks();
	}

	function onWebhookAction(webhook: TeamWebhook, value: string) {
		if (value === 'edit') editWebhook(webhook);
		else if (value === 'off') setWebhookEnabled(webhook, false);
		else if (value === 'on') setWebhookEnabled(webhook, true);
		else if (value === 'delete') deleteWebhook(webhook);
	}

	function assetCaption(asset: Asset): string {
		const provider = asset.providers?.length === 1 ? providerLabel(asset.providers[0]) : '';
		return [asset.type, provider].filter(Boolean).join(' · ');
	}
</script>

<svelte:head>
	<title>{team ? team.name : m.teams_detail_crumb()} · {m.settings_teams()}</title>
</svelte:head>

{#if loading && !team}
	<div class="mm-stack" style="--mm-gap: var(--space-5)" aria-busy="true">
		<mm-skeleton shape="text" count={1} style="width: 16rem"></mm-skeleton>
		<mm-skeleton shape="table-row" count={5}></mm-skeleton>
	</div>
{:else if error || !team}
	<div class="mm-stack" style="--mm-gap: var(--space-5)">
		<mm-page-header heading={m.teams_detail_crumb()}></mm-page-header>
		<mm-callout tone="danger" heading={m.teams_error_load_team()}>
			{error}
			<mm-button slot="actions" size="sm" icon="refresh" onclick={loadTeam}
				>{m.common_retry()}</mm-button
			>
		</mm-callout>
	</div>
{:else}
	<div class="mm-stack" style="--mm-gap: var(--space-5)">
		<mm-page-header
			heading={team.name}
			icon="groups"
			caption={[
				m.teams_member_count({ count: members.length }),
				m.teams_asset_count({ count: assetsTotal }),
				sso ? m.teams_source_sso({ provider: providerName(team.sso_provider ?? '') }) : ''
			]
				.filter(Boolean)
				.join(' · ')}
			description={team.description ?? ''}
		>
			{#if team.tags.length || (canEditTeam && !sso)}
				<div slot="tags">
					<Tags tags={team.tags} endpoint="/teams" id={team.id} canEdit={canEditTeam && !sso} />
				</div>
			{/if}
			{#if canManageTeams && !sso}
				<mm-menu-button
					slot="actions"
					variant="ghost"
					icon="more-horiz"
					icon-only
					label={m.teams_more_actions({ name: team.name })}
				>
					<mm-menu onmm-select={deleteTeam}>
						<mm-menu-item icon="delete" danger
							>{m.teams_delete_team_named({ name: team.name })}</mm-menu-item
						>
					</mm-menu>
				</mm-menu-button>
			{/if}
			{#if canEditTeam && !sso}
				<mm-button slot="actions" icon="edit" onclick={() => (editingTeam = true)}>
					{m.teams_edit_team()}
				</mm-button>
			{/if}
		</mm-page-header>

		<mm-tabs
			label={m.teams_tabs_aria()}
			value={activeTab}
			onmm-tab-change={(e: CustomEvent<MmTabChangeDetail>) => setTab(e.detail.value)}
		>
			<mm-tab value="overview" icon="groups">{m.common_overview()}</mm-tab>
			{#if canEditTeam}
				<mm-tab value="integrations" icon="webhook" count={webhooks.length || undefined}>
					{m.teams_integrations_heading()}
				</mm-tab>
			{/if}

			<mm-tab-panel value="overview">
				<div class="mm-split">
					<div class="mm-stack" style="--mm-gap: 0; min-width: 0">
						<mm-section
							heading={m.teams_members_heading()}
							description={sso ? m.teams_members_from_sso() : ''}
						>
							{#if canEditTeam && !sso}
								<mm-button
									slot="actions"
									size="sm"
									icon="person-add"
									onclick={() => (addingMembers = true)}
								>
									{m.teams_add_member_button()}
								</mm-button>
							{/if}
							{#if members.length === 0}
								<mm-empty-state
									compact
									heading={m.teams_no_members()}
									description={m.teams_no_members_body()}
								></mm-empty-state>
							{:else}
								<mm-list aria-label={m.teams_members_heading()}>
									{#each pagedMembers as member (member.user_id)}
										{@const actions = memberActions(member)}
										<mm-list-item>
											<mm-owner-chip
												slot="start"
												name={member.name}
												kind="person"
												src={member.profile_picture ?? ''}
											></mm-owner-chip>
											<span class="mm-muted">{member.email ?? member.username}</span>
											<span slot="end" class="mm-cluster" style="--mm-gap: var(--space-2)">
												<span class="mm-caption">
													{member.role === 'owner'
														? m.common_owner()
														: m.teams_role_member()}{member.source === 'sso'
														? ` · ${m.teams_from_sso()}`
														: ''}
												</span>
												{#if actions.length}
													<mm-menu-button
														variant="ghost"
														size="sm"
														icon="more-horiz"
														icon-only
														placement="bottom-end"
														label={m.teams_member_actions({ name: member.name })}
													>
														<mm-menu
															onmm-select={(e: CustomEvent<MenuSelectDetail>) =>
																onMemberAction(member, e.detail.value)}
														>
															{#each actions.filter((a) => !a.danger) as action (action.value)}
																<mm-menu-item value={action.value} icon={action.icon}
																	>{action.label}</mm-menu-item
																>
															{/each}
															{#if actions.some((a) => a.danger)}
																{#if actions.some((a) => !a.danger)}<mm-menu-separator
																	></mm-menu-separator>{/if}
																{#each actions.filter((a) => a.danger) as action (action.value)}
																	<mm-menu-item value={action.value} icon={action.icon} danger>
																		{action.label}
																	</mm-menu-item>
																{/each}
															{/if}
														</mm-menu>
													</mm-menu-button>
												{/if}
											</span>
										</mm-list-item>
									{/each}
								</mm-list>
								{#if members.length > MEMBERS_PAGE}
									<mm-pagination
										variant="compact"
										label={m.teams_members_pagination()}
										page={membersPage}
										page-size={MEMBERS_PAGE}
										total={members.length}
										onmm-page-change={(e: CustomEvent<MmPageChangeDetail>) =>
											(membersPage = e.detail.page)}
									></mm-pagination>
								{/if}
							{/if}
						</mm-section>

						<mm-section heading={m.teams_owned_assets()}>
							{#if assetsTotal === 0 && !loadingAssets}
								<mm-empty-state
									compact
									heading={m.teams_no_assets()}
									description={m.teams_no_assets_body()}
								></mm-empty-state>
							{:else}
								<mm-list aria-label={m.teams_owned_assets()} aria-busy={loadingAssets}>
									{#each assets as asset (asset.id)}
										<mm-list-item href={assetHref(asset)} secondary={assetCaption(asset)}>
											<mm-provider-logo
												slot="start"
												provider={asset.providers?.[0] ?? asset.type}
												size="16"
												decorative
											></mm-provider-logo>
											{asset.name}
										</mm-list-item>
									{/each}
								</mm-list>
								{#if assetsTotal > ASSETS_PAGE}
									<mm-pagination
										variant="compact"
										label={m.teams_assets_pagination()}
										page={assetsPage}
										page-size={ASSETS_PAGE}
										total={assetsTotal}
										disabled={loadingAssets}
										onmm-page-change={(e: CustomEvent<MmPageChangeDetail>) =>
											fetchAssets(e.detail.page)}
									></mm-pagination>
								{/if}
							{/if}
						</mm-section>
					</div>

					<div class="mm-stack" style="--mm-gap: 0; min-width: 0">
						<mm-section heading={m.common_details()}>
							<mm-description-list layout="stacked">
								{#if sso}
									<mm-description term={m.teams_single_sign_on()}>
										{m.teams_managed_by({ provider: providerName(team.sso_provider ?? '') })}
									</mm-description>
								{/if}
								{#each team.sso_groups ?? [] as g (g.provider + g.sso_group_name)}
									<mm-description term={m.teams_sso_mapping_term()}>
										{g.role_name
											? m.teams_sso_mapping_with_role({
													group: g.sso_group_name,
													provider: providerName(g.provider),
													role: roleLabel(g.role_name)
												})
											: m.teams_sso_mapping({
													group: g.sso_group_name,
													provider: providerName(g.provider)
												})}
									</mm-description>
								{/each}
								<mm-description term={m.teams_created()}
									>{formatDate(team.created_at)}</mm-description
								>
								<mm-description term={m.teams_updated()}
									>{formatDate(team.updated_at)}</mm-description
								>
							</mm-description-list>
						</mm-section>
						<mm-section heading={m.teams_metadata_heading()}>
							<MetadataView
								bind:metadata={team.metadata}
								endpoint="/teams"
								id={team.id}
								permissionResource="teams"
								permissionAction="manage"
								readOnly={sso}
								maxDepth={2}
							/>
						</mm-section>
					</div>
				</div>
			</mm-tab-panel>

			{#if canEditTeam}
				<mm-tab-panel value="integrations">
					<mm-section heading={m.teams_webhooks_heading()}>
						{#if webhooks.length}
							<mm-button
								slot="actions"
								icon="add"
								onclick={() => (webhookDrawer = { webhook: null })}
							>
								{m.teams_add_webhook()}
							</mm-button>
						{/if}
						{#if loadingWebhooks && webhooks.length === 0}
							<mm-skeleton shape="table-row" count={2}></mm-skeleton>
						{:else if webhooks.length === 0}
							<mm-empty-state
								heading={m.teams_no_webhooks_heading()}
								description={m.teams_no_webhooks_body()}
							>
								<mm-button
									slot="action"
									variant="primary"
									icon="add"
									onclick={() => (webhookDrawer = { webhook: null })}
								>
									{m.teams_add_webhook()}
								</mm-button>
							</mm-empty-state>
						{:else}
							<div class="mm-stack" style="--mm-gap: var(--space-4)">
								<mm-list aria-label={m.teams_webhooks_heading()}>
									{#each webhooks as webhook (webhook.id)}
										<mm-list-item
											secondary={[
												providerLabels()[webhook.provider] ?? webhook.provider,
												webhook.notification_types
													.map((t) => notificationTypeLabels()[t] ?? t)
													.join(', ')
											].join(' · ')}
										>
											<mm-icon
												slot="start"
												name={webhook.provider === 'generic' ? 'webhook' : 'chat'}
											></mm-icon>
											{webhook.name}
											<span slot="end" class="mm-cluster" style="--mm-gap: var(--space-1)">
												{#if webhook.last_error}
													<mm-status-pill status="danger" icon="error"
														>{m.teams_webhook_failing()}</mm-status-pill
													>
												{:else if !webhook.enabled}
													<span class="mm-muted">{m.teams_webhook_off()}</span>
												{/if}
												<mm-button
													size="sm"
													variant="ghost"
													icon="play-arrow"
													disabled={!webhook.enabled}
													loading={testingId === webhook.id}
													label={m.teams_webhook_send_test_aria({ name: webhook.name })}
													onclick={() => testWebhook(webhook)}
												>
													{m.teams_webhook_send_test()}
												</mm-button>
												<mm-menu-button
													variant="ghost"
													size="sm"
													icon="more-horiz"
													icon-only
													placement="bottom-end"
													label={m.teams_webhook_actions({ name: webhook.name })}
												>
													<mm-menu
														onmm-select={(e: CustomEvent<MenuSelectDetail>) =>
															onWebhookAction(webhook, e.detail.value)}
													>
														<mm-menu-item value="edit" icon="edit"
															>{m.teams_webhook_edit_title()}</mm-menu-item
														>
														{#if webhook.enabled}
															<mm-menu-item value="off" icon="notifications-off">
																{m.teams_webhook_turn_off()}
															</mm-menu-item>
														{:else}
															<mm-menu-item value="on" icon="notifications"
																>{m.teams_webhook_turn_on()}</mm-menu-item
															>
														{/if}
														<mm-menu-separator></mm-menu-separator>
														<mm-menu-item value="delete" icon="delete" danger>
															{m.teams_webhook_delete_title()}
														</mm-menu-item>
													</mm-menu>
												</mm-menu-button>
											</span>
										</mm-list-item>
									{/each}
								</mm-list>
								{#each failing as webhook (webhook.id)}
									<mm-callout
										tone="danger"
										heading={m.teams_webhook_failing_heading({ name: webhook.name })}
									>
										{webhook.last_error}
										{m.teams_webhook_failing_body()}
									</mm-callout>
								{/each}
							</div>
						{/if}
					</mm-section>
				</mm-tab-panel>
			{/if}
		</mm-tabs>
	</div>
{/if}

{#if team && editingTeam}
	<EditTeamDrawer
		{team}
		onClose={() => (editingTeam = false)}
		onSaved={(saved) => {
			team = saved;
			editingTeam = false;
		}}
	/>
{/if}

{#if addingMembers}
	<mm-drawer
		open
		dismissible
		heading={m.teams_add_members_heading()}
		onmm-close={() => (addingMembers = false)}
	>
		<TeamMemberPicker
			exclude={members.map((member) => member.user_id)}
			busyId={addingPersonId}
			onAdd={addMember}
		/>
		<mm-button slot="footer" variant="primary" data-close="done">{m.teams_done()}</mm-button>
	</mm-drawer>
{/if}

{#if webhookDrawer}
	<WebhookDrawer
		{teamId}
		webhook={webhookDrawer.webhook}
		onClose={() => (webhookDrawer = null)}
		onSaved={() => {
			webhookDrawer = null;
			fetchWebhooks();
		}}
	/>
{/if}
