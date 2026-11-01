<script lang="ts">
	import '@marmotdata/elements/key-value-editor.js';
	import { pairsToObject, type KeyValuePair } from '@marmotdata/elements/key-value-editor.js';
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { toast } from '@marmotdata/elements/toast.js';
	import { fetchApi } from '$lib/api';
	import { setCrumbs } from '$lib/mm';
	import { m } from '$lib/paraglide/messages';
	import { userProfile } from '$lib/stores/user';
	import type { Owner, RuleInput } from '$lib/dataproducts/types';
	import type { Asset } from '$lib/assets/types';
	import { assetCaption, assetProvider } from '$lib/dataproducts/assets';
	import { useUnsavedChangesGuard } from '$lib/utils/unsavedChanges.svelte';
	import OwnerField from './OwnerField.svelte';
	import TagField from './TagField.svelte';
	import AssetPicker from './AssetPicker.svelte';
	import RuleEditor from './RuleEditor.svelte';

	// Creating opens in place in the finished page's order: the name and owners first, then what is in it, with tags and metadata behind their own Add buttons.

	setCrumbs([
		{ label: m.products_heading(), href: resolve('/products') },
		{ label: m.products_new_title() }
	]);

	let name = $state('');
	let description = $state('');
	let owners = $state<Owner[]>([]);
	let tags = $state<string[]>([]);
	let metadataPairs = $state<KeyValuePair[]>([]);
	let assets = $state<Asset[]>([]);
	let rules = $state<{ key: number; rule: RuleInput }[]>([]);
	// Editors are kept by key, not position, so removing a rule never hands its neighbour the wrong state.
	const ruleEditors: Record<number, RuleEditor | null> = {};
	let nextRuleKey = 0;
	let picking = $state(false);
	let showTags = $state(false);
	let showMetadata = $state(false);
	let createAnother = $state(false);

	let nameError = $state('');
	let ownersError = $state('');
	let error = $state('');
	let saving = $state(false);
	let submitted = false;
	let defaultOwnerKey = '';

	let nameField = $state<HTMLElement | null>(null);

	let isDirty = $derived(
		!!name.trim() ||
			!!description.trim() ||
			tags.length > 0 ||
			metadataPairs.some((p) => p.key.trim()) ||
			assets.length > 0 ||
			rules.length > 0 ||
			owners.some((o) => `${o.type}:${o.id}` !== defaultOwnerKey)
	);

	const guard = useUnsavedChangesGuard(() => !submitted && !saving && isDirty);

	onMount(() => {
		setDefaultOwner();
	});

	// The person creating it is the likeliest owner, so they start as one and can be swapped out.
	async function setDefaultOwner() {
		const me = await userProfile.load();
		if (!me || owners.length > 0) return;
		defaultOwnerKey = `user:${me.id}`;
		owners = [
			{
				id: me.id,
				name: me.name || me.display_name || me.username,
				username: me.username,
				type: 'user',
				profile_picture: me.profile_picture
			}
		];
	}

	function emptyRule(): RuleInput {
		return {
			name: '',
			description: '',
			rule_type: 'query',
			query_expression: '',
			is_enabled: true
		};
	}

	function addRule() {
		rules = [...rules, { key: nextRuleKey++, rule: emptyRule() }];
	}

	function removeRule(key: number) {
		rules = rules.filter((r) => r.key !== key);
		delete ruleEditors[key];
	}

	function addAsset(asset: Asset) {
		if (!assets.some((a) => a.id === asset.id)) assets = [...assets, asset];
	}

	function removeAsset(asset: Asset) {
		const index = assets.findIndex((a) => a.id === asset.id);
		assets = assets.filter((a) => a.id !== asset.id);
		toast({
			message: m.products_asset_unpicked({ name: asset.name }),
			action: m.products_undo(),
			onAction: () => {
				const next = [...assets];
				next.splice(Math.min(index, next.length), 0, asset);
				assets = next;
			}
		});
	}

	function checkName() {
		nameError = name.trim() ? '' : m.products_name_required();
		return !nameError;
	}

	function checkOwners() {
		ownersError = owners.length > 0 ? '' : m.products_owners_required();
		return !ownersError;
	}

	function reset() {
		name = '';
		description = '';
		tags = [];
		metadataPairs = [];
		assets = [];
		rules = [];
		picking = false;
		showTags = false;
		showMetadata = false;
		error = '';
	}

	async function handleSubmit(event: Event) {
		event.preventDefault();
		const nameOk = checkName();
		const ownersOk = checkOwners();
		const rulesOk = rules.map((r) => ruleEditors[r.key]?.check() ?? true).every(Boolean);
		if (!nameOk) {
			nameField?.focus();
			return;
		}
		if (!ownersOk || !rulesOk) return;

		saving = true;
		error = '';
		try {
			const metadata = pairsToObject(metadataPairs);
			const res = await fetchApi('/products/', {
				method: 'POST',
				body: JSON.stringify({
					name: name.trim(),
					description: description.trim() || undefined,
					owners: owners.map((o) => ({ id: o.id, type: o.type })),
					tags,
					metadata: Object.keys(metadata).length > 0 ? metadata : undefined,
					rules:
						rules.length > 0
							? rules.map(({ rule: r }) => ({
									name: r.name.trim(),
									description: r.description?.trim() || undefined,
									rule_type: r.rule_type,
									query_expression: r.query_expression?.trim(),
									is_enabled: r.is_enabled
								}))
							: undefined
				})
			});
			if (!res.ok) {
				const data = await res.json().catch(() => ({}));
				throw new Error(data.error || m.products_error_create());
			}
			const created = await res.json();
			if (assets.length > 0) {
				await fetchApi(`/products/assets/${created.id}`, {
					method: 'POST',
					body: JSON.stringify({ asset_ids: assets.map((a) => a.id) })
				});
			}
			if (createAnother) {
				toast({ tone: 'success', message: m.products_created({ name: created.name }) });
				reset();
				await tick();
				nameField?.focus();
				return;
			}
			submitted = true;
			goto(resolve('/products/[id]', { id: created.id }));
		} catch (e) {
			error = e instanceof Error ? e.message : m.products_error_create();
		} finally {
			saving = false;
		}
	}
