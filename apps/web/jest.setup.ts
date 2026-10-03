import "@testing-library/jest-dom";
import { webcrypto } from "node:crypto";

// jsdom does not implement crypto.randomUUID, but src/lib/api/client.ts calls
// it on every request to build the X-Request-ID header. Without this the call
// throws a TypeError before fetch is ever reached, which masks the real
// behaviour under test. Node's WebCrypto does provide it.
if (typeof globalThis.crypto === "undefined") {
  Object.defineProperty(globalThis, "crypto", { value: webcrypto, configurable: true });
} else if (typeof globalThis.crypto.randomUUID !== "function") {
  Object.defineProperty(globalThis.crypto, "randomUUID", {
    value: webcrypto.randomUUID.bind(webcrypto),
    configurable: true,
    writable: true,
  });
}
