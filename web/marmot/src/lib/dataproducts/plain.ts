import { m } from '$lib/paraglide/messages';

type Joiner = 'AND' | 'OR' | 'NOT';

const CONDITION = /^@([\w.]+)\s*(!=|>=|<=|=|:|>|<|contains\b)\s*(.+)$/i;

function unquote(value: string): string {
	return value.trim().replace(/^"(.*)"$/, '$1');
}

function fieldLabel(path: string): string {
	const words = path
		.replace(/^metadata\./, '')
		.replace(/[._]+/g, ' ')
		.trim();
	return words.charAt(0).toUpperCase() + words.slice(1);
}

function generic(field: string, op: string, value: string): string {
	if (value.includes('*')) return m.products_plain_field_matches({ field, value });
	switch (op) {
		case '!=':
			return m.products_plain_field_is_not({ field, value });
		case 'contains':
			return m.products_plain_field_contains({ field, value });
		case '>':
			return m.products_plain_field_more_than({ field, value });
		case '>=':
			return m.products_plain_field_at_least({ field, value });
		case '<':
			return m.products_plain_field_less_than({ field, value });
		case '<=':
			return m.products_plain_field_at_most({ field, value });
		default:
			return m.products_plain_field_is({ field, value });
	}
}

function condition(segment: string): string {
	const match = CONDITION.exec(segment.trim());
	if (!match) return m.products_plain_text({ value: unquote(segment) });
	const path = match[1].toLowerCase();
	const op = match[2].toLowerCase();
	const value = unquote(match[3]);
	const plainEquals = (op === '=' || op === ':') && !value.includes('*');
	const negated = op === '!=' && !value.includes('*');
	switch (path) {
		case 'type':
			if (plainEquals) return m.products_plain_type({ value });
			if (negated) return m.products_plain_type_not({ value });
			return generic(m.products_plain_label_type(), op, value);
		case 'provider':
			if (plainEquals) return m.products_plain_provider({ value });
			if (negated) return m.products_plain_provider_not({ value });
			return generic(m.products_plain_label_provider(), op, value);
		case 'name':
			if (plainEquals) return m.products_plain_name({ value });
			return generic(m.products_plain_label_name(), op, value);
		case 'kind':
			return generic(m.products_plain_label_kind(), op, value);
		case 'term':
			if (plainEquals) return m.products_plain_term({ value });
			return generic(m.products_plain_label_term(), op, value);
		case 'tag':
		case 'tags':
		case 'metadata.tags':
			if (plainEquals) return m.products_plain_tag({ value });
			return generic(m.common_tags(), op, value);
		default:
			return generic(fieldLabel(path), op, value);
	}
}

/**
 * Reads a rule's search query in plain words ("Type is table and from kafka"), so anyone can tell what a rule matches without knowing the syntax. Grouped queries return null and the caller shows the raw query alone, since flattening the brackets would change their meaning.
 */
export function plainQuery(query: string | undefined): string | null {
	const q = (query ?? '').trim();
	if (!q || /[()]/.test(q.replace(/"[^"]*"/g, ''))) return null;

	const words = q.match(/"[^"]*"|\S+/g) ?? [];
	const parts: { joiner: Joiner; text: string }[] = [];
	let joiner: Joiner = 'AND';
	let negate = false;
	let current: string[] = [];
	const flush = () => {
		if (current.length)
			parts.push({ joiner: negate ? 'NOT' : joiner, text: condition(current.join(' ')) });
		current = [];
		joiner = 'AND';
		negate = false;
	};
	for (const word of words) {
		if (word === 'AND' || word === 'OR' || word === 'NOT') {
			flush();
			// "A NOT B" and "A AND NOT B" both mean A but not B, so NOT overrides the joiner.
			if (word === 'NOT') negate = true;
			else joiner = word;
			continue;
		}
		// Terms side by side are joined with AND by the search engine; free words stay together as one phrase.
		const complete = current.length > 0 && CONDITION.test(current.join(' '));
		const freeText = current.length > 0 && !current[0].startsWith('@');
		if (complete || (freeText && word.startsWith('@'))) flush();
		current.push(word);
	}
	flush();
	if (!parts.length) return null;

	return parts
		.map((part, i) => {
			if (i === 0) {
				return part.joiner === 'NOT' ? `${m.products_plain_not()} ${lower(part.text)}` : part.text;
			}
			const join =
				part.joiner === 'OR'
					? m.products_plain_or()
					: part.joiner === 'NOT'
						? m.products_plain_but_not()
						: m.products_plain_and();
			return `${join} ${lower(part.text)}`;
		})
		.join(' ');
}

function lower(text: string): string {
	return text.charAt(0).toLowerCase() + text.slice(1);
}
