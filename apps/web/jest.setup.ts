import "@testing-library/jest-dom";
import { randomUUID } from "node:crypto";
import { TextDecoder, TextEncoder } from "node:util";
import {
  CompressionStream,
  DecompressionStream,
  ReadableStream,
  TransformStream,
  WritableStream,
} from "node:stream/web";

// jest-environment-jsdom does not implement `fetch`, so `global.fetch` does not
// exist under Jest and `jest.spyOn(global, "fetch")` throws
// "Property `fetch` does not exist in the provided object".
//
// jsdom dropped its own XHR-based fetch stand-in in v12, and this project pins
// jest-environment-jsdom 29.7.0 (jsdom 20). The application calls global
// `fetch` directly, so the environment has to supply one.
//
// Node 18+ ships a spec-compliant fetch (undici) on Node's own globalThis, which
// Jest replaces with jsdom's window. Loading undici inside the jsdom sandbox
// additionally needs TextEncoder/TextDecoder and the WHATWG stream classes,
// which jsdom omits as well — so those are installed first, from node: builtins.
//
// Everything is installed only when missing, so a future jsdom that provides any
// of these natively keeps its own implementation. Tests that need to intercept
// traffic keep using jest.spyOn() as before.
const installGlobal = (name: string, value: unknown) => {
  const target = globalThis as unknown as Record<string, unknown>;
  if (typeof target[name] === "undefined") {
    target[name] = value;
  }
};

installGlobal("TextEncoder", TextEncoder);
installGlobal("TextDecoder", TextDecoder);

// jsdom 20 ships a `crypto` object without `randomUUID`, so `crypto.randomUUID()`
// — used by the API client to build X-Request-ID — throws a TypeError before the
// request is ever made. Real browsers have had it since 2022 (it is available in
// every browser that supports the app's baseline), so this restores the method
// rather than changing application code to work around a test-environment gap.
const cryptoGlobal = globalThis as unknown as { crypto?: { randomUUID?: unknown } };
if (cryptoGlobal.crypto && typeof cryptoGlobal.crypto.randomUUID === "undefined") {
  cryptoGlobal.crypto.randomUUID = randomUUID;
}
installGlobal("ReadableStream", ReadableStream);
installGlobal("WritableStream", WritableStream);
installGlobal("TransformStream", TransformStream);
installGlobal("CompressionStream", CompressionStream);
installGlobal("DecompressionStream", DecompressionStream);
installGlobal("ByteLengthQueuingStrategy", globalThis.ByteLengthQueuingStrategy);
installGlobal("CountQueuingStrategy", globalThis.CountQueuingStrategy);

// Loaded after the globals above are in place, because undici's module body
// reads them at import time.
// eslint-disable-next-line @typescript-eslint/no-require-imports
const undici = require("undici") as Record<string, unknown>;

for (const name of ["fetch", "Headers", "Request", "Response", "FormData", "File"]) {
  installGlobal(name, undici[name]);
}
