import { m } from '$lib/paraglide/messages';

const catalogue = m as unknown as Record<string, (() => string) | undefined>;

/** Marmot's compiled catalogue. Profile keys use dots, message ids use underscores. */
export function nativeMessage(key: string): string | undefined {
	for (const candidate of [key, key.replaceAll('.', '_')]) {
		const message = catalogue[candidate];
		if (typeof message === 'function') return message();
	}
	return undefined;
}

export function violationMessage(code: string): string {
	switch (code) {
		case 'required':
			return m.metamodel_error_required();
		case 'not_nullable':
			return m.metamodel_error_not_nullable();
		case 'type':
			return m.metamodel_error_type();
		case 'range':
			return m.metamodel_error_range();
		case 'length':
			return m.metamodel_error_length();
		case 'items':
			return m.metamodel_error_items();
		case 'enum':
			return m.metamodel_error_enum();
		case 'date':
			return m.metamodel_error_date();
		default:
			return m.metamodel_error_unknown();
	}
}
