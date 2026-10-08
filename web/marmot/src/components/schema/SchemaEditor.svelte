<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import type { Asset } from '$lib/assets/types';
	import IconifyIcon from '@iconify/svelte';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import SchemaSummary from './SchemaSummary.svelte';
	import { m } from '$lib/paraglide/messages';

	let {
		asset
	}: {
		asset: Asset;
	} = $props();

	let canManageAssets = $derived(auth.hasPermission('assets', 'manage'));

	// Schema state - just store as Record<string, unknown> like the asset
	let schemas = $state<Record<string, unknown>>({});
	let showAddSchema = $state(false);
	let editingKey = $state<string | null>(null);
	let activeTab = $state<string>('');
	let newSchemaName = $state('');
	let newSchemaContent = $state('');
	let editSchemaName = $state('');
	let editSchemaContent = $state('');
	let saving = $state(false);
	let showDeleteModal = $state(false);
	let keyToDelete = $state<string | null>(null);
	let showRawSchema = $state(false);

	// Load schemas from asset
	$effect(() => {
		if (asset?.schema) {
			schemas = asset.schema;
			// Set first schema as active if not set or if current active doesn't exist
			const schemaKeys = Object.keys(schemas);
			if (schemaKeys.length > 0 && (!activeTab || !schemas[activeTab])) {
				activeTab = schemaKeys[0];
			} else if (schemaKeys.length === 0) {
				activeTab = '';
			}
		}
	});

	async function saveSchemas(updatedSchemas: Record<string, unknown>) {
		if (!asset?.id) return;

		saving = true;
		try {
			const response = await fetchApi(`/assets/${asset.id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					schema: updatedSchemas
				})
			});

			if (response.ok) {
				schemas = updatedSchemas;
				asset.schema = updatedSchemas;
			} else {
				const errorData = await response.json();
				console.error('Failed to update schema:', errorData);
				alert(m.schema_update_error({ error: errorData.error || m.schema_error_unknown() }));
			}
		} catch (error) {
			console.error('Error updating schema:', error);
			alert(
				m.schema_update_error_generic({
					error: error instanceof Error ? error.message : m.schema_error_unknown()
				})
			);
		} finally {
			saving = false;
		}
	}

	async function addSchema() {
		if (!newSchemaName.trim() || !newSchemaContent.trim()) {
			return;
		}

		const schemaName = newSchemaName.trim();
		const updatedSchemas = {
			...schemas,
			[schemaName]: newSchemaContent.trim()
		};

		await saveSchemas(updatedSchemas);

		newSchemaName = '';
		newSchemaContent = '';
		showAddSchema = false;
		activeTab = schemaName;
	}

	async function updateSchema(key: string) {
		if (!editSchemaName.trim() || !editSchemaContent.trim()) {
			return;
		}

		const updatedSchemas = { ...schemas };
		// If name changed, delete old key
		if (key !== editSchemaName.trim()) {
			delete updatedSchemas[key];
		}
		updatedSchemas[editSchemaName.trim()] = editSchemaContent.trim();

		await saveSchemas(updatedSchemas);

		editingKey = null;
		editSchemaName = '';
		editSchemaContent = '';
	}

	function startEdit(key: string) {
		editingKey = key;
		editSchemaName = key;
		editSchemaContent =
			typeof schemas[key] === 'string' ? schemas[key] : JSON.stringify(schemas[key], null, 2);
	}

	function cancelEdit() {
		editingKey = null;
		editSchemaName = '';
		editSchemaContent = '';
	}

	function cancelAdd() {
		showAddSchema = false;
		newSchemaName = '';
		newSchemaContent = '';
	}

	function promptDelete(key: string) {
		keyToDelete = key;
		showDeleteModal = true;
	}

	async function confirmDelete() {
		if (keyToDelete === null) return;

		const updatedSchemas = { ...schemas };
		delete updatedSchemas[keyToDelete];

		// If deleting active tab, switch to another
		if (activeTab === keyToDelete) {
			const remainingKeys = Object.keys(updatedSchemas);
			activeTab = remainingKeys.length > 0 ? remainingKeys[0] : '';
		}

		await saveSchemas(updatedSchemas);

		keyToDelete = null;
		showDeleteModal = false;
	}

	function setActiveTab(tabName: string) {
		activeTab = tabName;
	}
</script>

<div class="space-y-4">
	<div class="flex items-center justify-end">
		{#if canManageAssets && !showAddSchema && Object.keys(schemas).length === 0}
			<button
				onclick={() => (showAddSchema = true)}
				class="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-earthy-terracotta-300 dark:border-earthy-terracotta-700 text-earthy-terracotta-700 dark:text-earthy-terracotta-300 hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/20 transition-colors"
			>
				<IconifyIcon icon="material-symbols:add" class="w-4 h-4 mr-1" />
				{m.schema_add_button()}
			</button>
		{/if}
	</div>

	{#if Object.keys(schemas).length === 0 && !showAddSchema}
		<div
			class="text-center py-12 border border-dashed border-gray-300 dark:border-gray-700 rounded-lg"
		>
			<IconifyIcon
				icon="material-symbols:schema-outline"
				class="w-12 h-12 mx-auto text-gray-400 dark:text-gray-600 mb-3"
			/>
			<p class="text-gray-500 dark:text-gray-400">{m.schema_none_defined()}</p>
			{#if canManageAssets}
				<button
					onclick={() => (showAddSchema = true)}
					class="mt-3 text-sm text-earthy-terracotta-600 dark:text-earthy-terracotta-400 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-300"
				>
					{m.schema_add_first()}
				</button>
			{/if}
		</div>
	{/if}

	{#if Object.keys(schemas).length > 1 || showAddSchema}
		<!-- Schema Tabs -->
		<div class="flex items-center justify-between gap-4 mb-4">
			<div class="flex items-center gap-4">
				{#if !showAddSchema && !editingKey}
					<button
						class="inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm font-medium rounded-lg text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-earthy-terracotta-50 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-500 transition-colors"
						onclick={() => (showRawSchema = !showRawSchema)}
					>
						{showRawSchema ? m.schema_show_formatted() : m.schema_show_raw()}
					</button>
				{/if}
				<div class="flex flex-wrap gap-2">
					{#if showAddSchema}
						<button
							class="px-4 py-2 text-sm font-medium rounded-lg border transition-colors bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/20 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 border-earthy-terracotta-200 dark:border-earthy-terracotta-800"
							aria-selected={true}
						>
							<IconifyIcon icon="material-symbols:add" class="w-4 h-4 inline-block mr-1" />
							{m.schema_new_button()}
						</button>
					{/if}
					{#each Object.keys(schemas) as schemaKey (schemaKey)}
						<button
							class="px-4 py-2 text-sm font-medium rounded-lg border transition-colors {activeTab ===
								schemaKey && !showAddSchema
								? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/20 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 border-earthy-terracotta-200 dark:border-earthy-terracotta-800'
								: 'text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800'}"
							aria-selected={activeTab === schemaKey && !showAddSchema}
							onclick={() => {
								showAddSchema = false;
								setActiveTab(schemaKey);
							}}
						>
							{schemaKey}
						</button>
					{/each}
				</div>
			</div>
			{#if canManageAssets}
				<div class="flex gap-2">
					{#if !showAddSchema}
						<button
							onclick={() => (showAddSchema = true)}
							class="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-earthy-terracotta-300 dark:border-earthy-terracotta-700 text-earthy-terracotta-700 dark:text-earthy-terracotta-300 hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/20 transition-colors"
							title={m.schema_add_title()}
						>
							<IconifyIcon icon="material-symbols:add" class="w-4 h-4 mr-1" />
							{m.schema_add_button()}
						</button>
					{/if}
					{#if activeTab && !showAddSchema}
						<button
							onclick={() => startEdit(activeTab)}
							class="p-1.5 text-gray-500 dark:text-gray-400 hover:text-earthy-terracotta-600 dark:hover:text-earthy-terracotta-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
							title={m.schema_edit_title()}
						>
							<IconifyIcon icon="material-symbols:edit-outline" class="w-4 h-4" />
						</button>
						<button
							onclick={() => promptDelete(activeTab)}
							class="p-1.5 text-gray-500 dark:text-gray-400 hover:text-red-600 dark:hover:text-red-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
							title={m.schema_delete_title()}
						>
							<IconifyIcon icon="material-symbols:delete-outline" class="w-4 h-4" />
						</button>
					{/if}
				</div>
			{/if}
		</div>
	{:else if Object.keys(schemas).length === 1 && canManageAssets}
		<!-- Single schema - just show action buttons -->
		<div class="flex items-center justify-between gap-2 mb-4">
			<div>
				{#if !showAddSchema && !editingKey}
					<button
						class="inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm font-medium rounded-lg text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-earthy-terracotta-50 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-500 transition-colors"
						onclick={() => (showRawSchema = !showRawSchema)}
					>
						{showRawSchema ? m.schema_show_formatted() : m.schema_show_raw()}
					</button>
				{/if}
			</div>
			<div class="flex items-center gap-2">
				<button
					onclick={() => (showAddSchema = true)}
					class="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-earthy-terracotta-300 dark:border-earthy-terracotta-700 text-earthy-terracotta-700 dark:text-earthy-terracotta-300 hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/20 transition-colors"
					title={m.schema_add_title()}
				>
					<IconifyIcon icon="material-symbols:add" class="w-4 h-4 mr-1" />
					{m.schema_add_button()}
				</button>
				{#if activeTab}
					<button
						onclick={() => startEdit(activeTab)}
						class="p-1.5 text-gray-500 dark:text-gray-400 hover:text-earthy-terracotta-600 dark:hover:text-earthy-terracotta-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
						title={m.schema_edit_title()}
					>
						<IconifyIcon icon="material-symbols:edit-outline" class="w-4 h-4" />
					</button>
					<button
						onclick={() => promptDelete(activeTab)}
						class="p-1.5 text-gray-500 dark:text-gray-400 hover:text-red-600 dark:hover:text-red-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
						title={m.schema_delete_title()}
					>
						<IconifyIcon icon="material-symbols:delete-outline" class="w-4 h-4" />
					</button>
				{/if}
			</div>
		</div>
	{:else if Object.keys(schemas).length === 1 && !canManageAssets}
		<!-- Single schema - non-editable, just show Show Raw button -->
		<div class="flex items-center mb-4">
			{#if !showAddSchema && !editingKey}
				<button
					class="inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm font-medium rounded-lg text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-earthy-terracotta-50 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-500 transition-colors"
					onclick={() => (showRawSchema = !showRawSchema)}
				>
					{showRawSchema ? m.schema_show_formatted() : m.schema_show_raw()}
				</button>
			{/if}
		</div>
	{/if}

	{#if Object.keys(schemas).length > 0 || showAddSchema}
		<!-- Active Schema Content -->
		{#if showAddSchema}
			<!-- Add New Schema -->
			<div
				class="p-4 space-y-4 border border-gray-200 dark:border-gray-700 rounded-b-lg bg-white dark:bg-gray-800"
			>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						{m.schema_name_label()}
					</label>
					<input
						type="text"
						bind:value={newSchemaName}
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500"
						placeholder={m.schema_name_placeholder()}
					/>
				</div>

				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						{m.schema_content_label()}
					</label>
					<textarea
						bind:value={newSchemaContent}
						rows="12"
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 font-mono text-sm focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500"
						placeholder={m.schema_content_placeholder()}
					></textarea>
				</div>

				<div class="flex gap-2">
					<button
						onclick={addSchema}
						disabled={saving}
						class="px-4 py-2 text-sm font-medium rounded-md bg-earthy-terracotta-600 text-white hover:bg-earthy-terracotta-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
					>
						{saving ? m.schema_saving() : m.schema_add_button()}
					</button>
					<button
						onclick={cancelAdd}
						disabled={saving}
						class="px-4 py-2 text-sm font-medium rounded-md border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
					>
						{m.common_cancel()}
					</button>
				</div>
			</div>
		{:else if activeTab && editingKey === activeTab}
			<!-- Edit mode -->
			<div
				class="p-4 space-y-4 border border-gray-200 dark:border-gray-700 rounded-b-lg bg-white dark:bg-gray-800"
			>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						{m.schema_name_label()}
					</label>
					<input
						type="text"
						bind:value={editSchemaName}
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500"
						placeholder={m.schema_name_placeholder()}
					/>
				</div>

				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						{m.schema_content_label()}
					</label>
					<textarea
						bind:value={editSchemaContent}
						rows="12"
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-gray-50 dark:bg-gray-900 text-gray-900 dark:text-gray-100 font-mono text-sm focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500"
						placeholder={m.schema_content_placeholder()}
					></textarea>
				</div>

				<div class="flex gap-2">
					<button
						onclick={() => updateSchema(activeTab)}
						disabled={saving}
						class="px-4 py-2 text-sm font-medium rounded-md bg-earthy-terracotta-600 text-white hover:bg-earthy-terracotta-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
					>
						{saving ? m.schema_saving() : m.common_save()}
					</button>
					<button
						onclick={cancelEdit}
						disabled={saving}
						class="px-4 py-2 text-sm font-medium rounded-md border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
					>
						{m.common_cancel()}
					</button>
				</div>
			</div>
		{:else if activeTab}
			<!-- View mode -->
			<div>
				{#key activeTab}
					<SchemaSummary schema={{ [activeTab]: schemas[activeTab] }} {showRawSchema} />
				{/key}
			</div>
		{/if}
	{/if}
</div>

<DeleteModal
	show={showDeleteModal}
	title={m.schema_delete_modal_title()}
	message={m.schema_delete_confirm()}
	onConfirm={confirmDelete}
	onCancel={() => {
		showDeleteModal = false;
		keyToDelete = null;
	}}
/>
