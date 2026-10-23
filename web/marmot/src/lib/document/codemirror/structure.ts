// Walks the Lezer syntax trees of JSON and YAML documents with one vocabulary, so a path such as bindings[2].members[0] can be found in either and the cursor can be placed in a path for completion.

import type { EditorState } from '@codemirror/state';
import { ensureSyntaxTree, syntaxTree } from '@codemirror/language';
import type { SyntaxNode, Tree } from '@lezer/common';
import type { PathSegment } from '../paths';

const MAPS = new Set(['Object', 'BlockMapping', 'FlowMapping']);
const SEQUENCES = new Set(['Array', 'BlockSequence', 'FlowSequence']);
const ENTRIES = new Set(['Property', 'Pair']);
const SCALARS = new Set(['String', 'Literal', 'QuotedLiteral']);
const PUNCTUATION = new Set([
	'{',
	'}',
	'[',
	']',
	',',
	':',
	'-',
	'?',
	'Comment',
	'LineComment',
	'BlockComment',
	'DirectiveEnd',
	'DocEnd',
	'Directive'
]);

function children(node: SyntaxNode): SyntaxNode[] {
	const out: SyntaxNode[] = [];
	for (let child = node.firstChild; child; child = child.nextSibling) out.push(child);
	return out;
}

function valueChildren(node: SyntaxNode): SyntaxNode[] {
	return children(node).filter((child) => !PUNCTUATION.has(child.name) && !child.type.isError);
}

// YAML anchors and tags wrap the value they annotate.
function unwrap(node: SyntaxNode | null): SyntaxNode | null {
	while (node && (node.name === 'Tagged' || node.name === 'Anchored')) {
		const inner = valueChildren(node).at(-1);
		if (!inner) break;
		node = inner;
	}
	return node;
}

/** Removes the quotes from a JSON or YAML scalar. */
export function unquote(text: string): string {
	const value = text.trim();
	if (value.startsWith('"')) {
		try {
			return JSON.parse(value.endsWith('"') && value.length > 1 ? value : `${value}"`);
		} catch {
			return value.slice(1, value.endsWith('"') ? -1 : undefined);
		}
	}
	if (value.startsWith("'")) {
		return value
			.slice(1, value.endsWith("'") && value.length > 1 ? -1 : undefined)
			.replace(/''/g, "'");
	}
	return value;
}

export function isScalar(node: SyntaxNode): boolean {
	return SCALARS.has(node.name);
}

function keyNode(entry: SyntaxNode): SyntaxNode | null {
	return entry.getChild('PropertyName') ?? entry.getChild('Key');
}

function keyName(state: EditorState, entry: SyntaxNode): string | null {
	const key = keyNode(entry);
	return key ? unquote(state.sliceDoc(key.from, key.to)) : null;
}

function entryValue(entry: SyntaxNode): SyntaxNode | null {
	const all = children(entry);
	const colon = all.findIndex((child) => child.name === ':');
	if (colon === -1) return null;
	return unwrap(
		all.slice(colon + 1).find((child) => !PUNCTUATION.has(child.name) && !child.type.isError) ??
			null
	);
}

function elements(sequence: SyntaxNode): SyntaxNode[] {
	if (sequence.name === 'Array') return valueChildren(sequence);
	return children(sequence).filter((child) => child.name === 'Item');
}

function elementValue(element: SyntaxNode): SyntaxNode {
	if (element.name !== 'Item') return element;
	return unwrap(valueChildren(element)[0] ?? null) ?? element;
}

function topValue(tree: Tree): SyntaxNode | null {
	let node: SyntaxNode | null = tree.topNode;
	if (node.name === 'Stream') node = node.getChild('Document');
	if (!node) return null;
	return unwrap(valueChildren(node)[0] ?? null);
}

function fullTree(state: EditorState): Tree {
	return ensureSyntaxTree(state, state.doc.length, 200) ?? syntaxTree(state);
}

export interface Range {
	from: number;
	to: number;
}

// A marker on a whole multi-line block would underline half the editor, so ranges are kept to their first line and never start on indentation.
function tidy(state: EditorState, range: Range): Range {
	let { from, to } = range;
	while (from < to && /\s/.test(state.sliceDoc(from, from + 1))) from++;
	const line = state.doc.lineAt(from);
	to = Math.min(Math.max(to, from), line.to);
	if (to === from) {
		if (to < line.to) to++;
		else if (from > line.from) from--;
	}
	return { from, to };
}

/**
 * The text range of the value at path, or of its key when preferKey is set. When part of the path is missing, the deepest part that exists is returned, so a problem with a field that is not there yet lands on the object that should hold it.
 */
export function locate(state: EditorState, path: PathSegment[], preferKey = false): Range | null {
	let node = topValue(fullTree(state));
	if (!node) return state.doc.length ? tidy(state, { from: 0, to: state.doc.length }) : null;
	let range: Range = { from: node.from, to: node.to };
	let keyRange: Range | null = null;

	for (let index = 0; index < path.length; index++) {
		const segment = path[index];
		const last = index === path.length - 1;
		if (typeof segment === 'string' && MAPS.has(node.name)) {
			const entry = children(node).find(
				(child) => ENTRIES.has(child.name) && keyName(state, child) === segment
			);
			if (!entry) return tidy(state, keyRange ?? range);
			const key = keyNode(entry);
			const value = entryValue(entry);
			keyRange = key ? { from: key.from, to: key.to } : { from: entry.from, to: entry.to };
			if (!value || (last && preferKey)) return tidy(state, keyRange);
			node = value;
			range = { from: value.from, to: value.to };
		} else if (typeof segment === 'number' && SEQUENCES.has(node.name)) {
			const element = elements(node)[segment];
			if (!element) return tidy(state, keyRange ?? range);
			node = elementValue(element);
			range = { from: node.from, to: node.to };
			keyRange = null;
		} else {
			return tidy(state, keyRange ?? range);
		}
	}
	// A container is better marked by the key that names it, or failing that by its first key, than by its first line.
	if ((MAPS.has(node.name) || SEQUENCES.has(node.name)) && keyRange) return tidy(state, keyRange);
	if (MAPS.has(node.name)) {
		const first = children(node).find((child) => ENTRIES.has(child.name));
		const key = first ? keyNode(first) : null;
		if (key) return tidy(state, { from: key.from, to: key.to });
	}
	return tidy(state, range);
}

