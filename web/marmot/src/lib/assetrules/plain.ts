import {
	parseSearch,
	type SearchCondition,
	type SearchOperator
} from '@marmotdata/elements/search-bar.js';
import { getProvider } from '@marmotdata/icons';
import { m } from '$lib/paraglide/messages';

// Plain-language rendering of rule queries ("Has the glossary term PII and from Snowflake"), shared by asset rules and data product rules so both read the same way.

const operatorWord = (op: SearchOperator): string =>
	({
		'=': m.assetrules_op_is(),
		'!=': m.assetrules_op_is_not(),
		contains: m.assetrules_op_contains(),
		matches: m.assetrules_op_matches(),
		'>': m.assetrules_op_more(),
		'<': m.assetrules_op_less(),
		'>=': m.assetrules_op_at_least(),
		'<=': m.assetrules_op_at_most(),
		range: m.assetrules_op_between()
	})[op];

/** The field in everyday words: `type` reads "Type", `metadata.row_count` reads "Row count". */
export function fieldLabel(field: string): string {
	switch (field) {
		case 'type':
			return m.assetrules_field_type();
		case 'kind':
			return m.assetrules_field_kind();
		case 'name':
			return m.assetrules_field_name();
		case 'provider':
			return m.assetrules_field_provider();
		case 'term':
			return m.assetrules_field_term();
	}
	const path = field.replace(/^metadata\./, '').replace(/[._]+/g, ' ');
	return path.charAt(0).toUpperCase() + path.slice(1);
}

/** A provider's own name ("PostgreSQL") when its logo is known, otherwise the value as written. */
export function providerLabel(value: string): string {
	return getProvider(value)?.name ?? value;
}

function describeCondition(c: SearchCondition): string {
	const or = ` ${m.assetrules_plain_or()} `;
	const values = (c.field === 'provider' ? c.values.map(providerLabel) : c.values).join(or);
	if (c.operator === 'range') {
		return m.assetrules_plain_range({
			field: fieldLabel(c.field),
			from: c.values[0] ?? '',
			to: c.to ?? ''
		});
	}
	if (c.field === 'provider' && c.operator === '=')
		return m.assetrules_plain_from({ value: values });
	if (c.field === 'provider' && c.operator === '!=')
		return m.assetrules_plain_not_from({ value: values });
	if (c.field === 'term' && c.operator === '=') return m.assetrules_plain_term({ value: values });
	if (c.field === 'name' && c.operator === '=') return m.assetrules_plain_named({ value: values });
	return m.assetrules_plain_condition({
		field: fieldLabel(c.field),
		operator: operatorWord(c.operator),
		value: values
	});
}

const lowerFirst = (s: string) => s.charAt(0).toLowerCase() + s.slice(1);

function describeAll(query: string): string | null {
	const model = parseSearch(query);
	if (!model) return null;
	const parts = model.conditions.map(describeCondition);
	if (model.text) parts.unshift(m.assetrules_plain_text({ text: model.text }));
	if (!parts.length) return null;
	return parts.map((p, i) => (i === 0 ? p : lowerFirst(p))).join(` ${m.assetrules_plain_and()} `);
}

/** Splits on a top-level OR, outside quotes and brackets. */
function splitOr(query: string): string[] {
	const out: string[] = [];
	let depth = 0;
	let quoted = false;
	let start = 0;
	for (let i = 0; i < query.length; i++) {
		const ch = query[i];
		if (ch === '"') quoted = !quoted;
		if (quoted) continue;
		if (ch === '(' || ch === '[') depth++;
		if (ch === ')' || ch === ']') depth--;
		if (depth === 0 && query.startsWith(' OR ', i)) {
			out.push(query.slice(start, i));
			start = i + 4;
			i = start - 1;
		}
	}
	out.push(query.slice(start));
	return out.map((p) => p.trim()).filter(Boolean);
}

/**
 * Describes a Marmot search query in plain words, such as "From Snowflake and type is table". Returns null for an empty query and for queries too complex to say simply (nested groups), so callers show the exact query alone with a fixed caption.
 */
export function plainQuery(query: string | undefined | null): string | null {
	const q = (query ?? '').trim();
	if (!q) return null;
	const whole = describeAll(q);
	if (whole) return whole;
	const alternatives = splitOr(q);
	if (alternatives.length < 2) return null;
	const described = alternatives.map(describeAll);
	if (described.some((d) => d === null)) return null;
	return (described as string[])
		.map((d, i) => (i === 0 ? d : lowerFirst(d)))
		.join(`, ${m.assetrules_plain_or()} `);
}
