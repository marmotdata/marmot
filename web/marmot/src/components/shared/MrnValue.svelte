<script lang="ts">
	import { m } from '$lib/paraglide/messages';

	// An MRN with a copy button beside it, wrapping only after its separators so a long value breaks at a segment boundary.
	let { value }: { value: string } = $props();

	// An MRN has no spaces, so offer line breaks after its separators; the browser then wraps at a segment boundary instead of inside a word
	function chunks(v: string): string[] {
		return v.split(/(?<=[/@.])/);
	}
</script>

<span class="mrn">
	<span class="mrn-text"
		>{#each chunks(value) as part, i (i)}{part}<wbr />{/each}</span
	>
	<mm-copy-button {value} label={m.asset_identity_copy_mrn()} size="sm"></mm-copy-button>
</span>

<style>
	/* Breaks come from the wbr after each separator; anywhere is only the fallback for a segment wider than the column. The copy button stays beside the MRN */
	.mrn {
		display: flex;
		align-items: flex-start;
		gap: var(--space-2);
		min-width: 0;
	}
	.mrn-text {
		min-width: 0;
		overflow-wrap: anywhere;
	}
	.mrn > mm-copy-button {
		flex-shrink: 0;
	}
</style>
