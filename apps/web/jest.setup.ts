import "@testing-library/jest-dom";
import { webcrypto } from "node:crypto";

// jsdom implements neither of the two Web APIs the app relies on, so specs that
// touch the API client or any crypto helper blew up before reaching an
// assertion:
//
//   * `jest.spyOn(global, "fetch")` threw "Property `fetch` does not exist in
//     the provided object";
//   * `crypto.randomUUID()` is undefined, so the client threw a TypeError from
//     outside its own try/catch and the network-failure spec received a
//     TypeError instead of the ApiClientError it asserts on.
//
// Both exist in every browser and in the Node runtime Next.js builds against,
// so supply them from there rather than weakening the tests.
const globals = globalThis as typeof globalThis & {
  crypto?: { randomUUID?: () => string };
};

if (typeof globals.crypto === "undefined") {
  globals.crypto = webcrypto as unknown as typeof globals.crypto;
}

if (typeof globals.crypto.randomUUID !== "function") {
  globals.crypto.randomUUID = () => webcrypto.randomUUID();
}

if (typeof globals.fetch === "undefined") {
  globals.fetch = ((...args: Parameters<typeof fetch>) =>
    fetch(...args)) as typeof fetch;
}