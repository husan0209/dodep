import { describe, it, expect, beforeEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import ProtectedRoute from "@/components/auth/ProtectedRoute";
import { useAuthStore } from "@/stores/authStore";
import { getPermissionsForRole } from "@/utils/permissions";
import type { Permission } from "@/types/admin";

/** Lives outside <Routes> so it reports the location after a redirect too. */
function LocationProbe() {
  const location = useLocation();
  return <div data-testid="pathname">{location.pathname}</div>;
}

function renderAt(path: string, permission?: Permission) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <LocationProbe />
      <Routes>
        <Route path="/login" element={<div>LOGIN PAGE</div>} />
        <Route
          path="*"
          element={
            <ProtectedRoute permission={permission}>
              <div>SECRET PAGE</div>
            </ProtectedRoute>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  useAuthStore.getState().clearAuth();
});

describe("ProtectedRoute", () => {
  it("redirects an anonymous visitor to /login and renders nothing else", () => {
    renderAt("/users");

    expect(screen.getByText("LOGIN PAGE")).toBeInTheDocument();
    expect(screen.queryByText("SECRET PAGE")).not.toBeInTheDocument();
    expect(screen.getByTestId("pathname")).toHaveTextContent("/login");
  });

  it("renders the children once the admin holds the required permission", () => {
    useAuthStore.setState({
      isAuthenticated: true,
      permissions: getPermissionsForRole("SUPPORT_AGENT"),
    });

    renderAt("/users", "user.view");

    expect(screen.getByText("SECRET PAGE")).toBeInTheDocument();
    expect(screen.queryByText("LOGIN PAGE")).not.toBeInTheDocument();
    expect(screen.queryByText("403")).not.toBeInTheDocument();
    expect(screen.getByTestId("pathname")).toHaveTextContent("/users");
  });

  it("shows a 403 instead of the children when the permission is missing", () => {
    useAuthStore.setState({
      isAuthenticated: true,
      permissions: getPermissionsForRole("VIEWER"),
    });

    renderAt("/settings/admin-users", "admin.manage");

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(
      screen.getByText("You don't have permission to access this page."),
    ).toBeInTheDocument();
    expect(screen.queryByText("SECRET PAGE")).not.toBeInTheDocument();
    // A 403 is not a redirect: the admin stays on the URL they requested.
    expect(screen.getByTestId("pathname")).toHaveTextContent(
      "/settings/admin-users",
    );
  });

  it("treats VIEWER's user.view as sufficient for a user.view page", () => {
    useAuthStore.setState({
      isAuthenticated: true,
      permissions: getPermissionsForRole("VIEWER"),
    });

    renderAt("/users", "user.view");

    expect(screen.getByText("SECRET PAGE")).toBeInTheDocument();
    expect(screen.queryByText("403")).not.toBeInTheDocument();
  });

  it("blocks an authenticated admin whose permission list is empty", () => {
    useAuthStore.setState({ isAuthenticated: true, permissions: [] });

    renderAt("/users", "user.view");

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.queryByText("SECRET PAGE")).not.toBeInTheDocument();
  });

  it("renders the children when no permission is required", () => {
    useAuthStore.setState({
      isAuthenticated: true,
      permissions: getPermissionsForRole("VIEWER"),
    });

    renderAt("/dashboard");

    expect(screen.getByText("SECRET PAGE")).toBeInTheDocument();
    expect(screen.queryByText("403")).not.toBeInTheDocument();
  });

  it("gates on the live store, not the permissions captured at mount", () => {
    useAuthStore.setState({
      isAuthenticated: true,
      permissions: getPermissionsForRole("VIEWER"),
    });
    renderAt("/settings/admin-users", "admin.manage");
    expect(screen.getByText("403")).toBeInTheDocument();

    // A super admin signing in on another tab must unlock this one.
    act(() => {
      useAuthStore.setState({
        permissions: getPermissionsForRole("SUPER_ADMIN"),
      });
    });

    expect(screen.getByText("SECRET PAGE")).toBeInTheDocument();
    expect(screen.queryByText("403")).not.toBeInTheDocument();
  });
});
