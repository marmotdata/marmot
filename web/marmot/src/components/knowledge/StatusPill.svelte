<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { statusOf, type KnowledgeKind, type KnowledgeState } from '#lib/knowledge/kinds.ts';

	// Says when an entry should not be relied on as it stands: not in use yet, no longer in use, or overdue a review. An entry that is simply current gets no pill, as the design system asks for the happy path.
	// TODO(design-system): knowledge states have no mm-status-pill `state` of their own, so tones and glyphs are set here; move them into the design system as `pending`, `historical` and `review-due` states.
	let {
		kind,
		status,
		state,
		reviewDue = false
	}: {
		kind: KnowledgeKind | undefined;
		status: string;
		state?: KnowledgeState;
		reviewDue?: boolean;
	} = $props();

	let label = $derived(statusOf(kind, status)?.label ?? '');
</script>

{#if !status}
	<mm-status-pill status="neutral" icon="info">{m.knowledge_status_unset()}</mm-status-pill>
{:else if state === 'historical'}
	<mm-status-pill status="neutral" icon="archive">{label}</mm-status-pill>
{:else if state === 'pending'}
	<mm-status-pill status="info" icon="pending"
		>{statusOf(kind, status)?.state === 'current'
			? m.knowledge_status_upcoming()
			: label}</mm-status-pill
	>
{/if}
{#if reviewDue && state !== 'historical'}
	<mm-status-pill status="warning" icon="schedule">{m.knowledge_status_review_due()}</mm-status-pill
	>
{/if}
