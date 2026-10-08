/**
 * Flags hard-coded user-facing English in Svelte templates so it moves into messages/en.json.
 */

// Attributes whose string-literal values are user-facing and therefore need translating.
const TRANSLATABLE_ATTRIBUTES = new Set([
	'placeholder',
	'aria-label',
	'title',
	'alt',
	'label',
	'text'
]);

// Two consecutive letters is the minimal signal of a word, letting punctuation and digits through without special casing
const CONSECUTIVE_LATIN = /[A-Za-z]{2}/;

// A URL shown as an example of the format a field expects is a technical token, not copy
const TECHNICAL_TOKEN = /^(https?:\/\/\S*|\S+\.\S+\/\S*)$/;

// Returns the trimmed text when it should be reported, null when it is allowed
function reportableText(raw) {
	const text = raw.trim();
	if (!CONSECUTIVE_LATIN.test(text)) return null;
	if (text === 'Marmot') return null;
	if (TECHNICAL_TOKEN.test(text)) return null;
	return text;
}

// Truncated so a long paragraph does not swamp the lint output
function truncate(text) {
	return text.length > 40 ? `${text.slice(0, 40)}…` : text;
}

export default {
	meta: {
		type: 'suggestion',
		docs: {
			description:
				'disallow untranslated user-facing strings in Svelte templates; use Paraglide m.<key>() messages instead'
		},
		schema: [],
		messages: {
			untranslatedText:
				'Untranslated template text "{{text}}". Move it to messages/en.json and use m.<key>().',
			untranslatedAttribute:
				'Untranslated string in {{attribute}} attribute: "{{text}}". Move it to messages/en.json and use m.<key>().'
		}
	},
	create(context) {
		// A no-op on plain TS/JS, where svelte-eslint-parser produces no template AST
		return {
			SvelteText(node) {
				// Text inside <style> blocks is CSS, not copy
				if (node.parent && node.parent.type === 'SvelteStyleElement') return;
				const text = reportableText(node.value);
				if (text === null) return;
				context.report({ node, messageId: 'untranslatedText', data: { text: truncate(text) } });
			},
			SvelteAttribute(node) {
				if (!TRANSLATABLE_ATTRIBUTES.has(node.key.name)) return;
				// Only literal chunks are flagged, so title={m.some_key()} passes through untouched
				for (const part of node.value) {
					if (part.type !== 'SvelteLiteral') continue;
					const text = reportableText(part.value);
					if (text === null) continue;
					context.report({
						node: part,
						messageId: 'untranslatedAttribute',
						data: { attribute: node.key.name, text: truncate(text) }
					});
				}
			}
		};
	}
};
