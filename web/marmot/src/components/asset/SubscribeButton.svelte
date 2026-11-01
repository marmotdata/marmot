<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import type { MenuSelectDetail } from '@marmotdata/elements/classes/menu.js';

	interface Props {
		assetId: string;
		/** Kept for callers that still pass it; Subscribe is always a quiet icon toggle now. */
		variant?: 'default' | 'icon-only';
	}

	let { assetId }: Props = $props();

	interface Subscription {
		id: string;
		asset_id: string;
		user_id: string;
		notification_types: string[];
	}

	const notificationTypes = [
		{ type: 'asset_change', label: m.asset_subscribe_type_asset_changes() },
		{ type: 'schema_change', label: m.asset_subscribe_type_schema_changes() },
		{ type: 'upstream_schema_change', label: m.asset_subscribe_type_upstream_schema() },
		{ type: 'downstream_schema_change', label: m.asset_subscribe_type_downstream_schema() },
		{ type: 'lineage_change', label: m.asset_subscribe_type_lineage_changes() },
		{ type: 'asset_deleted', label: m.asset_subscribe_type_asset_deletions() }
	];

	let subscription: Subscription | null = $state(null);
	let loading = $state(true);
	let menuOpen = $state(false);

	let isLoggedIn = $derived(!!auth.getToken());

	onMount(() => {
		if (isLoggedIn) {
			fetchSubscription();
		} else {
			loading = false;
		}
	});

	async function fetchSubscription() {
		try {
			const response = await fetchApi(`/subscriptions?asset_id=${assetId}`);
			if (response.ok) {
				subscription = await response.json();
			}
		} catch {
			// A missing subscription is the normal case, so failures just leave the bell off.
		} finally {
			loading = false;
		}
	}

	async function subscribe() {
		loading = true;
		try {
			const response = await fetchApi('/subscriptions', {
				method: 'POST',
				body: JSON.stringify({
					asset_id: assetId,
					notification_types: ['asset_change', 'schema_change']
				})
			});
			if (response.ok) {
				subscription = await response.json();
			}
		} catch {
			// Leave the toggle off; the person can try again.
		} finally {
			loading = false;
		}
	}

	async function unsubscribe() {
		if (!subscription) return;
		loading = true;
		try {
			const response = await fetchApi(`/subscriptions/${subscription.id}`, { method: 'DELETE' });
			if (response.ok) {
				subscription = null;
				menuOpen = false;
			}
		} catch {
			// Keep the subscription shown as on when the delete fails.
		} finally {
			loading = false;
		}
	}

	async function toggleType(type: string) {
		if (!subscription) return;
		const current = subscription.notification_types;
		let updated: string[];
		if (current.includes(type)) {
			updated = current.filter((t) => t !== type);
			// A subscription needs at least one kind of change.
			if (updated.length === 0) return;
		} else {
			updated = [...current, type];
		}

		try {
			const response = await fetchApi(`/subscriptions/${subscription.id}`, {
				method: 'PUT',
				body: JSON.stringify({ notification_types: updated })
			});
			if (response.ok) {
				subscription = await response.json();
			}
		} catch {
			// The checkbox state follows the server, so nothing to roll back.
		}
	}

	function onSelect(e: CustomEvent<MenuSelectDetail>) {
		if (e.detail.value === 'unsubscribe') {
			unsubscribe();
			return;
		}
		// Keep the menu open so several kinds can be ticked in one go.
		e.preventDefault();
		toggleType(e.detail.value);
	}
</script>

{#if subscription}
	<mm-menu-button
		variant="ghost"
		icon="notifications-active"
		icon-only
		label={m.asset_subscribe_manage_title()}
		open={menuOpen}
		disabled={loading}
	>
		<mm-menu onmm-select={onSelect}>
			<mm-menu-group label={m.asset_page_notify_me_about()}>
				{#each notificationTypes as { type, label } (type)}
					<mm-menu-item
						type="checkbox"
						value={type}
						checked={subscription.notification_types.includes(type)}>{label}</mm-menu-item
					>
				{/each}
			</mm-menu-group>
			<mm-menu-separator></mm-menu-separator>
			<mm-menu-item value="unsubscribe" icon="notifications-off"
				>{m.asset_unsubscribe_button()}</mm-menu-item
			>
		</mm-menu>
	</mm-menu-button>
{:else}
	<mm-button
		variant="ghost"
		icon="notifications"
		label={isLoggedIn
			? m.asset_subscribe_notifications_title()
			: m.asset_subscribe_login_required_title()}
		pressed={false}
		disabled={loading || !isLoggedIn}
		onclick={subscribe}
	></mm-button>
{/if}
