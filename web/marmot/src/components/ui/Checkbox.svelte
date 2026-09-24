<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	type Props = Omit<HTMLInputAttributes, 'type' | 'class'> & {
		checked?: boolean;
		class?: string;
	};

	let { checked = $bindable(false), class: className = '', disabled, ...rest }: Props = $props();
</script>

<input type="checkbox" bind:checked {disabled} class={className} {...rest} />
