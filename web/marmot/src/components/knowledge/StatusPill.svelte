<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { statusOf, type KnowledgeKind, type KnowledgeState } from '#lib/knowledge/kinds.ts';

	// Says when an entry should not be relied on as it stands: not in use yet, no longer in use, or overdue a review. An entry that is simply current gets no pill, as the design system asks for the happy path. The pill's states pick tone and glyph; the entry's own status word goes in the slot.
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
	<mm-status-pill state="historical">{label}</mm-status-pill>
{:else if state === 'pending'}
	<mm-status-pill state="pending"
		>{statusOf(kind, status)?.state === 'current'
			? m.knowledge_status_upcoming()
			: label}</mm-status-pill
	>
{/if}
{#if reviewDue && state !== 'historical'}
	<mm-status-pill state="review-due">{m.knowledge_status_review_due()}</mm-status-pill>
{/if}
