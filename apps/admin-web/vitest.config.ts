import react from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    css: true,
    coverage: {
      provider: "v8",
      include: [
        "src/entities/conversation/**/*.{ts,tsx}",
        "src/entities/run/**/*.{ts,tsx}",
        "src/entities/run-step/**/*.{ts,tsx}",
        "src/shared/api/sse/**/*.{ts,tsx}",
      ],
      exclude: ["**/*.test.{ts,tsx}", "**/index.ts"],
      thresholds: { branches: 85, functions: 85, lines: 85, statements: 85 },
      reporter: ["text", "json-summary"],
    },
  },
});
