<script lang="ts">
	import { fetchApi } from '$lib/api';
	import type { ExternalLink } from '$lib/assets/types';
	import { m } from '$lib/paraglide/messages';
	import { linkIcon } from './assetFormat';

	// "Add link" in the page header's Links row: a popover with the manual links (each removable) and a short form to add one; links a rule added are managed by the rule.
	let { links = $bindable(), assetId }: { links?: ExternalLink[]; assetId: string } = $props();

	let open = $state(false);
	let name = $state('');
	let url = $state('');
	let nameError = $state('');
	let urlError = $state('');
	let saving = $state(false);
	let saveError = $state('');

	async function save(updated: ExternalLink[]): Promise<boolean> {
		saving = true;
		saveError = '';
		try {
			const response = await fetchApi(`/assets/${assetId}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ external_links: updated })
			});
			if (!response.ok) throw new Error();
			links = updated;
			return true;
		} catch {
			saveError = m.asset_page_links_save_error();
			return false;
		} finally {
			saving = false;
		}
	}

	function validate(): boolean {
		nameError = name.trim() ? '' : m.asset_page_link_name_required();
		if (!url.trim()) urlError = m.links_url_required();
		else if (!/^https?:\/\//.test(url.trim())) urlError = m.links_url_invalid_scheme();
		else urlError = '';
		return !nameError && !urlError;
	}

	async function add(e: SubmitEvent) {
		e.preventDefault();
		if (!validate()) return;
		if (await save([...(links ?? []), { name: name.trim(), url: url.trim() }])) {
			name = '';
			url = '';
		}
	}

	function remove(index: number) {
		save((links ?? []).filter((_, i) => i !== index));
	}
</script>

<mm-popover
	label={m.asset_page_links_heading()}
	{open}
	onmm-toggle={(e: CustomEvent<{ open: boolean }>) => (open = e.detail.open)}
	style="--mm-popover-max-width: 26rem"
>
	<mm-button slot="trigger" variant="ghost" size="sm" icon="add"
		>{m.asset_page_add_link()}</mm-button
	>
	<div class="mm-stack" style="--mm-gap: var(--space-4); min-inline-size: 22rem">
		{#if links && links.length > 0}
			<mm-list dense aria-label={m.asset_page_links_heading()}>
				{#each links as link, i (`${link.url}-${i}`)}
					<mm-list-item secondary={link.url}>
						<mm-icon slot="start" name={linkIcon(link.icon)} aria-hidden="true"></mm-icon>
						{link.name}
						<mm-button
							slot="end"
							variant="ghost"
							size="sm"
							icon="close"
							label={m.asset_page_remove_link({ name: link.name })}
							disabled={saving}
							onclick={() => remove(i)}
						></mm-button>
					</mm-list-item>
				{/each}
			</mm-list>
		{/if}
		<form class="mm-stack" style="--mm-gap: var(--space-3)" onsubmit={add} novalidate>
			<mm-text-field
				label={m.asset_page_link_name()}
				value={name}
				error={nameError}
				autofocus
				onmm-input={(e: CustomEvent<{ value: string }>) => (name = e.detail.value)}
			></mm-text-field>
			<mm-text-field
				label={m.asset_page_link_url()}
				type="url"
				value={url}
				error={urlError}
				placeholder="https://"
				onmm-input={(e: CustomEvent<{ value: string }>) => (url = e.detail.value)}
			></mm-text-field>
			{#if saveError}
				<mm-text variant="dense" tone="danger" block role="alert">{saveError}</mm-text>
			{/if}
			<div class="mm-cluster">
				<mm-button type="submit" variant="primary" size="sm" icon="add" loading={saving}
					>{m.asset_page_add_link()}</mm-button
				>
			</div>
		</form>
	</div>
</mm-popover>
