import "@testing-library/jest-dom";
import { randomUUID } from "node:crypto";

// jsdom does not implement either of these.
//
// fetch: without it, any test that stubs the network with
// `jest.spyOn(global, "fetch")` dies before it runs with
//   Property `fetch` does not exist in the provided object
// Every such test replaces this with its own mock, so a rejecting placeholder is
// enough -- and it keeps an unstubbed call from hanging instead of failing fast.
//
// crypto.randomUUID: jsdom ships crypto.getRandomValues but not randomUUID, and
// ApiClientClient calls it to build the X-Request-ID header *before* its
// try/catch around fetch. So without it every request test failed with a bare
// `TypeError: crypto.randomUUID is not a function` instead of the
// ApiClientError it was asserting on.
if (typeof globalThis.fetch === "undefined") {
  globalThis.fetch = (() =>
    Promise.reject(new TypeError("fetch failed"))) as unknown as typeof fetch;
}

if (typeof globalThis.crypto === "undefined") {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (globalThis as any).crypto = {};
}

if (typeof globalThis.crypto.randomUUID !== "function") {
  globalThis.crypto.randomUUID = randomUUID;
}