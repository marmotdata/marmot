<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import type { MyAccess } from '$lib/iam/myAccess';
	import { attentionChecks, publishAttention, type Attention } from '$lib/settings/attention';

	// What an administrator should look at first: what has already broken, then what is about to. Each check runs on its own, so one failing or forbidden request only hides its own rows.
	let { access }: { access: MyAccess } = $props();

	let attention = $state<Attention[]>([]);
	let pendingChecks = $state(0);
	let failedChecks = $state(0);
	let started = false;

	function check(run: () => Promise<Attention[]>) {
		pendingChecks++;
		run()
			.then((rows) => (attention = [...attention, ...rows]))
			.catch(() => failedChecks++)
			.finally(() => {
				pendingChecks--;
				// The overview has just looked, so the gear and the sub-navigation follow it rather than waiting out their own interval
				if (pendingChecks === 0) publishAttention(attention);
			});
	}

	// The checks wait until the caller's permissions have been read so they run once
	$effect(() => {
		if (access.checked && !started) {
			started = true;
			for (const run of attentionChecks(access)) check(run);
		}
	});

	const RANK = { danger: 0, warning: 1, info: 2 } as const;
	let sorted = $derived([...attention].sort((a, b) => RANK[a.tone] - RANK[b.tone]));
	let hasChecks = $derived(attentionChecks(access).length > 0);
	let loading = $derived(pendingChecks > 0 && sorted.length === 0);
</script>

{#if hasChecks}
	<mm-panel
		heading={m.settings_attention_heading()}
		icon="flag"
		count={loading ? undefined : sorted.length}
		{loading}
		empty={!loading && sorted.length === 0}
	>
		<mm-list style="--mm-list-lead-size: 6.5rem">
			{#each sorted as row (row.id)}
				<mm-list-item secondary={row.detail ?? ''}>
					<mm-status-pill
						slot="start"
						status={row.tone}
						icon={row.tone === 'danger' ? 'error' : row.tone === 'warning' ? 'schedule' : 'info'}
						>{row.pill}</mm-status-pill
					>
					{row.title}
					<mm-button slot="end" size="sm" href={resolve(row.href as '/')}>{row.action}</mm-button>
				</mm-list-item>
			{/each}
		</mm-list>
		<mm-empty-state
			slot="empty"
			compact
			heading={failedChecks > 0
				? m.settings_attention_partial_title()
				: m.settings_attention_all_clear()}
			description={failedChecks > 0
				? m.settings_attention_partial_description()
				: m.settings_attention_all_clear_description()}
		></mm-empty-state>
		{#if failedChecks > 0 && sorted.length > 0}
			<p slot="footer" class="mm-caption m-0">{m.settings_attention_partial_description()}</p>
		{/if}
	</mm-panel>
{/if}
