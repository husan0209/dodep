import { randomUUID } from "node:crypto";

import "@testing-library/jest-dom";

// jest-environment-jsdom ships neither of these, while the app's API client
// calls both directly (src/lib/api/client.ts uses crypto.randomUUID for
// every request id, and fetch for the request itself).
//
// Without them, the client's own error handling never runs: the missing
// function throws a TypeError *before* its try block, so callers see a raw
// TypeError instead of the structured ApiClientError the client is built to
// produce — which is exactly what a network-failure test asserts.
if (typeof globalThis.fetch === "undefined") {
  globalThis.fetch = jest.fn() as unknown as typeof globalThis.fetch;
}

if (typeof globalThis.crypto?.randomUUID !== "function") {
  Object.defineProperty(globalThis.crypto, "randomUUID", {
    value: randomUUID,
    configurable: true,
    writable: true,
  });
}
