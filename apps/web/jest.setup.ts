import "@testing-library/jest-dom";
import { webcrypto } from "node:crypto";

// jsdom implements `crypto.getRandomValues` but not `crypto.randomUUID`, so any code
// calling it under Jest throws `TypeError: crypto.randomUUID is not a function`.
// `src/lib/api/client.ts` builds a request id that way, which made the network-diagnostics
// test fail with a raw TypeError instead of the ApiClientError it asserts.
//
// Node's WebCrypto is used as the source so the ids stay RFC 4122 compliant.
if (typeof globalThis.crypto === "undefined") {
  Object.defineProperty(globalThis, "crypto", {
    value: webcrypto,
    configurable: true,
    writable: true,
  });
} else if (typeof globalThis.crypto.randomUUID !== "function") {
  Object.defineProperty(globalThis.crypto, "randomUUID", {
    value: () => webcrypto.randomUUID(),
    configurable: true,
    writable: true,
  });
}