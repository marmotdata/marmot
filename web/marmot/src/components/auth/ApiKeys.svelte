<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { formatDateTime } from '$lib/utils';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import DatePicker from '$components/ui/DatePicker.svelte';
	import IconifyIcon from '@iconify/svelte';
	import { resolve } from '$app/paths';
	import Panel from '$components/ui/Panel.svelte';
	import Pill from '$components/ui/Pill.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import { myAccess, loadMyAccess } from '$lib/iam/myAccess';
	import { canSee, settingsGroups } from '$lib/settings/sections';
	import {
		BTN_GHOST,
		BTN_PRIMARY,
		BTN_SECONDARY,
		INPUT,
		TABLE,
		TBODY,
		TD,
		TH,
		TR
	} from '$lib/ui/classes';

	interface ApiKey {
		id: string;
		name: string;
		key?: string;
		created_at: string;
		expires_at?: string | null;
		last_used_at: string | null;
	}

	const expirationOptions = [
		{ value: '1', label: m.apikeys_expiration_days_option({ count: 1 }) },
		{ value: '7', label: m.apikeys_expiration_days_option({ count: 7 }) },
		{ value: '30', label: m.apikeys_expiration_days_option({ count: 30 }) },
		{ value: '90', label: m.apikeys_expiration_days_option({ count: 90 }) },
		{ value: '180', label: m.apikeys_expiration_days_option({ count: 180 }) },
		{ value: '365', label: m.apikeys_expiration_days_option({ count: 365 }) },
		{ value: 'custom', label: m.apikeys_expiration_custom() },
		{ value: 'never', label: m.apikeys_expiration_never() }
	];

	let apiKeys = $state<ApiKey[] | null>(null);
	let newKeyName = $state('');
	let expiration = $state('90');
	let customDate = $state('');
	let keyToDelete = $state<ApiKey | null>(null);
	let showDeleteDialog = $state(false);
	let newlyCreatedKey = $state<string | null>(null);
	let copied = $state(false);
	let creatingKey = $state(false);
	let isGenerating = $state(false);

	// Personal keys die with the person, so point anyone who can manage service accounts at the kind of key that outlives them
	const serviceAccountsItem = settingsGroups
		.flatMap((group) => group.items)
		.find((item) => item.id === 'service-accounts');

	onMount(async () => {
		loadMyAccess();
		await fetchApiKeys();
	});

	async function fetchApiKeys() {
		try {
			const response = await fetchApi('/users/apikeys');
			if (!response.ok) {
				const errorMsg = await handleApiError(response);
				toasts.error(errorMsg);
				apiKeys = [];
				return;
			}
			const data = await response.json();
			apiKeys = data === null ? [] : data;
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.apikeys_fetch_error());
			apiKeys = [];
		}
	}

	function startOfToday(): Date {
		const now = new Date();
		return new Date(now.getFullYear(), now.getMonth(), now.getDate());
	}

	function toDateInputValue(date: Date): string {
		const month = String(date.getMonth() + 1).padStart(2, '0');
		const day = String(date.getDate()).padStart(2, '0');
		return `${date.getFullYear()}-${month}-${day}`;
	}

	function daysUntil(dateValue: string): number | null {
		const [year, month, day] = dateValue.split('-').map(Number);
		if (!year || !month || !day) return null;
		const picked = new Date(year, month - 1, day);
		const days = Math.round((picked.getTime() - startOfToday().getTime()) / 86400000);
		return days > 0 ? days : null;
	}

	function resolveExpiresInDays(): number | null {
		if (expiration === 'never') return 0;
		if (expiration === 'custom') {
			return customDate ? daysUntil(customDate) : null;
		}
		return Number(expiration);
	}

	async function createApiKey() {
		if (!newKeyName.trim()) {
			toasts.warning(m.apikeys_error_name_empty());
			return;
		}

		if (apiKeys && apiKeys.some((key) => key.name === newKeyName)) {
			toasts.warning(m.apikeys_error_name_exists());
			return;
		}

		const expiresInDays = resolveExpiresInDays();
		if (expiresInDays === null) {
			toasts.warning(m.apikeys_error_custom_date_future());
			return;
		}

		try {
			isGenerating = true;
			const response = await fetchApi('/users/apikeys', {
				method: 'POST',
				body: JSON.stringify({ name: newKeyName, expires_in_days: expiresInDays })
			});

			if (!response.ok) {
				const errorMsg = await handleApiError(response);
				toasts.error(errorMsg);
				return;
			}

			const result = await response.json();

			apiKeys = [...(apiKeys || []), result];
			newlyCreatedKey = result.key;
			newKeyName = '';
			expiration = '90';
			customDate = '';
			creatingKey = false;
			toasts.success(m.apikeys_created_success());
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.apikeys_create_error());
		} finally {
			isGenerating = false;
		}
	}

	async function deleteApiKey() {
		if (!keyToDelete) return;

		try {
			const response = await fetchApi(`/users/apikeys/${keyToDelete.id}`, {
				method: 'DELETE'
			});
			if (!response.ok) {
				const errorMsg = await handleApiError(response);
				toasts.error(errorMsg);
				return;
			}
			const deleted = keyToDelete;
			toasts.success(m.apikeys_deleted_success({ name: deleted.name }));
			apiKeys = (apiKeys || []).filter((key) => key.id !== deleted.id);
			showDeleteDialog = false;
			keyToDelete = null;
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.apikeys_delete_error());
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && newKeyName && !isGenerating) {
			event.preventDefault();
			createApiKey();
		}
	}