</script>

<div class="mm-stack create">
	<mm-page-header heading={m.products_new_title()} description={m.products_new_description()}
	></mm-page-header>

	<form class="mm-stack" style="--mm-gap: var(--space-5)" onsubmit={handleSubmit} novalidate>
		<!-- svelte-ignore a11y_autofocus -->
		<mm-text-field
			bind:this={nameField}
			label={m.common_name()}
			name="name"
			required
			autofocus
			value={name}
			error={nameError}
			placeholder={m.products_name_placeholder()}
			oninput={(e: Event) => {
				name = (e.target as HTMLInputElement).value;
				if (nameError && name.trim()) nameError = '';
			}}
			onmm-change={checkName}
		></mm-text-field>

		<OwnerField
			bind:owners
			label={m.common_owners()}
			help={m.products_owners_help()}
			error={owners.length > 0 ? '' : ownersError}
		/>

		<mm-textarea
			label={m.common_description()}
			name="description"
			optional
			rows={4}
			value={description}
			placeholder={m.products_description_placeholder()}
			oninput={(e: Event) => (description = (e.target as HTMLTextAreaElement).value)}
		></mm-textarea>

		<mm-section heading={m.products_whats_in_it()} description={m.products_whats_in_it_hint()}>
			<div class="mm-stack" style="--mm-gap: var(--space-4)">
				{#each rules as entry (entry.key)}
					<RuleEditor
						bind:this={ruleEditors[entry.key]}
						bind:rule={entry.rule}
						heading={m.products_rule_heading()}
						onRemove={() => removeRule(entry.key)}
					/>
				{/each}

				{#if assets.length > 0}
					<mm-list aria-label={m.products_picked_assets()} style="--mm-list-inset: 0">
						{#each assets as asset (asset.id)}
							<mm-list-item secondary={assetCaption(asset)}>
								<mm-provider-logo slot="start" provider={assetProvider(asset)} size={20} decorative
								></mm-provider-logo>
								{asset.name}
								<mm-button
									slot="end"
									size="sm"
									variant="ghost"
									label={m.products_unpick_named({ name: asset.name })}
									onclick={() => removeAsset(asset)}>{m.common_remove()}</mm-button
								>
							</mm-list-item>
						{/each}
					</mm-list>
				{/if}

				{#if picking}
					<AssetPicker
						exclude={assets.map((a) => a.id)}
						onAdd={addAsset}
						onClose={() => (picking = false)}
					/>
				{/if}

				<div class="mm-cluster" style="--mm-gap: var(--space-2)">
					{#if !picking}
						<mm-button size="sm" icon="add" onclick={() => (picking = true)}
							>{m.products_pick_assets()}</mm-button
						>
					{/if}
					<mm-button size="sm" icon="add" onclick={addRule}
						>{rules.length > 0 ? m.products_add_another_rule() : m.products_add_rule()}</mm-button
					>
				</div>
			</div>
		</mm-section>

		{#if showTags}
			<TagField bind:tags label={m.common_tags()} />
		{/if}
		{#if showMetadata}
			<mm-key-value-editor
				label={m.products_tab_metadata()}
				value={metadataPairs}
				json
				onmm-change={(e: CustomEvent<{ value: KeyValuePair[] }>) =>
					(metadataPairs = e.detail.value)}
			></mm-key-value-editor>
		{/if}
		{#if !showTags || !showMetadata}
			<div class="mm-cluster more" role="group" aria-label={m.products_more_details()}>
				{#if !showTags}
					<mm-button variant="ghost" size="sm" icon="add" onclick={() => (showTags = true)}
						>{m.common_tags()}</mm-button
					>
				{/if}
				{#if !showMetadata}
					<mm-button variant="ghost" size="sm" icon="add" onclick={() => (showMetadata = true)}
						>{m.products_tab_metadata()}</mm-button
					>
				{/if}
			</div>
		{/if}

		{#if error}
			<mm-callout tone="danger" heading={m.products_create_failed()}>{error}</mm-callout>
		{/if}

		<div class="mm-cluster mm-cluster--between">
			<mm-checkbox
				label={m.products_create_another()}
				checked={createAnother}
				onmm-change={(e: CustomEvent<{ checked: boolean }>) => (createAnother = e.detail.checked)}
			></mm-checkbox>
			<div class="mm-cluster" style="--mm-gap: var(--space-2)">
				<mm-button href={resolve('/products')}>{m.common_cancel()}</mm-button>
				<mm-button variant="primary" type="submit" loading={saving}>{m.products_create()}</mm-button
				>
			</div>
		</div>
	</form>
</div>

<mm-confirm-dialog
	open={guard.showConfirm}
	heading={m.common_discard_prompt()}
	body={m.products_unsaved_changes_confirm()}
	confirm-label={m.common_discard()}
	cancel-label={m.common_keep_editing()}
	onmm-confirm={guard.confirmDiscard}
	onmm-close={(e: CustomEvent<{ returnValue?: string }>) => {
		if (e.detail?.returnValue !== 'confirm') guard.cancelDiscard();
	}}
></mm-confirm-dialog>

<style>
	.create {
		--mm-gap: var(--space-6);
		max-width: 44rem;
	}
	.more {
		--mm-gap: var(--space-1);
		padding-block: var(--space-2);
		border-block: 1px solid var(--mm-rule);
	}
</style>
