<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { m } from '$lib/paraglide/messages';
	import DocumentEditor from '$components/ui/DocumentEditor.svelte';
	import DocumentProblems from '$components/ui/DocumentProblems.svelte';
	import { parseDocument, stringifyDocument, type DocumentMode } from '$lib/document/format';
	import { checkDocument, omitProperties } from '$lib/document/check';
	import { pathMatches } from '$lib/document/paths';
	import type { CompletionProvider, DocumentProblem, EditorDiagnostic } from '$lib/document/types';
	import type { Schema } from '$lib/document/codemirror/schema';
	import { getRoleSchema, validateRole } from '$lib/roles/api';
	import { permissionKey, readRoleDocument, type RoleDocument } from '$lib/roles/document';
	import type { Permission } from '$lib/roles/types';

	// A role as a JSON or YAML document, {name, description, permission_keys}, checked against the API as it is typed. The parent owns saving and reads the document back with exportDocument.
	let {
		mode,
		initial,
		permissions,
		roleId,
		readonly = false,
		height = '20rem',
		onSubmit
	}: {
		mode: DocumentMode;
		initial: RoleDocument;
		permissions: Permission[];
		/** The role being edited, so its own name does not count as taken. */
		roleId?: string;
		readonly?: boolean;
		height?: string;
		onSubmit?: () => void;
	} = $props();

	let text = $state('');
	let ready = $state(false);
	let schema = $state<Schema | null>(null);
	let serverProblems = $state<EditorDiagnostic[]>([]);
	let shown = $state<EditorDiagnostic[]>([]);
	let checking = $state(false);
	let editor = $state<DocumentEditor>();

	onMount(async () => {
		const [loaded, rendered] = await Promise.all([
			getRoleSchema(),
			stringifyDocument(
				untrack(() => initial),
				untrack(() => mode)
			)
		]);
		// Saving sends keys, so permission_ids is left out of what the editor offers and accepts.
		schema = loaded ? omitProperties(loaded, ['permission_ids']) : null;
		text = rendered;
		ready = true;
	});

	let sequence = 0;

	$effect(() => {
		const current = text;
		if (!ready || readonly) return;
		const timer = setTimeout(() => validate(current), 500);
		return () => clearTimeout(timer);
	});

	async function validate(current: string) {
		const parsed = await parseDocument(current, mode);
		const id = ++sequence;
		if (!parsed.ok) {
			serverProblems = [];
			return;
		}
		checking = true;
		try {
			const result = await validateRole(parsed.value, roleId);
			if (id === sequence) {
				serverProblems = result.problems.map((problem) => ({ ...problem, severity: 'error' }));
			}
		} catch {
			// The live check is advisory; saving still reports what the API refuses.
			if (id === sequence) serverProblems = [];
		} finally {
			if (id === sequence) checking = false;
		}
	}

	/** The document as typed, or why it cannot be used yet. */
	export async function exportDocument(): Promise<
		{ ok: true; doc: RoleDocument } | { ok: false; message: string }
	> {
		const result = await checkDocument(text, mode, schema);
		if (!result.ok) return result;
		return { ok: true, doc: readRoleDocument(result.value) };
	}

	/** Marks problems the API reported when the document was saved. */
	export function showProblems(problems: DocumentProblem[]) {
		sequence++;
		checking = false;
		serverProblems = problems.map((problem) => ({ ...problem, severity: 'error' }));
	}

	const completions: CompletionProvider = ({ path, kind, prefix }) => {
		if (kind !== 'value' || !pathMatches(path, 'permission_keys.*')) return [];
		const typed = prefix.toLowerCase();
		return permissions
			.filter((permission) => permissionKey(permission).toLowerCase().includes(typed))
			.map((permission) => ({
				label: permissionKey(permission),
				detail: permission.description,
				type: 'permission'
			}));
	};
</script>

{#if !ready}
	<mm-skeleton loading shape="rect" style="--mm-skeleton-height: 16rem"></mm-skeleton>
{:else}
	<div class="mm-stack" style="--mm-gap: var(--space-3)">
		<DocumentEditor
			bind:this={editor}
			bind:value={text}
			{mode}
			{schema}
			{readonly}
			{height}
			{completions}
			diagnostics={serverProblems}
			label={readonly ? m.roles_document_label_readonly() : m.roles_document_label()}
			onSave={() => onSubmit?.()}
			onLint={(found) => (shown = found)}
		/>
		{#if !readonly}
			<DocumentProblems
				diagnostics={shown}
				{checking}
				onSelect={(diagnostic) => editor?.reveal(diagnostic)}
			/>
		{/if}
	</div>
{/if}