</script>

<div class="space-y-6">
	{#if serviceAccountsItem && canSee($myAccess, serviceAccountsItem)}
		<a
			href={resolve(serviceAccountsItem.href)}
			class="group inline-flex items-center gap-2 text-sm font-medium text-earthy-terracotta-700 hover:text-earthy-terracotta-800 dark:text-earthy-terracotta-400 dark:hover:text-earthy-terracotta-300"
		>
			<IconifyIcon icon="material-symbols:smart-toy-outline" class="h-4 w-4" />
			{m.apikeys_service_account_link()}
			<IconifyIcon
				icon="material-symbols:arrow-forward"
				class="h-4 w-4 transition-transform group-hover:translate-x-0.5"
			/>
		</a>
	{/if}

	{#if newlyCreatedKey}
		<Panel title={m.apikeys_created_heading()} icon="material-symbols:key-outline">
			{#snippet action()}
				<button
					type="button"
					class={BTN_GHOST}
					onclick={() => (newlyCreatedKey = null)}
					aria-label={m.common_close()}
				>
					<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
				</button>
			{/snippet}
			<p class="mb-3 text-sm text-amber-700 dark:text-amber-300">{m.apikeys_copy_warning()}</p>
			<div
				class="flex items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 p-2 pl-3 dark:border-gray-700 dark:bg-gray-900"
			>
				<code
					class="min-w-0 flex-1 select-all break-all font-mono text-sm text-gray-900 dark:text-gray-100"
					>{newlyCreatedKey}</code
				>
				<button
					type="button"
					class={BTN_SECONDARY}
					onclick={() => {
						navigator.clipboard.writeText(newlyCreatedKey || '');
						copied = true;
						setTimeout(() => (copied = false), 2000);
					}}
				>
					<IconifyIcon
						icon={copied ? 'material-symbols:check' : 'material-symbols:content-copy'}
						class="h-4 w-4"
					/>
					{copied ? m.common_copied() : m.common_copy()}
				</button>
			</div>
		</Panel>
	{/if}

	<Panel flush title={m.apikeys_heading()} icon="material-symbols:key-outline">
		{#snippet action()}
			{#if !creatingKey}
				<button
					type="button"
					class="{BTN_PRIMARY} !px-3 !py-1.5"
					onclick={() => {
						creatingKey = true;
						newlyCreatedKey = null;
					}}
				>
					<IconifyIcon icon="material-symbols:add" class="h-4 w-4" />
					{m.apikeys_new_key_button()}
				</button>
			{/if}
		{/snippet}

		{#if creatingKey}
			<div class="border-b border-gray-100 px-5 py-5 dark:border-gray-700/60">
				<h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-gray-100">
					{m.apikeys_create_heading()}
				</h3>
				<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
					<div>
						<label
							for="key-name"
							class="mb-1.5 block text-sm font-medium text-gray-900 dark:text-gray-100"
							>{m.apikeys_name_label()}</label
						>
						<input
							type="text"
							id="key-name"
							bind:value={newKeyName}
							onkeydown={handleKeydown}
							class={INPUT}
							placeholder={m.apikeys_name_placeholder()}
						/>
					</div>
					<div>
						<label
							for="key-expiration"
							class="mb-1.5 block text-sm font-medium text-gray-900 dark:text-gray-100"
							>{m.apikeys_expiration_label()}</label
						>
						<div class="flex flex-wrap items-center gap-3">
							<select id="key-expiration" bind:value={expiration} class="{INPUT} !w-44">
								{#each expirationOptions as option (option.value)}
									<option value={option.value}>{option.label}</option>
								{/each}
							</select>
							{#if expiration === 'custom'}
								<DatePicker
									id="key-custom-date"
									bind:value={customDate}
									min={toDateInputValue(new Date(startOfToday().getTime() + 86400000))}
								/>
								{#if customDate && daysUntil(customDate) !== null}
									<span class="text-xs text-gray-500 dark:text-gray-400">
										{m.apikeys_expires_in_days({ count: daysUntil(customDate) ?? 0 })}
									</span>
								{/if}
							{/if}
						</div>
						{#if expiration === 'never'}
							<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
								{m.apikeys_never_expires_hint()}
							</p>
						{/if}
					</div>
				</div>
				<div class="mt-5 flex justify-end gap-2">
					<button
						type="button"
						class={BTN_SECONDARY}
						onclick={() => (creatingKey = false)}
						disabled={isGenerating}>{m.common_cancel()}</button
					>
					<button
						type="button"
						class={BTN_PRIMARY}
						disabled={!newKeyName || isGenerating || (expiration === 'custom' && !customDate)}
						onclick={createApiKey}
					>
						{isGenerating ? m.apikeys_generating() : m.apikeys_generate_button()}
					</button>
				</div>
			</div>
		{/if}

		{#if apiKeys === null}
			<div class="p-5"><Skeleton class="h-10" rows={3} /></div>
		{:else if apiKeys.length === 0}
			{#if !creatingKey}
				<EmptyState
					dashed={false}
					icon="material-symbols:key-outline"
					title={m.apikeys_none_found()}
				>
					{#snippet action()}
						<button type="button" class={BTN_PRIMARY} onclick={() => (creatingKey = true)}>
							<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
							{m.apikeys_new_key_button()}
						</button>
					{/snippet}
				</EmptyState>
			{/if}
		{:else}
			<div class="overflow-x-auto">
				<table class="{TABLE} w-full">
					<thead>
						<tr class="border-b border-gray-100 dark:border-gray-700/60">
							<th class={TH}>{m.common_name()}</th>
							<th class={TH}>{m.apikeys_column_created()}</th>
							<th class={TH}>{m.apikeys_column_expires()}</th>
							<th class={TH}>{m.apikeys_column_last_used()}</th>
							<th class="{TH} text-right"><span class="sr-only">{m.common_actions()}</span></th>
						</tr>
					</thead>
					<tbody class={TBODY}>
						{#each apiKeys as key (key.id)}
							{@const expired = !!key.expires_at && new Date(key.expires_at) < new Date()}
							<tr class={TR}>
								<td class="{TD} whitespace-nowrap font-medium text-gray-900 dark:text-gray-100"
									>{key.name}</td
								>
								<td class="{TD} whitespace-nowrap tabular-nums text-gray-500 dark:text-gray-400">
									{formatDateTime(key.created_at)}
								</td>
								<td class="{TD} whitespace-nowrap tabular-nums text-gray-500 dark:text-gray-400">
									{#if expired}
										<Pill tone="danger">{m.apikeys_expired_badge()}</Pill>
									{:else}
										{key.expires_at ? formatDateTime(key.expires_at) : m.common_never()}
									{/if}
								</td>
								<td class="{TD} whitespace-nowrap tabular-nums text-gray-500 dark:text-gray-400">
									{key.last_used_at ? formatDateTime(key.last_used_at) : m.common_never()}
								</td>
								<td class="{TD} whitespace-nowrap text-right">
									<button
										type="button"
										class="{BTN_GHOST} hover:!bg-red-50 hover:!text-red-700 dark:hover:!bg-red-950/40 dark:hover:!text-red-300"
										aria-label={m.common_delete()}
										title={m.common_delete()}
										onclick={() => {
											keyToDelete = key;
											showDeleteDialog = true;
										}}
									>
										<IconifyIcon icon="material-symbols:delete-outline" class="h-4 w-4" />
									</button>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</Panel>
</div>

<DeleteModal
	show={showDeleteDialog}
	title={m.apikeys_delete_title()}
	message={m.apikeys_delete_message()}
	confirmText={m.common_delete()}
	resourceName={keyToDelete?.name || ''}
	requireConfirmation={true}
	onConfirm={deleteApiKey}
	onCancel={() => {
		showDeleteDialog = false;
		keyToDelete = null;
	}}
/>
