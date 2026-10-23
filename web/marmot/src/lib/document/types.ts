// Shared types for the document editor and the forms built on it.

import type { PathSegment } from './paths';

/** A problem the API reported against part of a document, as {path, message}. */
export interface DocumentProblem {
	path: string;
	message: string;
}

/** A problem shown in the editor and the list beside it, from the API or found while typing. */
export interface EditorDiagnostic {
	/** Where it is, as bindings[2].members[0]; empty for the whole document. */
	path: string;
	message: string;
	severity: 'error' | 'warning';
	/** The line it sits on, counted from 1, once the editor has placed it. */
	line?: number;
}

/** Where the cursor is when completions are asked for. */
export interface CompletionContext {
	path: PathSegment[];
	/** Whether a property name or a value is being typed. */
	kind: 'key' | 'value';
	/** What has been typed of it so far, without quotes. */
	prefix: string;
}

export interface CompletionOption {
	label: string;
	/** Shown beside the label, such as a display name. */
	detail?: string;
	/** Shown in the documentation panel. */
	info?: string;
	/** Grouping hint for the icon: a role, member, permission and so on. */
	type?: string;
}

/** Supplies extra completions, such as role names or members, for a position in the document. */
export type CompletionProvider = (
	context: CompletionContext
) => CompletionOption[] | Promise<CompletionOption[]>;
