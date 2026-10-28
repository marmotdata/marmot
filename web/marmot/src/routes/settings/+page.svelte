<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { resolve } from '$app/paths';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import ActionCentre from '$components/settings/ActionCentre.svelte';
	import { PANEL } from '$lib/ui/classes';
	import { m } from '$lib/paraglide/messages';
	import { myAccess } from '$lib/iam/myAccess';
	import { visibleGroups } from '$lib/settings/sections';

	let groups = $derived(visibleGroups($myAccess));
</script>

<svelte:head>
	<title>{m.settings_title()}</title>
</svelte:head>

<PageHeader title={m.settings_title()} description={m.settings_description()} />

{#if groups.length === 0 && $myAccess.checked}
	<EmptyState
		icon="material-symbols:lock-outline"
		title={m.settings_no_access_title()}
		description={m.settings_no_access_description()}
	/>
{:else}
	{#if groups.length > 0}
		<div class="mb-8">
			<ActionCentre access={$myAccess} />
		</div>
	{/if}
	<div class="space-y-8">
		{#each groups as group (group.id)}
			<section>
				<h2
					class="mb-3 text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400"
				>
					{group.label()}
				</h2>
				<div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
					{#each group.items as item (item.id)}
						<a
							href={resolve(item.href)}
							class="{PANEL} group flex items-start gap-3 p-4 transition hover:border-gray-300 dark:hover:border-gray-600"
						>
							<span
								class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg border border-gray-100 bg-gray-50 dark:border-gray-700 dark:bg-gray-900"
							>
								<IconifyIcon icon={item.icon} class="h-5 w-5 text-gray-500" />
							</span>
							<span class="min-w-0 flex-1">
								<span class="block text-sm font-semibold text-gray-900 dark:text-gray-100"
									>{item.label()}</span
								>
								<span class="mt-0.5 block text-xs text-gray-500 line-clamp-2 dark:text-gray-400"
									>{item.description()}</span
								>
							</span>
							<IconifyIcon
								icon="material-symbols:chevron-right"
								class="h-5 w-5 flex-shrink-0 text-gray-300 transition group-hover:text-earthy-terracotta-600"
							/>
						</a>
					{/each}
				</div>
			</section>
		{/each}
	</div>
{/if}
