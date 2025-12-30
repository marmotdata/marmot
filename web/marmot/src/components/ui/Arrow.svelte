<script>
	export let expanded = false;
</script>

<div
	on:click|stopPropagation
	class:rotate-180={expanded}
	class="arrow w-4 h-4 transition-transform duration-200 ease-in-out text-gray-700 dark:text-gray-300"
	aria-hidden="true"
>
	<svg class="m-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
		<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
	</svg>
</div>

<style>
	.arrow {
		transform: rotate(0deg);
	}

	.arrow.rotate-180 {
		transform: rotate(180deg);
	}
</style>
