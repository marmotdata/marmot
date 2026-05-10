import DOMPurify from 'dompurify';
import { marked } from 'marked';

// Allow the `class` attribute on common rendered elements so prose styling and
// our @-mention spans/links survive sanitization. mention links carry
// "mention mention-team" / "mention mention-user".
const PROFILE = {
	ADD_ATTR: ['target', 'rel'],
	ALLOW_DATA_ATTR: false
};

export function sanitizeHtml(dirty: string): string {
	return DOMPurify.sanitize(dirty, PROFILE);
}

export function renderMarkdownSafe(markdown: string): string {
	const html = marked(markdown) as string;
	return sanitizeHtml(html);
}
