<script lang="ts" module>
	import { listPermissions } from '$lib/roles/api';
	import type { Permission } from '$lib/roles/types';

	// Every expanded role needs the same catalogue of permissions, so it is fetched once and shared.
	let shared: Promise<Permission[]> | null = null;
	function catalogue(): Promise<Permission[]> {
		shared ??= listPermissions().catch((err) => {
			shared = null;
			throw err;
		});
		return shared;
	}
</script>

<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { onMount } from 'svelte';
	import { m } from '$lib/paraglide/messages';
	import { summariseRole, type AccessLevel } from '$lib/roles/summary';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import { LABEL } from '$lib/ui/classes';

	// What a role can do, area by area: an access level for each part of Marmot it reaches, the actions behind it, and one quiet line for everything it cannot touch.
	let { grantedKeys }: { grantedKeys: string[] } = $props();

	let all = $state<Permission[] | null>(null);
	let failed = $state(false);

	onMount(() => {
		catalogue()
			.then((list) => (all = list))
			.catch(() => (failed = true));
	});

	let summary = $derived(all ? summariseRole(all, grantedKeys) : []);
	let reached = $derived(
		summary
			.map((c) => ({ ...c, areas: c.areas.filter((a) => a.level !== 'none') }))
			.filter((c) => c.areas.length > 0)
	);
	let unreached = $derived(summary.flatMap((c) => c.areas.filter((a) => a.level === 'none')));

	// Each level has one plain label and a dot whose colour means the same everywhere: green reaches all of the area, blue only reads it, amber reaches some of it.
	const levelStyle: Record<AccessLevel, { label: () => string; dot: string; text: string }> = {
		full: {
			label: m.roles_level_full,
			dot: 'bg-earthy-green-600',
			text: 'text-earthy-green-800 dark:text-earthy-green-400'
		},
		read: {
			label: m.roles_level_read,
			dot: 'bg-earthy-blue-600',
			text: 'text-earthy-blue-800 dark:text-earthy-blue-300'
		},
		partial: {
			label: m.roles_level_partial,
			dot: 'bg-amber-500',
			text: 'text-amber-800 dark:text-amber-300'
		},
		none: { label: m.roles_level_none, dot: 'bg-gray-300', text: 'text-gray-500' }
	};
</script>

{#if failed}
	<p class="text-sm text-gray-500 dark:text-gray-400">{m.roles_summary_load_error()}</p>
{:else if !all}
	<Skeleton class="h-20" rows={2} />
{:else if reached.length === 0}
	<p class="text-sm text-gray-500 dark:text-gray-400">{m.roles_no_permissions_granted()}</p>
{:else}
	<div class="space-y-4">
		{#each reached as category (category.id)}
			<section>
				<h4 class="{LABEL} mb-2">{category.label}</h4>
				<div class="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
					{#each category.areas as area (area.id)}
						<div
							class="rounded-lg border border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-800"
						>
							<div class="flex items-center justify-between gap-2">
								<span
									class="flex min-w-0 items-center gap-2 text-sm font-medium text-gray-900 dark:text-gray-100"
								>
									<IconifyIcon icon={area.icon} class="h-4 w-4 flex-shrink-0 text-gray-400" />
									<span class="truncate">{area.label}</span>
								</span>
								<span
									class="inline-flex flex-shrink-0 items-center gap-1.5 text-xs font-medium {levelStyle[
										area.level
									].text}"
								>
									<span class="h-2 w-2 rounded-full {levelStyle[area.level].dot}" aria-hidden="true"
									></span>
									{levelStyle[area.level].label()}
								</span>
							</div>
							<ul class="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs">
								{#each area.actions as action (action.key)}
									<li
										class="flex items-center gap-1 {action.granted
											? 'text-gray-800 dark:text-gray-200'
											: 'text-gray-500 dark:text-gray-400'}"
										title="{action.key}: {action.description}"
									>
										<IconifyIcon
											icon={action.granted ? 'material-symbols:check' : 'material-symbols:close'}
											class="h-3.5 w-3.5 {action.granted
												? 'text-earthy-green-600 dark:text-earthy-green-500'
												: 'text-red-500 dark:text-red-400'}"
										/>
										<span class="sr-only"
											>{action.granted ? m.roles_summary_can() : m.roles_summary_cannot()}</span
										>
										{action.label}
									</li>
								{/each}
							</ul>
						</div>
					{/each}
				</div>
			</section>
		{/each}
		{#if unreached.length > 0}
			<p class="text-xs text-gray-500 dark:text-gray-400">
				<span class="font-medium">{m.roles_summary_no_access()}</span>
				{unreached.map((a) => a.label).join(', ')}
			</p>
		{/if}
	</div>
{/if}
