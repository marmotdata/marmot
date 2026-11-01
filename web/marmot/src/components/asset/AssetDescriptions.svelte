<script lang="ts">
	import { fetchApi } from '$lib/api';
	import type { Asset } from '$lib/assets/types';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';

	// The "About" blade section: the plugin's description and, unless the page header already shows them, the notes people wrote.
	let {
		asset,
		editable = true,
		showNotes = true
	}: { asset: Asset; editable?: boolean; showNotes?: boolean } = $props();

	let canManageAssets = $derived(editable && auth.hasPermission('assets', 'manage'));

	let userDescription = $state('');
	let isEditingUserDesc = $state(false);
	let savingUserDesc = $state(false);

	$effect(() => {
		userDescription = asset?.user_description || '';
	});

	let hasNotes = $derived(showNotes && (!!asset.user_description || canManageAssets));

	async function saveUserDescription(valueToSave?: string) {
		if (!asset?.id) return;
		const finalValue = (valueToSave !== undefined ? valueToSave : userDescription).trim();

		savingUserDesc = true;
		try {
			const response = await fetchApi(`/assets/${asset.id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ user_description: finalValue })
			});
			if (response.ok) {
				if (finalValue) {
					asset.user_description = finalValue;
				} else {
					delete asset.user_description;
				}
				userDescription = finalValue;
				isEditingUserDesc = false;
			}
		} catch (error) {
			console.error('Error saving user description:', error);
		} finally {
			savingUserDesc = false;
		}
	}

	function cancelEdit() {
		userDescription = asset.user_description || '';
		isEditingUserDesc = false;
	}
</script>

{#if asset.description || hasNotes}
	<mm-blade-section heading={m.asset_page_about_heading()}>
		{#if showNotes && canManageAssets && !isEditingUserDesc}
			<mm-button
				slot="actions"
				variant="ghost"
				size="sm"
				icon="edit"
				label={m.discover_edit_notes_title()}
				onclick={() => (isEditingUserDesc = true)}
			></mm-button>
		{/if}
		<div class="mm-stack" style="--mm-gap: var(--space-3)">
			{#if showNotes}
				{#if isEditingUserDesc}
					<mm-textarea
						label={m.asset_page_notes_label()}
						hide-label
						rows={4}
						autosize
						value={userDescription}
						placeholder={m.asset_page_notes_placeholder()}
						onmm-input={(e: CustomEvent<{ value: string }>) => (userDescription = e.detail.value)}
					></mm-textarea>
					<div class="mm-cluster">
						<mm-button
							variant="primary"
							size="sm"
							loading={savingUserDesc}
							onclick={() => saveUserDescription()}>{m.asset_page_save_notes()}</mm-button
						>
						<mm-button variant="ghost" size="sm" disabled={savingUserDesc} onclick={cancelEdit}
							>{m.common_cancel()}</mm-button
						>
						{#if asset.user_description}
							<mm-button
								class="mm-push"
								variant="ghost"
								size="sm"
								disabled={savingUserDesc}
								onclick={() => saveUserDescription('')}>{m.asset_page_remove_notes()}</mm-button
							>
						{/if}
					</div>
				{:else if asset.user_description}
					<mm-text variant="dense" block style="white-space: pre-wrap"
						>{asset.user_description}</mm-text
					>
				{:else if canManageAssets}
					<mm-text variant="dense" tone="secondary" block>{m.asset_page_no_notes()}</mm-text>
				{/if}
			{/if}
			{#if asset.description}
				<div class="mm-stack" style="--mm-gap: var(--space-1); align-items: flex-start">
					<mm-tag variant="plugin"></mm-tag>
					<mm-text
						variant="dense"
						tone={showNotes && asset.user_description ? 'secondary' : 'default'}
						block
						style="white-space: pre-wrap">{asset.description}</mm-text
					>
				</div>
			{/if}
		</div>
	</mm-blade-section>
{/if}
