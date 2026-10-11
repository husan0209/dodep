import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "@shared": path.resolve(__dirname, "../../libs/shared/typescript/src"),
    },
  },
  test: {
    // Component tests render antd + React 18; a DOM is required. Every test
    // file in this suite touches the DOM (directly or through zustand's
    // localStorage-backed persist middleware), so jsdom is the global default.
    environment: "jsdom",
    globals: false,
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
    restoreMocks: true,
    // Fail loudly instead of silently collecting nothing.
    passWithNoTests: false,
  },
});
