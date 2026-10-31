<script lang="ts">
	import '@marmotdata/elements/drawer.js';
	import '@marmotdata/elements/code-sample.js';
	import '@marmotdata/elements/number-field.js';
	import type { MmValueDetail } from '@marmotdata/elements/classes/form-field.js';
	import { createAPIKey } from '$lib/serviceaccounts/api';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/utils';
	import type { ServiceAccount, ServiceAccountAPIKey } from '$lib/serviceaccounts/types';

	// Creates a key, then shows it once with an example request. Once the key exists, any way of closing counts as done so the page picks up the new key.
	// With rotated, the drawer opens straight on that key's one-time display, since a rotation has already issued it
	let {
		sa,
		rotated = null,
		onClose,
		onDone
	}: {
		sa: ServiceAccount;
		rotated?: ServiceAccountAPIKey | null;
		onClose: () => void;
		onDone: () => void;
	} = $props();

	type Expiry = '30' | '90' | '365' | 'never' | 'custom';

	let name = $state('');
	let expires = $state<Expiry>('90');
	let customDays = $state('30');
	let saving = $state(false);
	let error = $state('');
	let nameError = $state('');
	// svelte-ignore state_referenced_locally
	let created = $state<ServiceAccountAPIKey | null>(rotated);

	let days = $derived(
		expires === 'never'
			? 0
			: expires === 'custom'
				? Math.max(1, parseInt(customDays, 10) || 1)
				: parseInt(expires, 10)
	);

	// The sample request is a shell command, so it stays out of the message catalog
	let example = $derived(
		`curl ${typeof location === 'undefined' ? '' : location.origin}/api/v1/assets/search?q=orders \\\n  -H "X-API-Key: $MARMOT_API_KEY"`
	);

	function expiryDate(day: string): string {
		return formatDate(new Date(Date.now() + parseInt(day, 10) * 86_400_000).toISOString());
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		const n = name.trim();
		nameError = !n
			? m.serviceaccounts_error_name_required()
			: n.length < 2
				? m.serviceaccounts_error_name_min()
				: '';
		if (nameError) return;
		saving = true;
		error = '';
		try {
			created = await createAPIKey(sa.id, {
				name: n,
				expires_in_days: days > 0 ? days : undefined
			});
		} catch (err) {
			error = err instanceof Error ? err.message : m.apikeys_create_error();
		} finally {
			saving = false;
		}
	}
</script>

<mm-drawer
	open
	heading={created
		? rotated
			? m.apikeys_rotated_heading()
			: m.serviceaccounts_key_created_banner()
		: m.serviceaccounts_create_key_for({ name: sa.name })}
	description={!created
		? ''
		: created.expires_at
			? m.serviceaccounts_key_created_expiring({
					key: created.name,
					account: sa.name,
					date: formatDate(created.expires_at)
				})
			: m.serviceaccounts_key_created_for({ key: created.name, account: sa.name })}
	onmm-close={() => (created ? onDone() : onClose())}
>
	{#if created}
		<div class="mm-stack" style="--mm-gap: var(--space-5)">
			<mm-callout tone="warning" heading={m.serviceaccounts_key_copy_now()}>
				{m.serviceaccounts_key_copy_now_body()}
			</mm-callout>
			<mm-code-sample
				language="shell"
				label={m.serviceaccounts_new_key_label()}
				code={created.key ?? ''}
			></mm-code-sample>
			<mm-section heading={m.serviceaccounts_key_use_it()}>
				<mm-code-sample
					wrap
					language="shell"
					label={m.serviceaccounts_example_request()}
					code={example}
				></mm-code-sample>
			</mm-section>
		</div>
		<mm-button slot="footer" variant="primary" data-close="done">
			{m.serviceaccounts_key_copied_button()}
		</mm-button>
	{:else}
		<form
			id="new-key"
			class="mm-stack"
			style="--mm-gap: var(--space-6)"
			novalidate
			onsubmit={handleSubmit}
		>
			{#if error}
				<mm-callout tone="danger" heading={m.serviceaccounts_key_create_failed()}
					>{error}</mm-callout
				>
			{/if}
			<mm-text-field
				label={m.common_name()}
				name="name"
				required
				value={name}
				error={nameError}
				help={m.serviceaccounts_key_name_help()}
				oninput={(e: Event) => (name = (e.currentTarget as unknown as { value: string }).value)}
			></mm-text-field>
			<mm-select
				label={m.serviceaccounts_key_expires()}
				name="expires"
				value={expires}
				onmm-change={(e: CustomEvent<MmValueDetail>) => (expires = e.detail.value as Expiry)}
			>
				<mm-option
					value="30"
					label={m.serviceaccounts_key_in_days({ count: 30, date: expiryDate('30') })}
				></mm-option>
				<mm-option
					value="90"
					label={m.serviceaccounts_key_in_days({ count: 90, date: expiryDate('90') })}
				></mm-option>
				<mm-option value="365" label={m.serviceaccounts_key_in_a_year({ date: expiryDate('365') })}
				></mm-option>
				<mm-option value="custom" label={m.serviceaccounts_key_custom()}></mm-option>
				<mm-option value="never" label={m.common_never()}></mm-option>
			</mm-select>
			{#if expires === 'custom'}
				<mm-number-field
					label={m.serviceaccounts_key_days_label()}
					name="days"
					min={1}
					value={customDays}
					unit={m.serviceaccounts_key_days_unit()}
					onmm-input={(e: CustomEvent<MmValueDetail>) => (customDays = e.detail.value)}
				></mm-number-field>
			{/if}
			<mm-callout tone="info" heading={m.serviceaccounts_key_gets_roles({ name: sa.name })}
			></mm-callout>
		</form>
		<mm-button slot="footer" data-close="cancel">{m.common_cancel()}</mm-button>
		<mm-button
			slot="footer"
			variant="primary"
			type="submit"
			form="new-key"
			icon="key"
			loading={saving}
		>
			{m.serviceaccounts_detail_create_key_button()}
		</mm-button>
	{/if}
</mm-drawer>
