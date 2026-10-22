<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';

	// Creating now happens in a blade on the admin list, so this old address forwards there and asks the list to open it
	onMount(() => {
		goto(resolve('/admin?tab=users&create=1'), { replaceState: true });
	});
</script>
