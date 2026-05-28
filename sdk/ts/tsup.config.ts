import { defineConfig } from "tsup";

export default defineConfig({
  entry: [
    "src/index.ts",
    "src/integrations/langchain/index.ts",
    "src/integrations/claude-agent/index.ts",
  ],
  format: ["esm", "cjs"],
  dts: true,
  sourcemap: true,
  clean: true,
  target: "es2022",
  platform: "neutral",
  treeshake: true,
});
