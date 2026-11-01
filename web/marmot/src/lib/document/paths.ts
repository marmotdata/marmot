// Paths into a JSON or YAML document in the spelling the API reports problems with, such as bindings[2].members[0].

export type PathSegment = string | number;

/** Splits bindings[2].members[0] into ['bindings', 2, 'members', 0]. An empty path is the document itself. */
export function parsePath(path: string): PathSegment[] {
	const segments: PathSegment[] = [];
	const pattern = /([^.[\]]+)|\[(\d+)\]/g;
	let match: RegExpExecArray | null;
	while ((match = pattern.exec(path)) !== null) {
		if (match[2] !== undefined) segments.push(Number(match[2]));
		else segments.push(match[1]);
	}
	return segments;
}

/** The inverse of parsePath. */
export function formatPath(segments: PathSegment[]): string {
	let out = '';
	for (const segment of segments) {
		if (typeof segment === 'number') out += `[${segment}]`;
		else out += out ? `.${segment}` : segment;
	}
	return out;
}

/** Converts a JSON Pointer such as /bindings/0/role, as ajv reports it, into segments. */
export function pointerToPath(pointer: string): PathSegment[] {
	if (!pointer) return [];
	return pointer
		.split('/')
		.slice(1)
		.map((part) => part.replace(/~1/g, '/').replace(/~0/g, '~'))
		.map((part) => (/^\d+$/.test(part) ? Number(part) : part));
}

/** Whether a path matches a pattern in which * stands for any one segment, such as bindings.*.members.*. */
export function pathMatches(segments: PathSegment[], pattern: string): boolean {
	const parts = pattern.split('.');
	return (
		parts.length === segments.length &&
		parts.every((part, index) => part === '*' || part === String(segments[index]))
	);
}
