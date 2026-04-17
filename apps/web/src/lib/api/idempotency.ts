import { useState, useCallback } from "react";
import { v4 as uuidv4 } from "uuid";

/**
 * Generate idempotency key for financial operations
 * Used to prevent duplicate transactions
 */
export function generateIdempotencyKey(): string {
  return uuidv4();
}

/**
 * Hook for using idempotency key in forms
 * Regenerates key on demand
 */
export function useIdempotencyKey() {
  const [key, setKey] = useState(() => generateIdempotencyKey());
  const regenerate = useCallback(() => setKey(uuidv4()), []);
  return { key, regenerate };
}
