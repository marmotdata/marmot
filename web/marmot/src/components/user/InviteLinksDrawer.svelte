<script lang="ts">
	import '@marmotdata/elements/drawer.js';
	import '@marmotdata/elements/code-sample.js';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/utils';
	import { inviteLink } from '$lib/users/invites';
	import type { Invitation } from '$lib/users/types';

	// Marmot sends no email, so the links are shown once here for whoever invited the people to pass on. Only a hash of each link is kept, so they cannot be shown again; a lost link is replaced from the member's menu.
	let { invitations, onClose }: { invitations: Invitation[]; onClose: () => void } = $props();

	let expires = $derived(invitations[0] ? formatDate(invitations[0].expires_at) : '');
</script>

<mm-drawer
	open
	heading={m.members_invite_links_heading({ count: invitations.length })}
	description={m.members_invite_links_description({ date: expires })}
	onmm-close={onClose}
>
	<div class="mm-stack" style="--mm-gap: var(--space-5)">
		<mm-callout tone="warning" heading={m.members_invite_links_once()}>
			{m.members_invite_links_once_body()}
		</mm-callout>
		{#each invitations as inv (inv.user.id)}
			<mm-code-sample wrap language="text" label={inv.user.username} code={inviteLink(inv)}
			></mm-code-sample>
		{/each}
	</div>
	<mm-button slot="footer" variant="primary" data-close="done">
		{m.members_invite_links_done()}
	</mm-button>
</mm-drawer>
