<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import DeleteModal from './DeleteModal.svelte';
	import Button from './Button.svelte';

	let apiKeys: any[] | null = null;
	let newKeyName = '';
	let keyToDelete: any = null;
	let showDeleteDialog = false;
	let newlyCreatedKey: string | null = null;
	let copied = false;
	let creatingKey = false;
	let isGenerating = false;
	let createKeyError: string | null = null;

	onMount(async () => {
		await fetchApiKeys();
	});

	async function fetchApiKeys() {
		try {
			const response = await fetchApi('/users/apikeys');
			const data = await response.json();
			apiKeys = data === null ? [] : data;
		} catch (err) {
			console.error('Failed to fetch API keys:', err);
			apiKeys = [];
		}
	}

	async function createApiKey() {
		createKeyError = null;

		if (!newKeyName.trim()) {
			createKeyError = 'Key name cannot be empty.';
			return;
		}

		if (apiKeys && apiKeys.some((key) => key.name === newKeyName)) {
			createKeyError = 'An API key with this name already exists.';
			return;
		}

		try {
			isGenerating = true;
			const response = await fetchApi('/users/apikeys', {
				method: 'POST',
				body: JSON.stringify({ name: newKeyName })
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.message || 'Failed to create API key.');
			}

			const result = await response.json();

			apiKeys = [...(apiKeys || []), result];
			newlyCreatedKey = result.key;
			newKeyName = '';
			creatingKey = false;
		} catch (err) {
			console.error(err);
			createKeyError = err.message;
		} finally {
			isGenerating = false;
		}
	}

	async function deleteApiKey() {
		if (!keyToDelete) return;

		try {
			await fetchApi(`/users/apikeys/${keyToDelete.id}`, {
				method: 'DELETE'
			});
			apiKeys = (apiKeys || []).filter((key) => key.id !== keyToDelete.id);
			showDeleteDialog = false;
			keyToDelete = null;
		} catch (err) {
			console.error('Failed to delete API key:', err);
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && newKeyName && !isGenerating) {
			event.preventDefault();
			createApiKey();
		}
	}
</script>

<div
	class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700"
>
	<div class="p-6">
		<div class="flex justify-between items-center mb-6">
			<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">API Keys</h3>
			<Button
				variant="filled"
				text={creatingKey ? 'Cancel' : 'New Key'}
				icon={creatingKey ? 'material-symbols:close' : 'material-symbols:add-2'}
				click={() => {
					creatingKey = !creatingKey;
					newlyCreatedKey = null;
					createKeyError = null;
				}}
			/>
		</div>

		{#if creatingKey}
			<div class="mb-6 bg-earthy-brown-100 dark:bg-gray-800 rounded-lg p-6 animate-slide-down">
				<h4 class="text-base font-medium text-gray-900 dark:text-gray-100 mb-4">
					Create New API Key
				</h4>
				<div class="space-y-4">
					{#if createKeyError}
						<div
							class="bg-red-50 border border-red-200 text-red-800 dark:text-red-100 px-4 py-2 rounded-md mb-4"
						>
							{createKeyError}
						</div>
					{/if}
					<div>
						<label
							for="key-name"
							class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>Key Name</label
						>
						<input
							type="text"
							id="key-name"
							bind:value={newKeyName}
							on:keydown={handleKeydown}
							class="block w-full px-4 py-3 bg-white dark:bg-gray-800 rounded-md shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:focus:border-earthy-terracotta-500 sm:text-sm border-gray-300 dark:border-gray-600"
							placeholder="Enter a descriptive name for your key"
						/>
					</div>
					<Button
						variant="filled"
						text={isGenerating ? 'Generating...' : 'Generate Key'}
						loading={isGenerating}
						disabled={!newKeyName || isGenerating}
						click={createApiKey}
					/>
				</div>
			</div>
		{/if}

		{#if newlyCreatedKey}
			<div class="mb-6 bg-earthy-brown-100 dark:bg-gray-800 rounded-lg p-6 animate-slide-down">
				<div class="flex justify-between items-start mb-3">
					<h4 class="text-base font-medium text-gray-900 dark:text-gray-100">
						New API Key Created
					</h4>
					<Button variant="clear" icon="cross" click={() => (newlyCreatedKey = null)} />
				</div>
				<p class="text-sm text-gray-600 dark:text-gray-400 mb-3">
					Make sure to copy your API key now. You won't be able to see it again!
				</p>
				<div class="flex items-center space-x-2 bg-earthy-brown-50 dark:bg-gray-900 rounded-md p-3">
					<code class="text-sm text-gray-900 dark:text-gray-100 flex-1 font-mono"
						>{newlyCreatedKey}</code
					>
					<Button
						variant="clear"
						text={copied ? 'Copied!' : 'Copy'}
						click={() => {
							navigator.clipboard.writeText(newlyCreatedKey || '');
							copied = true;
							setTimeout(() => (copied = false), 2000);
						}}
					/>
				</div>
			</div>
		{/if}

		{#if apiKeys === null}
			<p class="text-gray-500 dark:text-gray-400 text-sm">Loading...</p>
		{:else if apiKeys.length === 0}
			<p class="text-gray-500 dark:text-gray-400 text-sm">No API keys found.</p>
		{:else}
			<div class="overflow-x-auto">
				<table class="min-w-full">
					<thead>
						<tr>
							<th
								class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
								>Name</th
							>
							<th
								class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
								>Created</th
							>
							<th
								class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
								>Last Used</th
							>
							<th
								class="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
								>Actions</th
							>
						</tr>
					</thead>
					<tbody
						class="divide-y divide-earthy-brown-100 bg-earthy-brown-50 dark:divide-gray-700 dark:bg-gray-900"
					>
						{#each apiKeys as key}
							<tr class="hover:bg-earthy-brown-100 dark:hover:bg-gray-700 transition-colors">
								<td
									class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100"
									>{key.name}</td
								>
								<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600 dark:text-gray-400">
									{new Date(key.created_at).toLocaleString()}
								</td>
								<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600 dark:text-gray-400">
									{key.last_used_at ? new Date(key.last_used_at).toLocaleString() : 'Never'}
								</td>
								<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
									<Button
										variant="clear"
										text="Delete"
										click={() => {
											keyToDelete = key;
											showDeleteDialog = true;
										}}
									/>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
</div>

<DeleteModal
	show={showDeleteDialog}
	title="Delete API Key"
	message="Are you sure you want to delete this API key? This action cannot be undone."
	confirmText="Delete"
	resourceName={keyToDelete?.name || ''}
	requireConfirmation={true}
	onConfirm={deleteApiKey}
	onCancel={() => {
		showDeleteDialog = false;
		keyToDelete = null;
	}}
/>

<style>
	.animate-slide-down {
		animation: slideDown 0.2s ease-out;
	}

	@keyframes slideDown {
		from {
			opacity: 0;
			transform: translateY(-10px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}
</style>
