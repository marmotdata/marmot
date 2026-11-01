<script lang="ts">
	import SearchManagement from '$components/admin/SearchManagement.svelte';
	import QueryProcesses from '$components/admin/QueryProcesses.svelte';
	import { m } from '$lib/paraglide/messages';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { getSystemInfo, type SystemInfo } from '$lib/settings/system';

	// Plugin processes that answer data queries are system health, shown to whoever may read the query log
	let canViewQueryStatus = $derived(holdsAtRoot($myAccess, 'access:view'));

	let info = $state<SystemInfo | null>(null);

	async function loadInfo() {
		try {
			info = await getSystemInfo();
		} catch {
			// Versions are a convenience on this page, so a failed read leaves them out rather than blocking the rest
			info = null;
		}
	}

	$effect(() => {
		if ($myAccess.admin) loadInfo();
	});

	let caption = $derived(
		info
			? [
					m.system_caption_version({ version: info.build.version }),
					info.build.commit
						? m.system_caption_build({ commit: info.build.commit.slice(0, 7) })
						: '',
					m.system_caption_postgres({ version: info.database.version })
				]
					.filter(Boolean)
					.join(' · ')
			: ''
	);
</script>

<mm-page-header heading={m.settings_system()} {caption}></mm-page-header>
{#if $myAccess.admin}
	<SearchManagement index={info?.search_index} onReindexed={loadInfo} />
{/if}
{#if canViewQueryStatus}
	<QueryProcesses />
{/if}
