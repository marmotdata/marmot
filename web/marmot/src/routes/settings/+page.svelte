<script lang="ts">
	import { resolve } from '$app/paths';
	import ActionCentre from '$components/settings/ActionCentre.svelte';
	import { m } from '$lib/paraglide/messages';
	import { myAccess } from '$lib/iam/myAccess';
	import { mmIcon } from '$lib/mm';
	import { visibleGroups } from '$lib/settings/sections';

	let groups = $derived(visibleGroups($myAccess));
</script>

<mm-page-header heading={m.settings_title()}></mm-page-header>

{#if groups.length === 0 && $myAccess.checked}
	<mm-empty-state
		heading={m.settings_no_access_title()}
		description={m.settings_no_access_description()}
	></mm-empty-state>
{:else}
	<ActionCentre access={$myAccess} />
	<div>
		{#each groups as group (group.id)}
			<mm-section heading={group.label()}>
				<mm-list>
					{#each group.items as item (item.id)}
						<mm-list-item href={resolve(item.href)} secondary={item.summary()}>
							<mm-icon slot="start" name={mmIcon(item.icon)}></mm-icon>{item.label()}
							<mm-icon slot="end" name="chevron-right"></mm-icon>
						</mm-list-item>
					{/each}
				</mm-list>
			</mm-section>
		{/each}
	</div>
{/if}
