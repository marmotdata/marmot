<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/utils';
	import ThemeToggle from '$components/ui/ThemeToggle.svelte';
	import LanguageSelector from '$components/ui/LanguageSelector.svelte';
	import NotificationPreferencesToggle from '$components/ui/NotificationPreferencesToggle.svelte';
	import Panel from '$components/ui/Panel.svelte';
	import Pill from '$components/ui/Pill.svelte';
	import Alert from '$components/ui/Alert.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import Avatar from '$components/user/Avatar.svelte';
	import { LABEL, TABLE, TBODY, TD, TH, TR } from '$lib/ui/classes';

	// A setting row: what it is and why on the left, the control on the right; stacked on narrow screens.
	const ROW =
		'grid gap-3 py-4 first:pt-0 last:pb-0 sm:grid-cols-[minmax(0,18rem)_1fr] sm:items-center sm:gap-8';
	const ROW_LABEL = 'text-sm font-medium text-gray-900 dark:text-gray-100';
	const ROW_HINT = 'mt-0.5 text-xs text-gray-500 dark:text-gray-400';

	interface Permission {
		name: string;
		description: string;
		action: string;
		resource_type: string;
	}

	interface Role {
		name: string;
		permissions: Permission[];
	}

	interface User {
		name: string;
		email: string;
		username?: string;
		profile_picture?: string | null;
		active?: boolean;
		created_at?: string;
		updated_at?: string;
		roles: Role[];
	}

	let loading = $state(true);
	let error = $state<string | null>(null);
	let user = $state<User>({
		name: '',
		email: '',
		roles: []
	});

	// Every permission the caller's roles grant, once each, since two roles often share the same grant
	let permissions = $derived.by(() => {
		const seen: Record<string, { key: string; description: string }> = {};
		for (const role of user.roles ?? []) {
			for (const p of role.permissions ?? []) {
				const key = `${p.resource_type}:${p.action}`;
				seen[key] ??= { key, description: p.description };
			}
		}
		return Object.values(seen).sort((a, b) => a.key.localeCompare(b.key));
	});

	onMount(fetchProfile);

	async function fetchProfile() {
		try {
			loading = true;
			error = null;
			const response = await fetchApi('/users/me');
			if (!response.ok) {
				throw new Error(m.profile_load_error());
			}
			user = await response.json();
		} catch (err) {
			console.error('Profile fetch error:', err);
			error = err instanceof Error ? err.message : m.profile_load_error();
		} finally {
			loading = false;
		}
	}
</script>

{#if loading}
	<div class="space-y-6">
		<Skeleton class="h-24" />
		<Skeleton class="h-40" />
		<Skeleton class="h-64" />
	</div>
{:else if error}
	<Alert>{error}</Alert>
{:else}
	<!-- One column of full-width panels, each a list of rows with what a setting is for on the left and its control on the right, the way Settings reads -->
	<div class="space-y-6">
		<Panel>
			<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
				<div class="flex min-w-0 items-center gap-4">
					<Avatar
						name={user.name || user.username || ''}
						profilePicture={user.profile_picture}
						size="lg"
					/>
					<div class="min-w-0">
						<div class="flex flex-wrap items-center gap-2">
							<p
								class="truncate text-lg font-semibold tracking-tight text-gray-900 dark:text-gray-100"
							>
								{user.name}
							</p>
							<Pill tone={user.active ? 'success' : 'danger'}>
								{user.active ? m.common_active() : m.common_inactive()}
							</Pill>
						</div>
						<p class="truncate text-sm text-gray-500 dark:text-gray-400">@{user.username}</p>
					</div>
				</div>
				<dl class="flex gap-8 text-sm">
					<div>
						<dt class={LABEL}>{m.profile_account_created_label()}</dt>
						<dd class="mt-1 tabular-nums text-gray-900 dark:text-gray-100">
							{user.created_at ? formatDate(user.created_at) : '-'}
						</dd>
					</div>
					<div>
						<dt class={LABEL}>{m.profile_last_updated_label()}</dt>
						<dd class="mt-1 tabular-nums text-gray-900 dark:text-gray-100">
							{user.updated_at ? formatDate(user.updated_at) : '-'}
						</dd>
					</div>
				</dl>
			</div>
		</Panel>

		<Panel title={m.profile_preferences_heading()} icon="material-symbols:tune">
			<div class="divide-y divide-gray-100 dark:divide-gray-700/60">
				<div class={ROW}>
					<div>
						<p class={ROW_LABEL}>{m.profile_theme_label()}</p>
						<p class={ROW_HINT}>{m.account_theme_hint()}</p>
					</div>
					<div><ThemeToggle /></div>
				</div>
				<div class={ROW}>
					<div>
						<p class={ROW_LABEL}>{m.profile_language_label()}</p>
						<p class={ROW_HINT}>{m.account_language_hint()}</p>
					</div>
					<div><LanguageSelector /></div>
				</div>
			</div>
		</Panel>

		<Panel
			title={m.profile_notifications_label()}
			description={m.account_notifications_hint()}
			icon="material-symbols:notifications-outline"
		>
			<NotificationPreferencesToggle />
		</Panel>

		<!-- Roles and Permissions -->
		<Panel
			flush
			title={m.profile_roles_permissions_heading()}
			icon="material-symbols:badge-outline"
		>
			{#snippet action()}
				<div
					class="flex flex-wrap justify-end gap-1.5"
					aria-label={m.profile_assigned_roles_heading()}
				>
					{#each user.roles as role (role.name)}
						<Pill tone="brand">{role.name}</Pill>
					{/each}
				</div>
			{/snippet}
			{#if permissions.length > 0}
				<div class="overflow-x-auto">
					<table class="{TABLE} w-full">
						<thead>
							<tr class="border-b border-gray-100 dark:border-gray-700/60">
								<th class={TH}>{m.profile_permissions_heading()}</th>
								<th class={TH}>{m.common_description()}</th>
							</tr>
						</thead>
						<tbody class={TBODY}>
							<!-- resource_type:action, not permissions.name. The name column is decorative and inconsistent (view_assets beside service_accounts_view), while this pair is the key every check, API call and Terraform binding actually uses. -->
							{#each permissions as permission (permission.key)}
								<tr class={TR}>
									<td
										class="{TD} whitespace-nowrap font-mono text-xs text-gray-900 dark:text-gray-100"
										>{permission.key}</td
									>
									<td class="{TD} text-gray-500 dark:text-gray-400">{permission.description}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</Panel>
	</div>
{/if}
