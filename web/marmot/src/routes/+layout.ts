// Since we're using a static adapter with fallback and serving from a Go backend,
// we need to ensure the app properly hydrates on the client side.
// csr = true ensures client-side rendering is enabled for hydration.
export const ssr = false;
export const prerender = false;
export const csr = true;
