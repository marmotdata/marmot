<script lang="ts">
	import { onMount } from 'svelte';
	import { m } from '$lib/paraglide/messages';
	import type { DocumentMode } from '$lib/document/format';
	import type { CompletionProvider, EditorDiagnostic } from '$lib/document/types';
	import type { EditorController } from '$lib/document/codemirror/editor';
	import type { Schema } from '$lib/document/codemirror/schema';
	import IconifyIcon from '@iconify/svelte';
	import Alert from './Alert.svelte';

	// A JSON or YAML code editor on CodeMirror, which is fetched the first time an editor mounts so no other page pays for it. The schema drives local checks and completion, completions adds suggestions such as role names, diagnostics are the API's {path, message} problems placed on the text they name, and hints show a display name beside any value that has one.
	let {
		value = $bindable(''),
		mode = 'json',
		schema = null,
		completions,
		diagnostics = [],
		hints = {},
		readonly = false,
		label,
		describedBy,
		placeholder,
		height = '22rem',
		onSave,
		onLint,
		copyable = true
	}: {
		value?: string;
		mode?: DocumentMode;
		schema?: Schema | null;
		completions?: CompletionProvider;
		diagnostics?: EditorDiagnostic[];
		hints?: Record<string, string>;
		readonly?: boolean;
		label: string;
		describedBy?: string;
		placeholder?: string;
		height?: string;
		onSave?: () => void;
		onLint?: (diagnostics: EditorDiagnostic[]) => void;
		/** Shows a Copy button over the editor. Callers with a toolbar of their own that already copies turn it off. */
		copyable?: boolean;
	} = $props();

	let copied = $state(false);
	let copiedTimer: ReturnType<typeof setTimeout> | undefined;

	async function copy() {
		try {
			await navigator.clipboard.writeText(value);
			copied = true;
			clearTimeout(copiedTimer);
			copiedTimer = setTimeout(() => (copied = false), 1500);
		} catch {
			// Clipboard access can be refused, for example outside a secure context; the text is still selectable
		}
	}

	const hintId = $props.id();

	let host = $state<HTMLDivElement>();
	let controller = $state<EditorController | null>(null);
	let failed = $state(false);

	function isDark(): boolean {
		return document.documentElement.classList.contains('dark');
	}

	/** Moves the cursor to a problem and selects the text it is about. */
	export function reveal(diagnostic: Pick<EditorDiagnostic, 'path' | 'line'>) {
		controller?.reveal(diagnostic);
	}

	export function focus() {
		controller?.focus();
	}

	onMount(() => {
		let destroyed = false;
		let created: EditorController | null = null;
		const observer = new MutationObserver(() => created?.setDark(isDark()));
		observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
		// Escape belongs to the editor (closing suggestions, or the first half of Escape then Tab), so it must not also close the blade or modal around it.
		const keepEscape = (event: KeyboardEvent) => {
			if (event.key === 'Escape') event.stopPropagation();
		};
		host?.addEventListener('keydown', keepEscape);

		import('$lib/document/codemirror/editor')
			.then(({ createEditor }) => {
				if (destroyed || !host) return;
				created = createEditor({
					parent: host,
					text: value,
					mode,
					schema,
					readOnly: readonly,
					dark: isDark(),
					label,
					describedBy: describedBy ? `${describedBy} ${hintId}` : hintId,
					placeholder,
					completions,
					hints,
					diagnostics,
					onChange: (text) => (value = text),
					onSave: () => onSave?.(),
					onLint: (found) => onLint?.(found)
				});
				controller = created;
			})
			.catch(() => {
				failed = true;
			});

		return () => {
			destroyed = true;
			observer.disconnect();
			host?.removeEventListener('keydown', keepEscape);
			created?.destroy();
		};
	});

	$effect(() => {
		controller?.setText(value);
	});
	$effect(() => {
		controller?.setMode(mode);
	});
	$effect(() => {
		controller?.setSchema(schema);
	});
	$effect(() => {
		controller?.setReadOnly(readonly);
	});
	$effect(() => {
		controller?.setDiagnostics(diagnostics);
	});
	$effect(() => {
		controller?.setHints(hints);
	});
	$effect(() => {
		controller?.setCompletions(completions);
	});
	$effect(() => {
		controller?.setLabel(label, describedBy ? `${describedBy} ${hintId}` : hintId);
	});
</script>

{#if failed}
	<Alert tone="error">{m.ui_document_load_error()}</Alert>
{:else}
	<div
		class="relative overflow-hidden rounded-lg border border-gray-200 focus-within:border-earthy-terracotta-500 focus-within:ring-2 focus-within:ring-earthy-terracotta-500/30 dark:border-gray-700"
		style:height
	>
		<div bind:this={host} class="h-full"></div>
		{#if copyable && controller}
			<button
				type="button"
				onclick={copy}
				class="absolute right-2 top-2 z-10 inline-flex items-center gap-1 rounded-md border border-gray-200 bg-white/90 px-2 py-1 text-xs font-medium text-gray-600 shadow-sm backdrop-blur transition hover:bg-white hover:text-gray-900 dark:border-gray-700 dark:bg-gray-800/90 dark:text-gray-300 dark:hover:bg-gray-800 dark:hover:text-gray-100"
				aria-live="polite"
			>
				<IconifyIcon
					icon={copied ? 'material-symbols:check' : 'material-symbols:content-copy-outline'}
					class="h-3.5 w-3.5"
				/>
				{copied ? m.common_copied() : m.common_copy()}
			</button>
		{/if}
		{#if !controller}
			<div
				class="absolute inset-0 animate-pulse bg-gray-50 dark:bg-gray-900"
				role="status"
				aria-live="polite"
			>
				<span class="sr-only">{m.ui_document_loading()}</span>
			</div>
		{/if}
	</div>
	<p id={hintId} class="sr-only">
		{readonly ? m.ui_document_readonly_hint() : m.ui_document_keyboard_hint()}
	</p>
{/if}
