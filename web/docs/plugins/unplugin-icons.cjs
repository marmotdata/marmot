const IconsPlugin = require("unplugin-icons/webpack").default;

function unpluginIconsPlugin(context, options) {
  return {
    name: "unplugin-icons-plugin",
    configureWebpack(config, isServer) {
      return {
        plugins: [
          IconsPlugin({
            compiler: "jsx",
            jsx: "react",
            autoInstall: true,
            ...options,
          }),
        ],
        resolve: {
          alias: {
            "~icons": "unplugin-icons/resolver",
          },
        },
      };
    },
  };
}

module.exports = unpluginIconsPlugin;
