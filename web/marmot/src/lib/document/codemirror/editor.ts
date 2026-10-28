// The CodeMirror half of DocumentEditor. Everything here is loaded with a dynamic import the first time an editor opens, so CodeMirror never sits in the bundle of a page that does not show one.

import {
	Compartment,
	EditorSelection,
	EditorState,
	RangeSetBuilder,
	StateEffect,
	StateField,
	type Extension
} from '@codemirror/state';
import {
	Decoration,
	EditorView,
	ViewPlugin,
	WidgetType,
	drawSelection,
	highlightActiveLine,
	highlightActiveLineGutter,
	keymap,
	lineNumbers,
	placeholder as placeholderText,
	type DecorationSet,
	type ViewUpdate
} from '@codemirror/view';
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
import {
	HighlightStyle,
	bracketMatching,
	foldGutter,
	foldKeymap,
	indentOnInput,
	indentUnit,
	syntaxHighlighting
} from '@codemirror/language';
import { linter, lintGutter, lintKeymap, type Diagnostic } from '@codemirror/lint';
import {
	autocompletion,
	closeBrackets,
	closeBracketsKeymap,
	completionKeymap,
	type Completion,
	type CompletionContext as CodeMirrorCompletionContext,
	type CompletionResult
} from '@codemirror/autocomplete';
import { json } from '@codemirror/lang-json';
import { yaml } from '@codemirror/lang-yaml';
import { tags } from '@lezer/highlight';
import { load, CORE_SCHEMA } from 'js-yaml';
import type { DocumentMode } from '../format';
import { formatPath, parsePath, type PathSegment } from '../paths';
import type { CompletionOption, CompletionProvider, EditorDiagnostic } from '../types';
import { contextAt, eachScalar, keysAt, locate, type Range } from './structure';
import {
	expectsObject,
	keyOptions,
	schemasAt,
	validateAgainst,
	valueOptions,
	type Schema
} from './schema';

export interface EditorOptions {
	parent: HTMLElement;
	text: string;
	mode: DocumentMode;
	schema?: Schema | null;
	readOnly?: boolean;
	dark?: boolean;
	label: string;
	describedBy?: string;
	placeholder?: string;
	completions?: CompletionProvider;
	hints?: Record<string, string>;
	diagnostics?: EditorDiagnostic[];
	onChange?: (text: string) => void;
	onSave?: () => void;
	onLint?: (diagnostics: EditorDiagnostic[]) => void;
}

export interface EditorController {
	setText(text: string): void;
	setMode(mode: DocumentMode): void;
	setSchema(schema: Schema | null): void;
	setReadOnly(readOnly: boolean): void;
	setDark(dark: boolean): void;
	setDiagnostics(diagnostics: EditorDiagnostic[]): void;
	setHints(hints: Record<string, string>): void;
	setCompletions(provider: CompletionProvider | undefined): void;
	setLabel(label: string, describedBy?: string): void;
	reveal(diagnostic: Pick<EditorDiagnostic, 'path' | 'line'>): void;
	focus(): void;
	destroy(): void;
}

// Colours from the app's palette (tailwind.config.js) so the editor reads as part of the page in both themes.
const LIGHT = {
	background: '#ffffff',
	text: '#1A1A1A',
	gutter: '#F9F9F9',
	gutterText: '#B0B0B0',
	border: '#ECECEC',
	activeLine: 'rgba(26, 26, 26, 0.03)',
	selection: '#ffd7c8',
	cursor: '#d25a30',
	key: '#c74624',
	string: '#35593b',
	number: '#4a656f',
	punctuation: '#8F8F8F',
	hint: '#8F8F8F',
	tooltip: '#ffffff'
};

const DARK = {
	background: '#1A1A1A',
	text: '#ECECEC',
	gutter: '#2E2E2E',
	gutterText: '#696969',
	border: '#4D4D4D',
	activeLine: 'rgba(255, 255, 255, 0.04)',
	selection: 'rgba(245, 112, 74, 0.32)',
	cursor: '#ffa382',
	key: '#ffa382',
	string: '#b9cdb9',
	number: '#a2b4c6',
	punctuation: '#8F8F8F',
	hint: '#8F8F8F',
	tooltip: '#2E2E2E'
};

