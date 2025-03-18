<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth';

	onMount(() => {
		const params = new URLSearchParams(window.location.search);
		const token = params.get('token');
		const returnTo = params.get('returnTo') || '/';

		if (token) {
			auth.setToken(token);
			goto(returnTo);
		} else {
			goto('/login?error=Authentication failed');
		}
	});
</script>

<div class="min-h-screen flex items-center justify-center bg-earthy-brown-50 dark:bg-gray-900">
	<div
		class="animate-spin rounded-full h-12 w-12 border-b-2 border-gray-900 dark:border-gray-100"
	></div>
</div>
