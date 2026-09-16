import fsd from "@feature-sliced/steiger-plugin";
import { defineConfig } from "steiger";

export default defineConfig([
  ...fsd.configs.recommended,
  { ignores: ["**/*.test.*", "src/test/**", "src/shared/api/generated/**"] },
  {
    files: ["./src/**"],
    rules: {
      "fsd/insignificant-slice": "off",
      "fsd/repetitive-naming": "off",
      "fsd/segments-by-purpose": "off",
      "fsd/excessive-slicing": "off",
    },
  },
  {
    files: ["./src/shared/ui/**"],
    rules: { "fsd/public-api": "off", "fsd/import-locality": "off" },
  },
]);
