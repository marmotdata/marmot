<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';

	// Creating a key now happens in a blade on the service account page, so this old address forwards there and asks the page to open it
	onMount(() => {
		goto(resolve(`/settings/service-accounts/${page.params.id}?create=1`), { replaceState: true });
	});
</script>
