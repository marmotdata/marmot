<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from '@iconify/svelte';
	import {
		notificationPreferences,
		type NotificationType
	} from '$lib/stores/notificationPreferences';

	let loading = true;
	let preferences: Record<NotificationType, boolean>;

	const notificationTypes: {
		type: NotificationType;
		label: string;
		description: string;
		icon: string;
	}[] = [
		{
			type: 'system',
			label: 'System',
			description: 'System announcements and updates',
			icon: 'material-symbols:info-outline'
		},
		{
			type: 'schema_change',
			label: 'Schema Changes',
			description: 'Schema changes to assets you own',
			icon: 'material-symbols:schema-outline'
		},
		{
			type: 'upstream_schema_change',
			label: 'Upstream Schema Changes',
			description: 'Schema changes to assets upstream of yours',
			icon: 'material-symbols:arrow-upward-alt'
		},
		{
			type: 'downstream_schema_change',
			label: 'Downstream Schema Changes',
			description: 'Schema changes to assets downstream of yours',
			icon: 'material-symbols:arrow-downward-alt'
		},
		{
			type: 'asset_change',
			label: 'Asset Changes',
			description: 'Metadata changes to assets you own',
			icon: 'material-symbols:database-outline'
		},
		{
			type: 'lineage_change',
			label: 'Lineage Changes',
			description: 'New or removed lineage connections involving your assets',
			icon: 'material-symbols:timeline'
		},
		{
			type: 'mention',
			label: 'Mentions',
			description: 'When someone mentions you',
			icon: 'material-symbols:alternate-email'
		},
		{
			type: 'job_complete',
			label: 'Job Completion',
			description: 'Pipeline job completions',
			icon: 'material-symbols:check-circle-outline'
		}
	];

	notificationPreferences.subscribe((value) => {
		preferences = value;
	});

	onMount(async () => {
		await notificationPreferences.initialize();
		loading = false;
	});

	function handleToggle(type: NotificationType) {
		notificationPreferences.setPreference(type, !preferences[type]);
	}
</script>

<div class="space-y-1">
	{#each notificationTypes as { type, label, description, icon }}
		<div class="flex items-center justify-between py-2">
			<div class="flex items-center gap-3">
				<Icon {icon} class="w-5 h-5 text-gray-400 dark:text-gray-500" />
				<div>
					<div class="text-sm text-gray-900 dark:text-gray-100">{label}</div>
					<div class="text-xs text-gray-500 dark:text-gray-400">{description}</div>
				</div>
			</div>
			<button
				type="button"
				role="switch"
				aria-checked={preferences[type]}
				disabled={loading}
				onclick={() => handleToggle(type)}
				class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors
					{preferences[type] ? 'bg-earthy-terracotta-600' : 'bg-gray-300 dark:bg-gray-600'}
					{loading ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}"
			>
				<span
					class="inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform shadow-sm
						{preferences[type] ? 'translate-x-[18px]' : 'translate-x-0.5'}"
				/>
			</button>
		</div>
	{/each}
</div>
