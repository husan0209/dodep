import { useEffect, useRef, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useAuthStore } from "@/stores/authStore";

const INACTIVITY_TIMEOUT_MS = 15 * 60 * 1000; // 15 minutes
const EVENTS = ["mousedown", "mousemove", "keydown", "click", "scroll", "touchstart"];

export function useInactivityLogout() {
  const navigate = useNavigate();
  const { clearAuth, isAuthenticated } = useAuthStore();
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const logout = useCallback(() => {
    clearAuth();
    navigate("/login", { replace: true });
  }, [clearAuth, navigate]);

  const resetTimer = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
    }
    if (isAuthenticated) {
      timerRef.current = setTimeout(logout, INACTIVITY_TIMEOUT_MS);
    }
  }, [isAuthenticated, logout]);

  useEffect(() => {
    if (!isAuthenticated) return;

    // Start timer
    resetTimer();

    // Add event listeners
    const listener = () => resetTimer();
    EVENTS.forEach((event) => document.addEventListener(event, listener));

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
      EVENTS.forEach((event) => document.removeEventListener(event, listener));
    };
  }, [isAuthenticated, resetTimer]);
}
