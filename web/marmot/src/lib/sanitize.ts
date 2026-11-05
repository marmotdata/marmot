import DOMPurify, { type Config, type DOMPurify as Purifier } from 'dompurify';
import { marked } from 'marked';

// User-written markdown (docs, glossary descriptions) is rendered with {@html}, so everything it can produce is allowlisted here rather than trusting DOMPurify's broad defaults: only the elements and attributes markdown and the doc editor emit survive, and attribute values are checked too.

const ALLOWED_TAGS = [
	'p',
	'br',
	'hr',
	'h1',
	'h2',
	'h3',
	'h4',
	'h5',
	'h6',
	'strong',
	'b',
	'em',
	'i',
	's',
	'del',
	'code',
	'pre',
	'blockquote',
	'ul',
	'ol',
	'li',
	'a',
	'img',
	'table',
	'thead',
	'tbody',
	'tr',
	'th',
	'td',
	'span',
	'input'
];

const ALLOWED_ATTR = [
	'href',
	'src',
	'alt',
	'title',
	'class',
	'style',
	'align',
	'colspan',
	'rowspan',
	'start',
	'type',
	'checked',
	'disabled'
];

const CONFIG: Config = {
	ALLOWED_TAGS,
	ALLOWED_ATTR,
	ALLOW_DATA_ATTR: false,
	ALLOW_ARIA_ATTR: false,
	ALLOW_UNKNOWN_PROTOCOLS: false,
	ALLOW_SELF_CLOSE_IN_ATTR: false,
	SANITIZE_DOM: true,
	SANITIZE_NAMED_PROPS: true,
	KEEP_CONTENT: true
};

// Classes are limited to syntax highlighting and mentions, so content can never borrow the app's own classes to dress itself up as Marmot UI.
const SAFE_CLASS =
	/^(hljs(-[a-z0-9-]+)?|language-[a-z0-9+#-]+|token|mention|mention-user|mention-team)$/i;
// The doc editor keeps centred and right-aligned blocks as HTML with a text-align style; that is the only style that survives.
const TEXT_ALIGN = /^\s*text-align\s*:\s*(left|center|right)\s*;?\s*$/i;
const ALIGN_VALUE = /^(left|center|right)$/i;
const DATA_IMAGE = /^data:image\/(png|gif|jpe?g|webp);base64,[a-z0-9+/=\s]+$/i;

function sameOrigin(url: URL): boolean {
	return typeof window !== 'undefined' && url.origin === window.location.origin;
}

function parseURL(value: string): URL | null {
	try {
		return new URL(
			value,
			typeof window !== 'undefined' ? window.location.origin : 'http://localhost'
		);
	} catch {
		return null;
	}
}

// Links may go to the web, mail or this Marmot; protocol-relative and scheme-less tricks resolve through URL and are judged by the result.
function safeHref(value: string): boolean {
	const trimmed = value.trim();
	if (trimmed.startsWith('#')) return true;
	const url = parseURL(trimmed);
	if (!url) return false;
	return url.protocol === 'https:' || url.protocol === 'http:' || url.protocol === 'mailto:';
}

// Images may only come from this Marmot (doc uploads) or be inline; anything else would let a page track who reads it.
function safeSrc(value: string): boolean {
	const trimmed = value.trim();
	if (DATA_IMAGE.test(trimmed)) return true;
	const url = parseURL(trimmed);
	return !!url && (url.protocol === 'https:' || url.protocol === 'http:') && sameOrigin(url);
}

let purifier: Purifier | null = null;

function getPurifier(): Purifier {
	if (purifier) return purifier;
	// A dedicated instance keeps these hooks away from any other DOMPurify use in the app.
	const p = DOMPurify(window);

	p.addHook('uponSanitizeAttribute', (node, data) => {
		const name = data.attrName;
		const value = data.attrValue ?? '';
		const tag = node.nodeName.toLowerCase();
		let keep = true;
		switch (name) {
			case 'href':
				keep = tag === 'a' && safeHref(value);
				break;
			case 'src':
				keep = tag === 'img' && safeSrc(value);
				break;
			case 'style': {
				const m = value.match(TEXT_ALIGN);
				if (m) data.attrValue = `text-align: ${m[1].toLowerCase()}`;
				else keep = false;
				break;
			}
			case 'class':
				data.attrValue = value
					.split(/\s+/)
					.filter((c) => SAFE_CLASS.test(c))
					.join(' ');
				keep = data.attrValue.length > 0;
				break;
			case 'align':
				keep = ALIGN_VALUE.test(value);
				break;
			case 'type':
			case 'checked':
			case 'disabled':
				keep = tag === 'input';
				break;
		}
		if (!keep) data.keepAttr = false;
	});

	p.addHook('afterSanitizeAttributes', (node) => {
		const el = node as Element;
		const tag = el.nodeName.toLowerCase();
		// Only GFM task-list boxes are inputs, and they are display only.
		if (tag === 'input') {
			if (el.getAttribute('type') !== 'checkbox') {
				el.remove();
				return;
			}
			el.setAttribute('disabled', '');
		}
		if (tag === 'a') {
			const href = el.getAttribute('href');
			const url = href ? parseURL(href) : null;
			// Off-site links open in a new tab and can neither reach back into Marmot nor learn where they came from.
			if (url && !sameOrigin(url) && !href!.trim().startsWith('#')) {
				el.setAttribute('target', '_blank');
				el.setAttribute('rel', 'noopener noreferrer nofollow');
			} else {
				el.removeAttribute('target');
			}
		}
	});

	purifier = p;
	return p;
}

export function sanitizeHtml(dirty: string): string {
	if (typeof window === 'undefined') return '';
	return getPurifier().sanitize(dirty, CONFIG) as string;
}

// The doc editor loads a page as HTML too, so it gets the same rules plus the data attributes its mention nodes are rebuilt from.
const EDITOR_CONFIG: Config = {
	...CONFIG,
	ALLOWED_ATTR: [...ALLOWED_ATTR, 'data-type', 'data-id', 'data-label', 'data-mention-type']
};

/** Sanitises a page's HTML before the doc editor parses it, so stored content can never carry script into the editor either. */
export function sanitizeEditorHtml(dirty: string): string {
	if (typeof window === 'undefined') return '';
	return getPurifier().sanitize(dirty, EDITOR_CONFIG) as string;
}

export function escapeHtml(text: string): string {
	return text
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#39;');
}

export function renderMarkdownSafe(markdown: string): string {
	return sanitizeHtml(marked(markdown) as string);
}
