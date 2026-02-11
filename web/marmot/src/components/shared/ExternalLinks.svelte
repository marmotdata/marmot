<script lang="ts">
	import { fetchApi } from '$lib/api';
	import type { ExternalLink, EnrichedExternalLink } from '$lib/assets/types';
	import Button from '$components/ui/Button.svelte';
	import IconPicker from '$components/shared/IconPicker.svelte';
	import IconifyIcon from '@iconify/svelte';

	let {
		links = $bindable([]),
		enrichedLinks = [],
		endpoint,
		id,
		canEdit = false,
		saving = $bindable(false)
	}: {
		links: ExternalLink[];
		enrichedLinks?: EnrichedExternalLink[];
		endpoint?: string;
		id?: string;
		canEdit?: boolean;
		saving?: boolean;
	} = $props();

	// Rule-managed links are those with source !== 'asset'
	let ruleManagedLinks = $derived(enrichedLinks.filter((l) => l.source !== 'asset'));

	let newName = $state('');
	let newUrl = $state('');
	let newIcon = $state('');
	let showAddForm = $state(false);
	let urlError = $state('');

	const isLocalMode = $derived(!endpoint || !id);

	function validateUrl(url: string): boolean {
		if (!url.trim()) {
			urlError = 'URL is required';
			return false;
		}
		if (!url.startsWith('http://') && !url.startsWith('https://')) {
			urlError = 'URL must start with http:// or https://';
			return false;
		}
		urlError = '';
		return true;
	}

	async function addLink() {
		if (!newName.trim()) return;
		if (!validateUrl(newUrl)) return;

		const linkToAdd: ExternalLink = { name: newName.trim(), url: newUrl.trim() };
		if (newIcon.trim()) {
			linkToAdd.icon = newIcon.trim();
		}

		if (isLocalMode) {
			links = [...links, linkToAdd];
			resetForm();
			return;
		}

		saving = true;
		try {
			const updatedLinks = [...links, linkToAdd];

			const response = await fetchApi(`${endpoint}/${id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					external_links: updatedLinks
				})
			});

			if (response.ok) {
				links = updatedLinks;
				resetForm();
			} else {
				console.error('Failed to add external link');
			}
		} catch (error) {
			console.error('Error adding external link:', error);
		} finally {
			saving = false;
		}
	}

	async function removeLink(index: number) {
		if (isLocalMode) {
			links = links.filter((_, i) => i !== index);
			return;
		}

		saving = true;
		try {
			const updatedLinks = links.filter((_, i) => i !== index);

			const response = await fetchApi(`${endpoint}/${id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					external_links: updatedLinks
				})
			});

			if (response.ok) {
				links = updatedLinks;
			} else {
				console.error('Failed to remove external link');
			}
		} catch (error) {
			console.error('Error removing external link:', error);
		} finally {
			saving = false;
		}
	}

	function resetForm() {
		showAddForm = false;
		newName = '';
		newUrl = '';
		newIcon = '';
		urlError = '';
	}
</script>

{#if links.length > 0 || ruleManagedLinks.length > 0 || canEdit}
<div class="space-y-2">
	<div class="flex gap-1.5 items-center flex-wrap">
		{#each links as link, i}
			<div class="relative group">
				<Button
					icon={link.icon || 'material-symbols:link'}
					text={link.name}
					variant="clear"
					href={link.url}
					target="_blank"
				/>
				{#if canEdit}
					<button
						onclick={() => removeLink(i)}
						disabled={saving}
						class="absolute -top-1.5 -right-1.5 hidden group-hover:flex items-center justify-center w-4 h-4 rounded-full bg-red-500 text-white hover:bg-red-600 disabled:opacity-50 shadow-sm"
						aria-label="Remove link {link.name}"
					>
						<IconifyIcon icon="material-symbols:close-rounded" class="w-3 h-3" aria-hidden="true" />
					</button>
				{/if}
			</div>
		{/each}

		{#each ruleManagedLinks as ruleLink}
			<Button
				icon={ruleLink.icon || 'material-symbols:link'}
				text={ruleLink.name}
				variant="clear"
				href={ruleLink.url}
				target="_blank"
			/>
		{/each}

		{#if canEdit && !showAddForm}
			<button
				onclick={() => (showAddForm = true)}
				disabled={saving}
				class="inline-flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm font-medium border border-dashed border-gray-300 dark:border-gray-600 text-gray-400 dark:text-gray-500 hover:border-earthy-terracotta-400 dark:hover:border-earthy-terracotta-600 hover:text-earthy-terracotta-600 dark:hover:text-earthy-terracotta-400 hover:bg-earthy-terracotta-50/50 dark:hover:bg-earthy-terracotta-900/10 disabled:opacity-50 transition-colors"
			>
				<IconifyIcon icon="material-symbols:add-link" class="w-4 h-4" aria-hidden="true" />
				Add link
			</button>
		{/if}
	</div>

	{#if showAddForm}
		<div class="flex items-center gap-2 flex-wrap">
			<IconPicker bind:value={newIcon} />
			<input
				type="text"
				bind:value={newName}
				onkeydown={(e) => {
					if (e.key === 'Escape') resetForm();
				}}
				placeholder="Link name"
				aria-label="Link name"
				class="w-36 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:ring-1 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-600"
				autofocus
			/>
			<input
				type="text"
				bind:value={newUrl}
				onkeydown={(e) => {
					if (e.key === 'Enter') addLink();
					if (e.key === 'Escape') resetForm();
				}}
				oninput={() => {
					if (urlError) urlError = '';
				}}
				placeholder="https://..."
				aria-label="Link URL"
				class="w-56 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:ring-1 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-600 {urlError
					? 'border-red-400 dark:border-red-500'
					: ''}"
			/>
			<div class="flex items-center gap-1.5">
				<button
					onclick={addLink}
					disabled={saving || !newName.trim() || !newUrl.trim()}
					class="inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium rounded-lg bg-earthy-terracotta-700 text-white hover:bg-earthy-terracotta-800 disabled:opacity-40 transition-colors"
					aria-label="Add link"
				>
					<IconifyIcon icon="material-symbols:check-rounded" class="w-4 h-4" aria-hidden="true" />
					Add
				</button>
				<button
					onclick={resetForm}
					disabled={saving}
					class="inline-flex items-center px-3 py-2 text-sm font-medium rounded-lg text-gray-600 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
					aria-label="Cancel adding link"
				>
					Cancel
				</button>
			</div>
			{#if urlError}
				<p class="text-xs text-red-500 dark:text-red-400">{urlError}</p>
			{/if}
		</div>
	{/if}
</div>
{/if}
