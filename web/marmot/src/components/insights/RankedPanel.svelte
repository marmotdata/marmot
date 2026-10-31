<script lang="ts" module>
	export interface RankedItem {
		key: string;
		label: string;
		value?: number;
		href?: string;
		secondary?: string;
		provider?: string;
		avatar?: 'person' | 'agent' | 'team';
	}
</script>

<script lang="ts">
	import { formatNumber } from '$lib/utils/format';

	let {
		heading,
		icon,
		emptyText,
		items,
		loading = false,
		linkHref = '',
		linkLabel = ''
	}: {
		heading: string;
		icon: string;
		emptyText: string;
		items: RankedItem[];
		loading?: boolean;
		linkHref?: string;
		linkLabel?: string;
	} = $props();
</script>

<mm-panel
	{heading}
	{icon}
	{loading}
	empty={!loading && items.length === 0}
	link-href={linkHref || undefined}
	link-label={linkLabel || undefined}
>
	<mm-list dense>
		{#each items as item (item.key)}
			<mm-list-item href={item.href || undefined} secondary={item.secondary || undefined}>
				{#if item.provider}
					<mm-provider-logo slot="start" provider={item.provider} size="16" decorative
					></mm-provider-logo>
				{:else if item.avatar}
					<mm-avatar slot="start" kind={item.avatar} name={item.label} size={20} decorative
					></mm-avatar>
				{/if}
				{item.label}
				{#if item.value !== undefined}
					<mm-text slot="end" variant="caption" numeric tone="secondary"
						>{formatNumber(item.value)}</mm-text
					>
				{/if}
			</mm-list-item>
		{/each}
	</mm-list>
	<mm-empty-state slot="empty" compact description={emptyText}></mm-empty-state>
</mm-panel>
