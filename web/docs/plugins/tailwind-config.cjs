/**
 * Infima's element styles (a, table, ul, h1…) are unlayered, and unlayered CSS beats any cascade
 * layer whatever its specificity, so the design system's layered classes (.mm-btn, .mm-link) would
 * lose to `a { color }`. Lifting the design system's rules, and the site's own styles built on
 * them (marketing, document, docs), out of their layers lets ordinary specificity decide: a class
 * beats an element selector.
 */
const unlayer = () => ({
  postcssPlugin: "marmot-unlayer",
  Once(root) {
    root.walkAtRules("layer", (rule) => {
      const file = rule.source?.input?.file ?? "";
      if (!/@marmotdata[\\/]|design-system[\\/]packages[\\/]|src[\\/]css[\\/](marketing|document|docs)\.css$/.test(file)) return;
      if (rule.nodes) rule.replaceWith(rule.nodes);
      else rule.remove();
    });
  },
});
unlayer.postcss = true;

function tailwindPlugin(context, options) {
  return {
    name: "tailwind-plugin",
    configurePostCss(postcssOptions) {
      postcssOptions.plugins = [
        // The design system's packages resolve through their exports maps, which postcss-import
        // cannot read; css-loader resolves them (and rewrites their font URLs) instead.
        require("postcss-import")({ filter: (path) => !path.startsWith("@marmotdata/") }),
        unlayer(),
        require("tailwindcss"),
        require("autoprefixer"),
      ];
      return postcssOptions;
    },
  };
}
module.exports = tailwindPlugin;
