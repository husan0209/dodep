/**
 * Global test setup.
 *
 * Runs once per test file, before the file's imports are evaluated.
 */
import "@testing-library/jest-dom/vitest";
import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";

// jsdom does not implement matchMedia; antd's responsive observer touches it
// while rendering Grid/Result components.
if (typeof window.matchMedia !== "function") {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

// Unmount anything a component test left behind between test cases.
afterEach(() => {
  cleanup();
  localStorage.clear();
});
