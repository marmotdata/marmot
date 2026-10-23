<script lang="ts">
	// A placeholder in the shape of what is loading. Preferred to spinners, so the page does not jump when content arrives.
	let { class: extra = 'h-24', rows = 1 }: { class?: string; rows?: number } = $props();
</script>

<div class="space-y-3" aria-hidden="true">
	{#each Array.from({ length: rows }, (_, i) => i) as i (i)}
		<div class="animate-pulse rounded-xl bg-gray-100 dark:bg-gray-800 {extra}"></div>
	{/each}
</div>
