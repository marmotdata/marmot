<script lang="ts">
	import MrnValue from '$components/shared/MrnValue.svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { formatRelativeTime } from '$lib/utils';

	// The Identity section of the Metadata tab: the asset's MRN, the MRNs it used to have and which writers describe it.
	let { assetId, mrn }: { assetId: string; mrn: string } = $props();

	interface WriterInfo {
		kind: string;
		pipeline?: string;
		source?: string;
		scheduled?: boolean;
		principal_type?: string;
		principal_id?: string;
		principal_name?: string;
		zone?: string;
	}

	interface Contribution {
		writer: string;
		writer_info?: WriterInfo;
		role: string;
		fields?: Record<string, unknown>;
		first_seen_at: string;
		last_seen_at: string;
	}

	interface Identity {
		mrn: string;
		aliases: string[];
		contributions: Contribution[];
	}

	let identity = $state<Identity | null>(null);
	let failed = $state(false);

	const roleLabels: Record<string, () => string> = {
		manual: m.asset_identity_role_manual,
		native: m.asset_identity_role_native,
		federated: m.asset_identity_role_federated,
		reference: m.asset_identity_role_reference
	};

	// Who a writer is, in words; the raw writer key stays available as a tooltip for anyone matching it to a pipeline or API key
	function writerLabel(c: Contribution): string {
		const w = c.writer_info;
		if (!w) return c.writer;
		const who =
			w.principal_name ||
			(w.principal_type === 'service_account'
				? m.asset_identity_writer_service_account()
				: m.asset_identity_writer_someone());
		switch (w.kind) {
			case 'pipeline': {
				const pipeline = w.pipeline ?? '';
				if (w.scheduled) return m.asset_identity_writer_pipeline_scheduled({ pipeline });
				if (w.principal_id && w.zone)
					return m.asset_identity_writer_pipeline_by_zone({ pipeline, who, zone: w.zone });
				if (w.principal_id) return m.asset_identity_writer_pipeline_by({ pipeline, who });
				return m.asset_identity_writer_pipeline({ pipeline });
			}
			case 'api':
				return w.principal_id
					? m.asset_identity_writer_api({ who })
					: m.asset_identity_writer_api_anonymous();
			case 'openlineage':
				return m.asset_identity_writer_openlineage();
			case 'ui':
				return m.asset_identity_writer_ui();
			case 'marmot':
				return m.asset_identity_writer_marmot();
			case 'legacy':
				return m.asset_identity_writer_legacy();
			default:
				return c.writer;
		}
	}

	// An MRN has no spaces, so offer line breaks after its separators; the browser then wraps at a segment boundary instead of inside a word
	function chunks(value: string): string[] {
		return value.split(/(?<=[/@.])/);
	}

	// Only the fields a writer actually set are worth naming
	function fieldNames(c: Contribution): string {
		return Object.entries(c.fields ?? {})
			.filter(([, v]) => v != null && v !== '' && !(Array.isArray(v) && v.length === 0))
			.map(([k]) => k)
			.join(', ');
	}

	$effect(() => {
		const id = assetId;
		let current = true;
		identity = null;
		failed = false;
		fetchApi(`/assets/identity/${encodeURIComponent(id)}`)
			.then(async (res) => {
				if (!res.ok) throw new Error(res.statusText);
				const data = await res.json();
				if (current)
					identity = {
						mrn: data.mrn ?? mrn,
						aliases: data.aliases ?? [],
						contributions: data.contributions ?? []
					};
			})
			.catch(() => {
				if (current) failed = true;
			});
		return () => {
			current = false;
		};
	});

	let shownMrn = $derived(identity?.mrn ?? mrn);
</script>

<mm-section heading={m.asset_identity_heading()} description={m.asset_identity_description()}>
	<div class="mm-stack" style="--mm-gap: var(--space-4)">
		<mm-description-list>
			<mm-description term={m.asset_identity_mrn()} mono>
				<MrnValue value={shownMrn} />
			</mm-description>
			{#if identity && identity.aliases.length > 0}
				<mm-description term={m.asset_identity_previous()} mono>
					{#each identity.aliases as alias (alias)}
						<div class="mrn-text">
							{#each chunks(alias) as part, i (i)}{part}<wbr />{/each}
						</div>
					{/each}
				</mm-description>
			{/if}
		</mm-description-list>

		{#if failed}
			<mm-text variant="dense" tone="secondary" block>{m.asset_identity_load_error()}</mm-text>
		{:else if identity && identity.contributions.length > 0}
			<ul class="writers" aria-label={m.asset_identity_writers()}>
				{#each identity.contributions as c (c.writer + c.role)}
					<li class="row">
						<div class="mm-stack" style="--mm-gap: 0; min-width: 0">
							<mm-text variant="dense" title={c.writer}>{writerLabel(c)}</mm-text>
							<mm-text variant="caption" tone="secondary"
								>{[
									fieldNames(c),
									m.asset_identity_last_seen({ when: formatRelativeTime(c.last_seen_at) })
								]
									.filter(Boolean)
									.join(' · ')}</mm-text
							>
						</div>
						<mm-tag class="mm-push">{roleLabels[c.role]?.() ?? c.role}</mm-tag>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</mm-section>

<style>
	/* Breaks come from the wbr after each separator; anywhere is only the fallback for a segment wider than the column */
	.mrn-text {
		min-width: 0;
		overflow-wrap: anywhere;
	}
	.writers {
		list-style: none;
		margin: 0;
		padding: 0;
		border-block-start: 1px solid var(--mm-rule);
	}
	.writers > li {
		padding-block: var(--space-3);
		border-block-end: 1px solid var(--mm-rule);
	}
	.row {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}
</style>