export interface CursorContext {
	path: PathSegment[];
	kind: 'key' | 'value';
	/** The range of what has been typed, without any quotes, which a completion replaces. */
	from: number;
	to: number;
	prefix: string;
	/** The quote the typed text sits inside, if any. */
	quote: '"' | "'" | '';
	/** Whether the cursor follows a key's colon on the same line, so it is certainly a value. */
	afterColon: boolean;
}

function walk(
	state: EditorState,
	start: SyntaxNode,
	pos: number,
	from: number
): { path: PathSegment[]; kind: 'key' | 'value' } {
	const path: PathSegment[] = [];
	let kind: 'key' | 'value' | null = null;
	let previous: SyntaxNode | null = null;
	for (let node: SyntaxNode | null = start; node; previous = node, node = node.parent) {
		if (ENTRIES.has(node.name)) {
			const colon = node.getChild(':');
			if (!colon || pos <= colon.from) {
				kind ??= 'key';
			} else {
				kind ??= 'value';
				const name = keyName(state, node);
				if (name !== null) path.unshift(name);
			}
		} else if (MAPS.has(node.name)) {
			kind ??= 'key';
		} else if (SEQUENCES.has(node.name)) {
			kind ??= 'value';
			const anchor = previous ? previous.from : from;
			const before = elements(node).filter(
				(element) =>
					element.to <= anchor &&
					element !== previous &&
					!(element.from <= from && from <= element.to)
			);
			path.unshift(before.length);
		}
	}
	return { path, kind: kind ?? 'key' };
}

const TOKEN_BEFORE = /[^\s"',[\]{}]*$/;
const TOKEN_AFTER = /^[^\s"',[\]{}]*/;

/** Where the cursor sits in the document's structure, for completion. */
export function contextAt(state: EditorState, pos: number): CursorContext {
	const line = state.doc.lineAt(pos);
	const before = state.sliceDoc(line.from, pos);
	let token = TOKEN_BEFORE.exec(before)?.[0] ?? '';
	// A JSON colon with no space after it is not part of the value.
	while (token.startsWith(':')) token = token.slice(1);
	const from = pos - token.length;
	let rest = TOKEN_AFTER.exec(state.sliceDoc(pos, line.to))?.[0] ?? '';
	if (/:$/.test(rest)) rest = rest.replace(/:+$/, '');
	const to = pos + rest.length;
	const quoteChar = state.sliceDoc(from - 1, from);
	const quote = quoteChar === '"' || quoteChar === "'" ? quoteChar : '';
	const lead = state.sliceDoc(line.from, from - quote.length);
	const keyBefore = /(?:^|[\s{,])(["']?)([^"'\s:{},[\]-][^"':{},[\]]*?)\1\s*:\s*$/.exec(lead);
	const afterColon = keyBefore !== null;

	const tree = syntaxTree(state);
	let { path, kind } = walk(state, tree.resolveInner(pos, -1), pos, from);

	// After "key: " with nothing typed, the parser often has not attached the cursor to the pair yet, so walk again from the colon.
	if (kind === 'key' && keyBefore) {
		const name = keyBefore[2].trim();
		const colon = line.from + lead.lastIndexOf(':');
		const again = walk(state, tree.resolveInner(colon, -1), colon + 1, colon + 1);
		if (again.kind === 'value' && again.path.at(-1) === name) {
			path = again.path;
		} else {
			path = [...path, name];
		}
		kind = 'value';
	}

	return { path, kind, from, to, prefix: token, quote, afterColon };
}

/** The keys already present in the mapping at path, so completion does not offer them twice. */
export function keysAt(state: EditorState, path: PathSegment[]): Set<string> {
	const keys = new Set<string>();
	let node = topValue(fullTree(state));
	for (const segment of path) {
		if (!node) return keys;
		if (typeof segment === 'string' && MAPS.has(node.name)) {
			const entry = children(node).find(
				(child) => ENTRIES.has(child.name) && keyName(state, child) === segment
			);
			node = entry ? entryValue(entry) : null;
		} else if (typeof segment === 'number' && SEQUENCES.has(node.name)) {
			const element = elements(node)[segment];
			node = element ? elementValue(element) : null;
		} else {
			return keys;
		}
	}
	if (node && MAPS.has(node.name)) {
		for (const child of children(node)) {
			if (!ENTRIES.has(child.name) || !child.getChild(':')) continue;
			const name = keyName(state, child);
			if (name) keys.add(name);
		}
	}
	return keys;
}

/** Every scalar in the visible part of the document, for inline hints. */
export function eachScalar(
	state: EditorState,
	ranges: readonly Range[],
	visit: (node: { from: number; to: number; text: string }) => void
) {
	const tree = syntaxTree(state);
	for (const { from, to } of ranges) {
		tree.iterate({
			from,
			to,
			enter(node) {
				if (SCALARS.has(node.name)) {
					visit({
						from: node.from,
						to: node.to,
						text: unquote(state.sliceDoc(node.from, node.to))
					});
				}
			}
		});
	}
}
