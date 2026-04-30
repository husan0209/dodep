import { Navigate, useLocation } from "react-router-dom";
import { useAuthStore } from "@/stores/authStore";
import { hasPermission } from "@/utils/permissions";
import type { Permission } from "@/types/admin";
import { Result, Button } from "antd";

interface ProtectedRouteProps {
  children: React.ReactNode;
  permission?: Permission;
}

export default function ProtectedRoute({
  children,
  permission,
}: ProtectedRouteProps) {
  const { isAuthenticated, permissions } = useAuthStore();
  const location = useLocation();

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  if (permission && !hasPermission(permissions, permission)) {
    return (
      <Result
        status="403"
        title="403"
        subTitle="You don't have permission to access this page."
        extra={
          <Button type="primary" onClick={() => window.history.back()}>
            Go Back
          </Button>
        }
      />
    );
  }

  return <>{children}</>;
}