function themeFor(dark: boolean): Extension {
	const c = dark ? DARK : LIGHT;
	const theme = EditorView.theme(
		{
			'&': {
				backgroundColor: c.background,
				color: c.text,
				fontSize: '13px',
				height: '100%'
			},
			'&.cm-focused': { outline: 'none' },
			'.cm-scroller': {
				fontFamily:
					'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace',
				lineHeight: '1.6'
			},
			'.cm-content': { caretColor: c.cursor, padding: '8px 0' },
			'.cm-cursor, .cm-dropCursor': { borderLeftColor: c.cursor },
			'&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground, ::selection':
				{ backgroundColor: c.selection },
			'.cm-gutters': {
				backgroundColor: c.gutter,
				color: c.gutterText,
				borderRight: `1px solid ${c.border}`
			},
			'.cm-activeLine': { backgroundColor: c.activeLine },
			'.cm-activeLineGutter': { backgroundColor: c.activeLine },
			'.cm-tooltip': {
				backgroundColor: c.tooltip,
				color: c.text,
				border: `1px solid ${c.border}`,
				borderRadius: '8px',
				overflow: 'hidden'
			},
			'.cm-tooltip-autocomplete > ul > li[aria-selected]': {
				backgroundColor: dark ? '#4D4D4D' : '#ECECEC',
				color: c.text
			},
			'.cm-completionDetail': { color: c.hint, fontStyle: 'normal', marginLeft: '0.75em' },
			'.cm-completionInfo': { padding: '6px 10px', maxWidth: '22rem' },
			'.cm-diagnostic': { padding: '4px 8px' },
			'.cm-value-hint': {
				color: c.hint,
				fontFamily: 'ui-sans-serif, system-ui, sans-serif',
				fontSize: '12px',
				marginLeft: '0.75em',
				userSelect: 'none'
			},
			'.cm-placeholder': { color: c.gutterText }
		},
		{ dark }
	);
	const highlight = HighlightStyle.define([
		{ tag: [tags.propertyName, tags.definition(tags.propertyName)], color: c.key },
		{ tag: [tags.string, tags.special(tags.string)], color: c.string },
		{ tag: [tags.number, tags.bool, tags.null, tags.atom], color: c.number },
		{
			tag: [tags.punctuation, tags.separator, tags.bracket, tags.squareBracket, tags.brace],
			color: c.punctuation
		},
		{ tag: [tags.comment, tags.lineComment], color: c.gutterText, fontStyle: 'italic' },
		{ tag: [tags.meta, tags.keyword, tags.labelName], color: c.number }
	]);
	return [theme, syntaxHighlighting(highlight)];
}

const setServerDiagnostics = StateEffect.define<EditorDiagnostic[]>();
const refresh = StateEffect.define<null>();

const serverDiagnostics = StateField.define<EditorDiagnostic[]>({
	create: () => [],
	update(value, transaction) {
		for (const effect of transaction.effects) {
			if (effect.is(setServerDiagnostics)) value = effect.value;
		}
		return value;
	}
});

class HintWidget extends WidgetType {
	constructor(readonly text: string) {
		super();
	}
	eq(other: HintWidget) {
		return other.text === this.text;
	}
	toDOM() {
		const span = document.createElement('span');
		span.className = 'cm-value-hint';
		span.textContent = this.text;
		return span;
	}
	ignoreEvent() {
		return true;
	}
}

function hintPlugin(hints: Record<string, string>): Extension {
	if (Object.keys(hints).length === 0) return [];
	const build = (view: EditorView): DecorationSet => {
		const builder = new RangeSetBuilder<Decoration>();
		eachScalar(view.state, view.visibleRanges, (node) => {
			const hint = hints[node.text];
			if (hint) {
				builder.add(node.to, node.to, Decoration.widget({ widget: new HintWidget(hint), side: 1 }));
			}
		});
		return builder.finish();
	};
	return ViewPlugin.fromClass(
		class {
			decorations: DecorationSet;
			constructor(view: EditorView) {
				this.decorations = build(view);
			}
			update(update: ViewUpdate) {
				if (update.docChanged || update.viewportChanged || update.transactions.length) {
					this.decorations = build(update.view);
				}
			}
		},
		{ decorations: (plugin) => plugin.decorations }
	);
}

function parseText(
	text: string,
	mode: DocumentMode
): { ok: true; value: unknown } | { ok: false; message: string; offset: number } {
	if (mode === 'json') {
		try {
			return { ok: true, value: JSON.parse(text) };
		} catch (error) {
			const message = error instanceof Error ? error.message : String(error);
			const position = /position (\d+)/.exec(message);
			return { ok: false, message, offset: position ? Number(position[1]) : text.length };
		}
	}
	try {
		return { ok: true, value: load(text, { schema: CORE_SCHEMA }) };
	} catch (error) {
		const mark = (error as { mark?: { position: number } }).mark;
		const reason = (error as { reason?: string }).reason;
		return {
			ok: false,
			message: reason ?? (error instanceof Error ? error.message : String(error)),
			offset: mark?.position ?? text.length
		};
	}
}

