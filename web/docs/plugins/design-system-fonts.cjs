/**
 * The design system's Geist fonts come in with @marmotdata/tokens/index.css. Docusaurus sends
 * fonts through url-loader, which fails on them under this webpack, so they are taken out of that
 * rule and emitted with webpack's own asset modules instead.
 */
const fromDesignSystem = /[\\/]@marmotdata[\\/]tokens[\\/].*\.woff2$/;

function designSystemFontsPlugin() {
  return {
    name: "design-system-fonts",
    configureWebpack(config) {
      for (const rule of config.module?.rules ?? []) {
        if (rule && rule.test instanceof RegExp && rule.test.test("font.woff2")) {
          rule.exclude = [].concat(rule.exclude ?? [], fromDesignSystem);
        }
      }
      return {
        module: {
          rules: [{ test: fromDesignSystem, type: "asset/resource", generator: { filename: "assets/fonts/[name]-[contenthash][ext]" } }],
        },
      };
    },
  };
}
module.exports = designSystemFontsPlugin;
