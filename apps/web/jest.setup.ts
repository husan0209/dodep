import '@testing-library/jest-dom'
// jest-environment-jsdom runs specs inside a jsdom realm, which implements
// neither `fetch` nor the WHATWG encoding/stream classes. Every spec that
// touches src/lib/api fails with
// "Property `fetch` does not exist on the provided object", so the platform
// gaps have to be closed here, once, instead of per spec.
import 'whatwg-fetch'
import { webcrypto } from 'node:crypto'
import { ReadableStream } from 'node:stream/web'
import { TextDecoder, TextEncoder } from 'node:util'

Object.assign(globalThis, {
  ReadableStream,
  TextDecoder,
  TextEncoder,
})

// jsdom < 21 ships `crypto.getRandomValues` but not `crypto.randomUUID`, which
// the API client uses for its X-Request-ID correlation header.
if (typeof globalThis.crypto?.randomUUID !== 'function') {
  Object.defineProperty(globalThis, 'crypto', {
    configurable: true,
    value: webcrypto,
    writable: true,
  })
}