// Plain YAML scalars cannot start with an indicator or contain ": " or " #"; anything else is written bare.
function yamlScalar(value: string): string {
	if (
		value === '' ||
		/^[\s\-?:,[\]{}#&*!|>'"%@`]/.test(value) ||
		/: |\s#|\s$/.test(value) ||
		/^(true|false|null|~|yes|no|on|off|[-+]?[\d.]+)$/i.test(value)
	) {
		return JSON.stringify(value);
	}
	return value;
}

export function createEditor(options: EditorOptions): EditorController {
	let mode = options.mode;
	let schema: Schema | null = options.schema ?? null;
	let provider = options.completions;
	let applyingExternal = false;

	const language = new Compartment();
	const theme = new Compartment();
	const readOnly = new Compartment();
	const hints = new Compartment();
	const attributes = new Compartment();

	const languageFor = (next: DocumentMode) => (next === 'json' ? json() : yaml());
	const readOnlyFor = (value: boolean) => [
		EditorState.readOnly.of(value),
		EditorView.contentAttributes.of({ 'aria-readonly': String(value) })
	];
	const attributesFor = (label: string, describedBy?: string) =>
		EditorView.contentAttributes.of({
			'aria-label': label,
			...(describedBy ? { 'aria-describedby': describedBy } : {})
		});

	function lint(view: EditorView): Diagnostic[] {
		const state = view.state;
		const text = state.doc.toString();
		const found: { diagnostic: Diagnostic; path: string }[] = [];
		if (text.trim()) {
			const parsed = parseText(text, mode);
			if (!parsed.ok) {
				const from = Math.min(parsed.offset, state.doc.length);
				const line = state.doc.lineAt(from);
				found.push({
					diagnostic: {
						from,
						to: Math.max(from, Math.min(from + 1, line.to)),
						severity: 'error',
						message: parsed.message,
						source: 'syntax'
					},
					path: ''
				});
			} else {
				if (schema) {
					for (const problem of validateAgainst(schema, parsed.value)) {
						const range = locate(state, problem.path, problem.key);
						if (!range) continue;
						found.push({
							diagnostic: {
								...range,
								severity: 'error',
								message: problem.message,
								source: 'schema'
							},
							path: formatPath(problem.path)
						});
					}
				}
				for (const diagnostic of state.field(serverDiagnostics)) {
					const range = locate(state, parsePath(diagnostic.path)) ?? { from: 0, to: 0 };
					found.push({
						diagnostic: {
							...range,
							severity: diagnostic.severity,
							message: diagnostic.message,
							source: 'server'
						},
						path: diagnostic.path
					});
				}
			}
		}
		found.sort((a, b) => a.diagnostic.from - b.diagnostic.from);
		options.onLint?.(
			found.map(({ diagnostic, path }) => ({
				path,
				message: diagnostic.message,
				severity: diagnostic.severity === 'warning' ? 'warning' : 'error',
				line: state.doc.lineAt(diagnostic.from).number
			}))
		);
		return found.map(({ diagnostic }) => diagnostic);
	}

	function render(option: CompletionOption, kind: 'key' | 'value', quote: string): string {
		if (kind === 'key') {
			if (mode === 'json') return quote ? option.label : `"${option.label}": `;
			return `${option.label}: `;
		}
		if (quote) return option.label;
		return mode === 'json' ? JSON.stringify(option.label) : yamlScalar(option.label);
	}

	async function complete(context: CodeMirrorCompletionContext): Promise<CompletionResult | null> {
		const cursor = contextAt(context.state, context.pos);
		if (!context.explicit && !cursor.prefix && !cursor.quote) return null;
		let kind = cursor.kind;
		if (kind === 'value' && !cursor.afterColon && schema) {
			if (expectsObject(schemaAtPath(cursor.path))) kind = 'key';
		}
		const found: CompletionOption[] = [];
		if (schema) {
			if (kind === 'key') {
				const present = keysAt(context.state, cursor.path);
				for (const option of keyOptions(schema, cursor.path)) {
					if (present.has(option.name) && option.name !== cursor.prefix) continue;
					found.push({ label: option.name, info: option.description, type: 'property' });
				}
			} else {
				for (const option of valueOptions(schema, cursor.path)) {
					found.push({ label: option.value, info: option.description, type: 'constant' });
				}
			}
		}
		if (provider) {
			try {
				found.push(...(await provider({ path: cursor.path, kind, prefix: cursor.prefix })));
			} catch {
				// A failed lookup leaves the schema's own suggestions.
			}
		}
		if (found.length === 0) return null;
		const seen = new Set<string>();
		const completions: Completion[] = [];
		for (const option of found) {
			if (seen.has(option.label)) continue;
			seen.add(option.label);
			completions.push({
				label: option.label,
				detail: option.detail,
				info: option.info,
				type: option.type,
				apply: (view, _completion, from, to) => {
					let insert = render(option, kind, cursor.quote);
					let end = to;
					// Inside quotes, step over the closing quote the editor added, and add one if there is none.
					if (cursor.quote) {
						if (view.state.sliceDoc(end, end + 1) === cursor.quote) end++;
						insert += cursor.quote;
						if (kind === 'key' && mode === 'json') insert += ': ';
					}
					view.dispatch({
						changes: { from, to: end, insert },
						selection: EditorSelection.cursor(from + insert.length),
						userEvent: 'input.complete'
					});
				}
			});
		}
		// No validFor: suggestions from the API depend on what has been typed, so each keystroke asks again.
		return { from: cursor.from, to: cursor.to, options: completions };
	}

	function schemaAtPath(path: PathSegment[]) {
		return schema ? schemaAtCache(schema, path) : [];
	}

	const view = new EditorView({
		parent: options.parent,
		state: EditorState.create({
			doc: options.text,
			extensions: [
				lineNumbers(),
				highlightActiveLineGutter(),
				foldGutter(),
				lintGutter(),
				history(),
				drawSelection(),
				indentOnInput(),
				indentUnit.of('  '),
				EditorState.tabSize.of(2),
				bracketMatching(),
				closeBrackets(),
				highlightActiveLine(),
				autocompletion({ override: [complete], icons: false }),
				serverDiagnostics,
				linter(lint, {
					delay: 300,
					needsRefresh: (update) =>
						update.transactions.some((transaction) =>
							transaction.effects.some(
								(effect) => effect.is(setServerDiagnostics) || effect.is(refresh)
							)
						)
				}),
				keymap.of([
					{
						key: 'Mod-s',
						preventDefault: true,
						run: () => {
							options.onSave?.();
							return true;
						}
					},
					...closeBracketsKeymap,
					...defaultKeymap,
					...historyKeymap,
					...foldKeymap,
					...completionKeymap,
					...lintKeymap,
					indentWithTab
				]),
				options.placeholder ? placeholderText(options.placeholder) : [],
				EditorView.updateListener.of((update) => {
					if (update.docChanged && !applyingExternal)
						options.onChange?.(update.state.doc.toString());
				}),
				language.of(languageFor(mode)),
				theme.of(themeFor(!!options.dark)),
				readOnly.of(readOnlyFor(!!options.readOnly)),
				hints.of(hintPlugin(options.hints ?? {})),
				attributes.of(attributesFor(options.label, options.describedBy))
			]
		})
	});

	if (options.diagnostics?.length) {
		view.dispatch({ effects: setServerDiagnostics.of(options.diagnostics) });
	}

	function rangeFor(diagnostic: Pick<EditorDiagnostic, 'path' | 'line'>): Range | null {
		if (diagnostic.path) return locate(view.state, parsePath(diagnostic.path));
		if (diagnostic.line && diagnostic.line <= view.state.doc.lines) {
			const line = view.state.doc.line(diagnostic.line);
			return { from: line.from, to: line.from };
		}
		return null;
	}

	return {
		setText(text) {
			if (text === view.state.doc.toString()) return;
			applyingExternal = true;
			view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: text } });
			applyingExternal = false;
		},
		setMode(next) {
			if (next === mode) return;
			mode = next;
			view.dispatch({ effects: [language.reconfigure(languageFor(next)), refresh.of(null)] });
		},
		setSchema(next) {
			schema = next;
			view.dispatch({ effects: refresh.of(null) });
		},
		setReadOnly(value) {
			view.dispatch({ effects: readOnly.reconfigure(readOnlyFor(value)) });
		},
		setDark(dark) {
			view.dispatch({ effects: theme.reconfigure(themeFor(dark)) });
		},
		setDiagnostics(diagnostics) {
			view.dispatch({ effects: setServerDiagnostics.of(diagnostics) });
		},
		setHints(next) {
			view.dispatch({ effects: hints.reconfigure(hintPlugin(next)) });
		},
		setCompletions(next) {
			provider = next;
		},
		setLabel(label, describedBy) {
			view.dispatch({ effects: attributes.reconfigure(attributesFor(label, describedBy)) });
		},
		reveal(diagnostic) {
			const range = rangeFor(diagnostic);
			if (!range) return;
			view.dispatch({
				selection: EditorSelection.range(range.from, range.to),
				effects: EditorView.scrollIntoView(range.from, { y: 'center' })
			});
			view.focus();
		},
		focus() {
			view.focus();
		},
		destroy() {
			view.destroy();
		}
	};
}

// schemasAt walks references on every call; completions ask for the same few paths repeatedly while typing.
const schemaCache = new WeakMap<Schema, Map<string, Schema[]>>();

function schemaAtCache(root: Schema, path: PathSegment[]): Schema[] {
	let byPath = schemaCache.get(root);
	if (!byPath) {
		byPath = new Map();
		schemaCache.set(root, byPath);
	}
	const key = formatPath(path);
	let found = byPath.get(key);
	if (!found) {
		found = schemasAt(root, path);
		byPath.set(key, found);
	}
	return found;
}
