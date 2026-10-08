<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import StepperPage from '$components/ui/StepperPage.svelte';
	import TagsInput from '$components/shared/TagsInput.svelte';
	import MetadataView from '$components/shared/MetadataView.svelte';
	import Avatar from '$components/user/Avatar.svelte';
	import { fetchApi } from '$lib/api';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { createKeyboardNavigationState } from '$lib/keyboard';
	import { m } from '$lib/paraglide/messages';
	import { formatList } from '$lib/utils';
	import {
		notificationTypeOptions,
		providerOptions,
		providerLabels,
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

	interface PendingWebhook {
		key: string;
		name: string;
		provider: 'slack' | 'discord' | 'generic';
		webhook_url: string;
		notification_types: string[];
		enabled: boolean;
	}

	let name = $state('');
	let description = $state('');
	let members = $state<Owner[]>([]);
	let memberRoles = $state<Record<string, 'owner' | 'member'>>({});
	let tags = $state<string[]>([]);
	let metadata = $state<Record<string, unknown>>({});
	let webhooks = $state<PendingWebhook[]>([]);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let currentStep = $state(1);

	// Member search state
	let memberQuery = $state('');
	let memberResults = $state<Owner[]>([]);
	let memberSearchLoading = $state(false);
	let memberSearchTimer: ReturnType<typeof setTimeout> | undefined;
	let focusedMemberIndex = $state(-1);
	let memberDropdownRef = $state<HTMLDivElement>();

	const stepperSteps = [
		{ title: m.teams_step_basic_info(), icon: 'material-symbols:info-outline' },
		{ title: m.teams_step_members(), icon: 'material-symbols:group-outline' },
		{ title: m.teams_step_integrations(), icon: 'material-symbols:webhook' },
		{ title: m.teams_step_review(), icon: 'material-symbols:summarize' }
	];

	let canProceedToStep2 = $derived(name.trim().length >= 2);

	// Filter search results to exclude already-added members
	let availableResults = $derived(memberResults.filter((r) => !members.some((m) => m.id === r.id)));

	// Reset keyboard focus when results change
	$effect(() => {
		const _r = availableResults;
		focusedMemberIndex = -1;
	});

	function scrollFocusedIntoView() {
		if (memberDropdownRef && focusedMemberIndex >= 0) {
			const buttons = memberDropdownRef.querySelectorAll<HTMLButtonElement>(
				'button[data-member-result]'
			);
			const el = buttons[focusedMemberIndex];
			if (el) el.scrollIntoView({ block: 'nearest' });
		}
	}

	const { handleKeydown: handleMemberKeydown } = createKeyboardNavigationState<Owner>(
		() => availableResults,
		() => focusedMemberIndex,
		(i) => {
			focusedMemberIndex = i;
			scrollFocusedIntoView();
		},
		{
			onSelect: (owner) => addMember(owner),
			onEscape: () => {
				memberQuery = '';
				memberResults = [];
				focusedMemberIndex = -1;
			}
		}
	);

	// Webhook validity check for step 4 → step 5
	let webhooksValid = $derived.by(() => {
		return webhooks.every(
			(w) =>
				w.name.trim().length > 0 &&
				w.webhook_url.trim().length > 0 &&
				w.notification_types.length > 0
		);
	});

	function canNavigateToStep(step: number): boolean {
		if (step >= 2 && !canProceedToStep2) return false;
		if (step >= 4 && !webhooksValid) return false;
		return true;
	}

	function scheduleMemberSearch() {
		if (memberSearchTimer) clearTimeout(memberSearchTimer);
		const q = memberQuery.trim();
		if (q.length < 2) {
			memberResults = [];
			memberSearchLoading = false;
			return;
		}
		memberSearchLoading = true;
		memberSearchTimer = setTimeout(async () => {
			try {
				const res = await fetchApi(`/owners/search?q=${encodeURIComponent(q)}&limit=50`);
				if (res.ok) {
					const data = await res.json();
					const all: Owner[] = data.owners ?? [];
					memberResults = all.filter((o) => o.type === 'user');
				} else {
					memberResults = [];
				}
			} catch {
				memberResults = [];
			} finally {
				memberSearchLoading = false;
			}
		}, 250);
	}

	function addMember(owner: Owner) {
		if (members.some((m) => m.id === owner.id)) return;
		members = [...members, owner];
		if (!memberRoles[owner.id]) {
			memberRoles = { ...memberRoles, [owner.id]: 'member' };
		}
		memberQuery = '';
		memberResults = [];
	}

	function toggleMemberRole(id: string) {
		memberRoles = {
			...memberRoles,
			[id]: memberRoles[id] === 'owner' ? 'member' : 'owner'
		};
	}

	function removeMember(id: string) {
		members = members.filter((m) => m.id !== id);
		const next = { ...memberRoles };
		delete next[id];
		memberRoles = next;
	}

	function addWebhook() {
		webhooks = [
			...webhooks,
			{
				key: crypto.randomUUID(),
				name: '',
				provider: 'slack',
				webhook_url: '',
				notification_types: [],
				enabled: true
			}
		];
	}

	function removeWebhook(key: string) {
		webhooks = webhooks.filter((w) => w.key !== key);
	}

	function toggleWebhookNotificationType(key: string, type: string) {
		webhooks = webhooks.map((w) => {
			if (w.key !== key) return w;
			const next = w.notification_types.includes(type)
				? w.notification_types.filter((t) => t !== type)
				: [...w.notification_types, type];
			return { ...w, notification_types: next };
		});
	}

	function handleNext() {
		error = null;
		if (currentStep === 1) {
			if (!canProceedToStep2) {
				error = m.teams_error_name_too_short();
				return;
			}
			currentStep = 2;
			return;
		}
		if (currentStep === 3) {
			if (!webhooksValid) {
				error = m.teams_error_integration_incomplete();
				return;
			}
		}
		currentStep = Math.min(currentStep + 1, stepperSteps.length);
	}

	async function handleSave() {
		if (!name.trim()) {
			error = m.teams_error_name_required();
			return;
		}

		try {
			saving = true;
			error = null;

			// 1) Create the team
			const createResp = await fetchApi('/teams', {
				method: 'POST',
				body: JSON.stringify({ name: name.trim(), description: description.trim() })
			});
			if (!createResp.ok) {
				error = await handleApiError(createResp);
				return;
			}
			const team = await createResp.json();

			const warnings: string[] = [];

			// 2) Update tags/metadata if either provided
			const hasMeta = Object.keys(metadata ?? {}).length > 0;
			if (tags.length > 0 || hasMeta) {
				const patch: Record<string, unknown> = {};
				if (tags.length > 0) patch.tags = tags;
				if (hasMeta) patch.metadata = metadata;
				const patchResp = await fetchApi(`/teams/${team.id}`, {
					method: 'PUT',
					body: JSON.stringify(patch)
				});
				if (!patchResp.ok) {
					warnings.push(m.teams_failed_item_tags_metadata());
				}
			}

			// 3) Add members
			for (const member of members) {
				const role = memberRoles[member.id] ?? 'member';
				const memResp = await fetchApi(`/teams/${team.id}/members`, {
					method: 'POST',
					body: JSON.stringify({ user_id: member.id, role })
				});
				if (!memResp.ok) {
					warnings.push(m.teams_failed_item_member({ name: member.name }));
				}
			}

			// 4) Add webhooks
			for (const w of webhooks) {
				const payload: CreateWebhookInput = {
					name: w.name.trim(),
					provider: w.provider,
					webhook_url: w.webhook_url.trim(),
					notification_types: w.notification_types,
					enabled: w.enabled
				};
				const hookResp = await fetchApi(`/teams/${team.id}/webhooks`, {
					method: 'POST',
					body: JSON.stringify(payload)
				});
				if (!hookResp.ok) {
					warnings.push(m.teams_failed_item_webhook({ name: w.name }));
				}
			}

			if (warnings.length > 0) {
				toasts.warning(m.teams_created_with_warnings({ items: formatList(warnings) }));
			} else {
				toasts.success(m.teams_created_toast({ name: team.name }));
			}

			goto(resolve(`/teams/${team.id}`));
		} catch (err) {
			error = err instanceof Error ? err.message : m.teams_error_create();
		} finally {
			saving = false;
		}
	}

	function goBack() {
		goto(resolve('/admin?tab=teams'));
	}
</script>

<StepperPage
	title={m.teams_create_team()}
	steps={stepperSteps}
	{currentStep}
	onBack={goBack}
	onCancel={goBack}
	onPrevious={() => currentStep--}
	onNext={currentStep < stepperSteps.length ? handleNext : undefined}
	onSave={currentStep === stepperSteps.length ? handleSave : undefined}
	canProceed={currentStep === 1 ? canProceedToStep2 : currentStep === 3 ? webhooksValid : true}
	{saving}
	saveLabel={m.teams_create_team()}
	savingLabel={m.teams_creating()}
	{error}
	{canNavigateToStep}
	onStepClick={(step) => (currentStep = step)}
>
	<!-- Step 1: Basic Info -->
	{#if currentStep === 1}
		<div class="space-y-6">
			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
					<IconifyIcon
						icon="material-symbols:info-outline"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					{m.teams_basic_info_heading()}
				</h3>

				<div class="space-y-6">
					<div>
						<label
							for="team-name"
							class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
						>
							{m.teams_name_label()} <span class="text-red-500">*</span>
						</label>
						<input
							id="team-name"
							type="text"
							bind:value={name}
							placeholder={m.teams_name_placeholder()}
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
							{m.teams_name_hint()}
						</p>
					</div>

					<div>
						<label
							for="team-description"
							class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
						>
							{m.common_description()}
						</label>
						<textarea
							id="team-description"
							bind:value={description}
							rows="3"
							placeholder={m.teams_description_placeholder()}
							class="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all resize-none"
						></textarea>
						<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
							{m.teams_description_hint()}
						</p>
					</div>
				</div>
			</div>

			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
					<IconifyIcon
						icon="material-symbols:sell-outline"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					{m.common_tags()}
					<span class="ml-2 text-xs font-normal text-gray-500">{m.teams_optional_suffix()}</span>
				</h3>
				<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
					{m.teams_tags_hint()}
				</p>
				<TagsInput bind:tags placeholder={m.teams_tags_placeholder()} />
			</div>

			<div
				class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
			>
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center">
					<IconifyIcon
						icon="material-symbols:data-object"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					{m.teams_metadata_heading()}
					<span class="ml-2 text-xs font-normal text-gray-500">{m.teams_optional_suffix()}</span>
				</h3>
				<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
					{m.teams_metadata_hint()}
				</p>
				<MetadataView bind:metadata />
			</div>
		</div>
	{/if}

	<!-- Step 2: Members -->
	{#if currentStep === 2}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
		>
			<div class="flex items-center justify-between mb-4">
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 flex items-center">
					<IconifyIcon
						icon="material-symbols:group-outline"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					{m.teams_members_heading()}
					<span class="ml-2 text-xs font-normal text-gray-500">{m.teams_optional_suffix()}</span>
				</h3>
				<span class="text-xs text-gray-500 dark:text-gray-400">
					{m.teams_members_to_add_count({ count: members.length })}
				</span>
			</div>

			<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
				{m.teams_members_hint()}
			</p>

			<!-- Search box -->
			<div class="relative">
				<IconifyIcon
					icon="material-symbols:search"
					class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400 pointer-events-none"
				/>
				<input
					type="text"
					bind:value={memberQuery}
					oninput={scheduleMemberSearch}
					onkeydown={handleMemberKeydown}
					placeholder={m.teams_member_search_placeholder()}
					class="w-full pl-9 pr-9 py-2.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
				/>
				{#if memberQuery}
					<button
						type="button"
						onclick={() => {
							memberQuery = '';
							memberResults = [];
						}}
						class="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded-md text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
						aria-label={m.teams_clear_search_aria()}
					>
						<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
					</button>
				{/if}

				<!-- Search results dropdown -->
				{#if memberQuery.trim().length >= 2}
					<div
						bind:this={memberDropdownRef}
						class="absolute z-10 mt-1 w-full bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg overflow-hidden max-h-64 overflow-y-auto"
					>
						{#if memberSearchLoading}
							<div
								class="flex items-center justify-center gap-2 px-4 py-6 text-sm text-gray-500 dark:text-gray-400"
							>
								<div
									class="animate-spin rounded-full h-4 w-4 border-2 border-gray-300 border-t-earthy-terracotta-600"
								></div>
								{m.teams_searching()}
							</div>
						{:else if availableResults.length === 0}
							<div class="px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
								{memberResults.length === 0 ? m.teams_no_users_found() : m.teams_all_users_added()}
							</div>
						{:else}
							{#each availableResults as owner, i (owner.id)}
								<button
									type="button"
									data-member-result
									onclick={() => addMember(owner)}
									onmouseenter={() => (focusedMemberIndex = i)}
									class="w-full flex items-center gap-3 px-4 py-2.5 text-left transition-colors
										{i === focusedMemberIndex
										? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20'
										: 'hover:bg-earthy-brown-50 dark:hover:bg-gray-700'}"
								>
									<Avatar name={owner.name} profilePicture={owner.profile_picture} size="sm" />
									<div class="flex-1 min-w-0">
										<div class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
											{owner.name}
										</div>
										{#if owner.email}
											<div class="text-xs text-gray-500 dark:text-gray-400 truncate">
												{owner.email}
											</div>
										{/if}
									</div>
									<IconifyIcon
										icon="material-symbols:add"
										class="h-4 w-4 text-earthy-terracotta-600 dark:text-earthy-terracotta-400 shrink-0"
									/>
								</button>
							{/each}
						{/if}
					</div>
				{/if}
			</div>

			<!-- Members list -->
			<div class="mt-4">
				{#if members.length === 0}
					<div
						class="flex items-center gap-3 text-sm text-gray-500 dark:text-gray-400 px-4 py-6 rounded-lg bg-gray-50 dark:bg-gray-900/40 border border-dashed border-gray-200 dark:border-gray-700"
					>
						<IconifyIcon icon="material-symbols:person-add-outline" class="h-5 w-5 text-gray-400" />
						<span>{m.teams_no_members_yet()}</span>
					</div>
				{:else}
					<ul
						class="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden bg-white dark:bg-gray-800/50"
					>
						{#each members as owner (owner.id)}
							{@const role = memberRoles[owner.id] ?? 'member'}
							<li
								class="flex items-center gap-3 px-4 py-3 border-b border-gray-100 dark:border-gray-700/60 last:border-b-0"
							>
								<Avatar name={owner.name} profilePicture={owner.profile_picture} size="md" />
								<div class="flex-1 min-w-0">
									<div class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
										{owner.name}
									</div>
									{#if owner.email}
										<div class="text-xs text-gray-500 dark:text-gray-400 truncate">
											{owner.email}
										</div>
									{/if}
								</div>
								<button
									type="button"
									onclick={() => toggleMemberRole(owner.id)}
									class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium transition-colors
										{role === 'owner'
										? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/40 text-earthy-terracotta-700 dark:text-earthy-terracotta-300 hover:bg-earthy-terracotta-200'
										: 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'}"
									title={m.teams_toggle_role_title()}
								>
									<IconifyIcon
										icon={role === 'owner'
											? 'material-symbols:star'
											: 'material-symbols:person-outline'}
										class="h-3.5 w-3.5"
									/>
									{role === 'owner' ? m.common_owner() : m.teams_role_member()}
								</button>
								<button
									type="button"
									onclick={() => removeMember(owner.id)}
									class="text-gray-400 hover:text-red-500 dark:hover:text-red-400 p-1 -m-1"
									aria-label={m.common_remove()}
								>
									<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
								</button>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		</div>
	{/if}

	<!-- Step 3: Integrations -->
	{#if currentStep === 3}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
		>
			<div class="flex items-center justify-between mb-4">
				<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 flex items-center">
					<IconifyIcon
						icon="material-symbols:webhook"
						class="h-5 w-5 mr-2 text-earthy-terracotta-600"
					/>
					{m.teams_integrations_heading()}
					<span class="ml-2 text-xs font-normal text-gray-500">{m.teams_optional_suffix()}</span>
				</h3>
				<span class="text-xs text-gray-500 dark:text-gray-400">
					{m.teams_webhooks_configured_count({ count: webhooks.length })}
				</span>
			</div>

			<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
				{m.teams_integrations_hint()}
			</p>

			{#if webhooks.length === 0}
				<div
					class="flex flex-col items-center text-center gap-2 py-8 rounded-lg border border-dashed border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/40 mb-4"
				>
					<IconifyIcon icon="material-symbols:webhook" class="h-8 w-8 text-gray-400" />
					<p class="text-sm font-medium text-gray-700 dark:text-gray-300">
						{m.teams_no_integrations_yet()}
					</p>
					<p class="text-xs text-gray-500 dark:text-gray-400 max-w-sm">
						{m.teams_no_integrations_hint()}
					</p>
				</div>
			{:else}
				<div class="space-y-4 mb-4">
					{#each webhooks as w, i (w.key)}
						<div
							class="rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-gray-50/60 dark:bg-gray-900/30"
						>
							<div
								class="flex items-center justify-between px-4 py-2.5 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/40"
							>
								<div
									class="flex items-center gap-2 text-sm font-medium text-gray-900 dark:text-gray-100"
								>
									<IconifyIcon
										icon={providerOptions().find((p) => p.value === w.provider)?.icon ??
											'mdi:webhook'}
										class="h-4 w-4"
									/>
									{w.name || m.teams_integration_fallback_name({ number: i + 1 })}
								</div>
								<button
									type="button"
									onclick={() => removeWebhook(w.key)}
									class="text-gray-400 hover:text-red-500 dark:hover:text-red-400 p-1 -m-1"
									aria-label={m.teams_remove_integration_aria()}
								>
									<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
								</button>
							</div>
							<div class="p-4 space-y-4">
								<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
									<div>
										<label
											for="wh-name-{w.key}"
											class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1"
										>
											{m.common_name()} <span class="text-red-500">*</span>
										</label>
										<input
											id="wh-name-{w.key}"
											type="text"
											bind:value={w.name}
											placeholder={m.teams_webhook_name_placeholder()}
											class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600"
										/>
									</div>
									<div>
										<label
											for="wh-provider-{w.key}"
											class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1"
										>
											{m.teams_webhook_provider_label()}
										</label>
										<select
											id="wh-provider-{w.key}"
											bind:value={w.provider}
											class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600"
										>
											{#each providerOptions() as opt (opt.value)}
												<option value={opt.value}>{opt.label}</option>
											{/each}
										</select>
									</div>
								</div>

								<div>
									<label
										for="wh-url-{w.key}"
										class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1"
									>
										{m.teams_webhook_url_label()} <span class="text-red-500">*</span>
									</label>
									<input
										id="wh-url-{w.key}"
										type="url"
										bind:value={w.webhook_url}
										placeholder="https://hooks.slack.com/services/..."
										class="w-full px-3 py-2 text-sm font-mono border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600"
									/>
								</div>

								<div>
									<div class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-2">
										{m.teams_notification_types_label()} <span class="text-red-500">*</span>
									</div>
									<div class="grid grid-cols-2 md:grid-cols-3 gap-1.5">
										{#each notificationTypeOptions() as opt (opt.type)}
											{@const selected = w.notification_types.includes(opt.type)}
											<button
												type="button"
												onclick={() => toggleWebhookNotificationType(w.key, opt.type)}
												class="flex items-center gap-1.5 px-2 py-1.5 rounded-md border text-xs text-left transition-colors
													{selected
													? 'border-earthy-terracotta-500 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 text-earthy-terracotta-700 dark:text-earthy-terracotta-300'
													: 'border-gray-200 dark:border-gray-600 text-gray-600 dark:text-gray-400 hover:border-gray-300'}"
											>
												<IconifyIcon icon={opt.icon} class="h-3.5 w-3.5" />
												{opt.label}
											</button>
										{/each}
									</div>
								</div>

								<label
									class="inline-flex items-center gap-2 text-xs text-gray-700 dark:text-gray-300 cursor-pointer"
								>
									<input
										type="checkbox"
										bind:checked={w.enabled}
										class="rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-600 focus:ring-earthy-terracotta-500"
									/>
									{m.common_enabled()}
								</label>
							</div>
						</div>
					{/each}
				</div>
			{/if}

			<button
				type="button"
				onclick={addWebhook}
				class="w-full flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg border border-dashed border-earthy-terracotta-300 dark:border-earthy-terracotta-800 text-earthy-terracotta-700 dark:text-earthy-terracotta-400 hover:border-earthy-terracotta-500 hover:bg-earthy-terracotta-50/60 dark:hover:bg-earthy-terracotta-900/20 transition-colors"
			>
				<IconifyIcon icon="material-symbols:add" class="h-4 w-4" />
				<span class="text-sm font-medium">{m.teams_add_integration()}</span>
			</button>
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
				<div
					class="flex items-center justify-center h-12 w-12 rounded-lg bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/40 text-earthy-terracotta-700 dark:text-earthy-terracotta-300 shrink-0"
				>
					<IconifyIcon icon="material-symbols:groups-outline" class="h-6 w-6" />
				</div>
				<div class="min-w-0">
					<div class="text-xs uppercase tracking-wider text-gray-500 dark:text-gray-400 mb-0.5">
						{m.teams_ready_to_create()}
					</div>
					<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 truncate">
						{name || m.teams_unnamed_team()}
					</h3>
				</div>
			</div>

			<div class="p-6 space-y-5">
				<!-- Description -->
				<div>
					<div
						class="flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5 uppercase tracking-wide"
					>
						<IconifyIcon icon="material-symbols:description-outline" class="h-3.5 w-3.5" />
						{m.common_description()}
					</div>
					{#if description.trim()}
						<p class="text-sm text-gray-900 dark:text-gray-100 leading-relaxed">
							{description}
						</p>
					{:else}
						<p class="text-sm text-gray-400 dark:text-gray-500 italic">{m.teams_not_set()}</p>
					{/if}
				</div>

				<!-- Members -->
				<div class="border-t border-gray-100 dark:border-gray-700/60 pt-5">
					<div class="flex items-center justify-between mb-2.5">
						<div
							class="flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide"
						>
							<IconifyIcon icon="material-symbols:group-outline" class="h-3.5 w-3.5" />
							{m.teams_members_heading()}
						</div>
						<span class="text-xs text-gray-400 dark:text-gray-500">
							{m.teams_members_to_add_count({ count: members.length })}
						</span>
					</div>
					{#if members.length === 0}
						<p class="text-sm text-gray-400 dark:text-gray-500 italic">{m.common_none()}</p>
					{:else}
						<ul class="space-y-1.5">
							{#each members as owner (owner.id)}
								{@const role = memberRoles[owner.id] ?? 'member'}
								<li
									class="flex items-center gap-3 px-3 py-2 rounded-md bg-gray-50 dark:bg-gray-900/40 border border-gray-100 dark:border-gray-700/60"
								>
									<Avatar name={owner.name} profilePicture={owner.profile_picture} size="sm" />
									<div class="flex-1 min-w-0">
										<div class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
											{owner.name}
										</div>
										{#if owner.email}
											<div class="text-xs text-gray-500 dark:text-gray-400 truncate">
												{owner.email}
											</div>
										{/if}
									</div>
									<span
										class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium
											{role === 'owner'
											? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/40 text-earthy-terracotta-700 dark:text-earthy-terracotta-300'
											: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400'}"
									>
										<IconifyIcon
											icon={role === 'owner'
												? 'material-symbols:star'
												: 'material-symbols:person-outline'}
											class="h-2.5 w-2.5"
										/>
										{role === 'owner' ? m.common_owner() : m.teams_role_member()}
									</span>
								</li>
							{/each}
						</ul>
					{/if}
				</div>

				<!-- Tags -->
				<div class="border-t border-gray-100 dark:border-gray-700/60 pt-5">
					<div class="flex items-center justify-between mb-2.5">
						<div
							class="flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide"
						>
							<IconifyIcon icon="material-symbols:sell-outline" class="h-3.5 w-3.5" />
							{m.common_tags()}
						</div>
						<span class="text-xs text-gray-400 dark:text-gray-500">{tags.length}</span>
					</div>
					{#if tags.length === 0}
						<p class="text-sm text-gray-400 dark:text-gray-500 italic">{m.common_none()}</p>
					{:else}
						<div class="flex flex-wrap gap-1.5">
							{#each tags as tag (tag)}
								<span
									class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300"
								>
									{tag}
								</span>
							{/each}
						</div>
					{/if}
				</div>

				<!-- Metadata -->
				<div class="border-t border-gray-100 dark:border-gray-700/60 pt-5">
					<div class="flex items-center justify-between mb-2.5">
						<div
							class="flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide"
						>
							<IconifyIcon icon="material-symbols:data-object" class="h-3.5 w-3.5" />
							{m.teams_metadata_heading()}
						</div>
						<span class="text-xs text-gray-400 dark:text-gray-500">
							{m.teams_metadata_entries_count({ count: Object.keys(metadata ?? {}).length })}
						</span>
					</div>
					{#if Object.keys(metadata ?? {}).length === 0}
						<p class="text-sm text-gray-400 dark:text-gray-500 italic">{m.common_none()}</p>
					{:else}
						<div class="border border-gray-200 dark:border-gray-700 rounded-md overflow-hidden">
							<dl class="divide-y divide-gray-100 dark:divide-gray-700/60">
								{#each Object.entries(metadata ?? {}) as [key, value] (key)}
									<div class="flex items-start gap-3 px-3 py-2 text-xs">
										<dt class="w-1/3 font-mono text-gray-500 dark:text-gray-400 break-all">
											{key}
										</dt>
										<dd class="flex-1 text-gray-900 dark:text-gray-100 break-all">
											{typeof value === 'string' ? value : JSON.stringify(value)}
										</dd>
									</div>
								{/each}
							</dl>
						</div>
					{/if}
				</div>

				<!-- Integrations -->
				<div class="border-t border-gray-100 dark:border-gray-700/60 pt-5">
					<div class="flex items-center justify-between mb-2.5">
						<div
							class="flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide"
						>
							<IconifyIcon icon="material-symbols:webhook" class="h-3.5 w-3.5" />
							{m.teams_integrations_heading()}
						</div>
						<span class="text-xs text-gray-400 dark:text-gray-500">{webhooks.length}</span>
					</div>
					{#if webhooks.length === 0}
						<p class="text-sm text-gray-400 dark:text-gray-500 italic">{m.common_none()}</p>
					{:else}
						<ul class="space-y-1.5">
							{#each webhooks as w (w.key)}
								<li
									class="flex items-center gap-3 px-3 py-2 rounded-md bg-gray-50 dark:bg-gray-900/40 border border-gray-100 dark:border-gray-700/60"
								>
									<IconifyIcon
										icon={providerOptions().find((p) => p.value === w.provider)?.icon ??
											'mdi:webhook'}
										class="h-4 w-4 text-gray-500 dark:text-gray-400 shrink-0"
									/>
									<div class="flex-1 min-w-0">
										<div class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
											{w.name || m.teams_unnamed()}
										</div>
										<div class="text-xs text-gray-500 dark:text-gray-400">
											{providerLabels()[w.provider] ?? w.provider} · {m.teams_webhook_types_count({
												count: w.notification_types.length
											})}
										</div>
									</div>
									<span
										class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium
											{w.enabled
											? 'bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-400'
											: 'bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400'}"
									>
										{w.enabled ? m.common_enabled() : m.common_disabled()}
									</span>
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
					<p class="font-medium text-gray-900 dark:text-gray-100">{m.teams_next_steps_heading()}</p>
					<p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
						{m.teams_next_steps_hint()}
					</p>
				</div>
			</div>
		</div>
	{/if}
</StepperPage>
