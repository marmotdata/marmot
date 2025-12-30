<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { toasts, handleApiError } from '$lib/stores/toast';

	export let onTeamCreated: () => void;

	let name = '';
	let description = '';
	let loading = false;

	async function handleSubmit() {
		loading = true;

		try {
			const response = await fetchApi('/teams', {
				method: 'POST',
				body: JSON.stringify({ name, description })
			});

			if (response.ok) {
				toasts.success(`Team "${name}" created successfully`);
				name = '';
				description = '';
				onTeamCreated();
			} else {
				const errorMsg = await handleApiError(response);
				toasts.error(errorMsg);
			}
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : 'An error occurred');
		} finally {
			loading = false;
		}
	}
</script>

<div
	class="mb-6 bg-white dark:bg-gray-800 p-4 rounded-lg border border-gray-200 dark:border-gray-700"
>
	<h3 class="text-lg font-medium mb-4">Create New Team</h3>

	<form on:submit|preventDefault={handleSubmit} class="space-y-4">
		<div>
			<label for="name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
				Team Name
			</label>
			<input
				id="name"
				type="text"
				bind:value={name}
				required
				class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-600"
				placeholder="Data Engineering"
			/>
		</div>

		<div>
			<label
				for="description"
				class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
			>
				Description
			</label>
			<textarea
				id="description"
				bind:value={description}
				rows="3"
				class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-600"
				placeholder="Team responsible for data platform and pipelines"
			/>
		</div>

		<div class="flex justify-end">
			<button
				type="submit"
				disabled={loading || !name}
				class="px-4 py-2 bg-earthy-terracotta-700 text-white rounded-md hover:bg-earthy-terracotta-800 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 disabled:opacity-50 disabled:cursor-not-allowed"
			>
				{loading ? 'Creating...' : 'Create Team'}
			</button>
		</div>
	</form>
</div>
