import { fetchApi } from '$lib/api';

// Doc images require authentication, but <img> tags cannot send the bearer token,
// so images are fetched with fetchApi and displayed via object URLs.
const DOC_IMAGE_PATH =
	/^\/api\/v1\/docs\/images\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

const DOC_IMAGE_SRC =
	/(<img\b[^>]*?)\ssrc="((?:https?:\/\/[^"/]+)?\/api\/v1\/docs\/images\/[0-9a-f-]{36})"/g;

const cache = new Map<string, Promise<string>>();

// Returns the same-origin path of a doc image URL, or null for anything else so
// the token is never sent to another origin.
export function docImagePath(src: string): string | null {
	try {
		const url = new URL(src, window.location.origin);
		if (url.origin !== window.location.origin || !DOC_IMAGE_PATH.test(url.pathname)) {
			return null;
		}
		return url.pathname;
	} catch {
		return null;
	}
}

export function loadDocImage(path: string): Promise<string> {
	let pending = cache.get(path);
	if (!pending) {
		pending = fetchApi(path, { prefix: null }).then(async (response) => {
			if (!response.ok) {
				throw new Error(`Failed to load image: ${response.status}`);
			}
			return URL.createObjectURL(await response.blob());
		});
		pending.catch(() => cache.delete(path));
		cache.set(path, pending);
	}
	return pending;
}

// Moves doc image URLs in sanitised HTML from src to data-doc-image-src so the
// browser does not request them unauthenticated before docImages hydrates them.
export function deferDocImages(html: string): string {
	return html.replace(DOC_IMAGE_SRC, '$1 data-doc-image-src="$2"');
}

// Svelte action that loads every deferred doc image inside node, including ones added
// later when the rendered content changes.
export function docImages(node: HTMLElement) {
	function hydrate() {
		node.querySelectorAll<HTMLImageElement>('img[data-doc-image-src]').forEach((img) => {
			const src = img.dataset.docImageSrc ?? '';
			const path = docImagePath(src);
			img.removeAttribute('data-doc-image-src');
			if (!path) {
				img.src = src;
				return;
			}
			loadDocImage(path)
				.then((objectUrl) => (img.src = objectUrl))
				.catch(() => {});
		});
	}

	hydrate();
	const observer = new MutationObserver(hydrate);
	observer.observe(node, { childList: true, subtree: true });
	return { destroy: () => observer.disconnect() };
}
