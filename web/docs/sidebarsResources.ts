import type { SidebarsConfig } from "@docusaurus/plugin-content-docs";

// The /resources hub intentionally has NO sidebar. It is a shallow, SEO-focused
// resource centre: a homepage of cards links out to one level of child pages,
// and each page links back via in-page navigation and the footer Topics column.
// Exporting no sidebars makes every resources page render full width, like an
// article rather than documentation.
const sidebars: SidebarsConfig = {};

export default sidebars;
