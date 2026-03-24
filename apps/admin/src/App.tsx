import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ConfigProvider, App as AntApp } from "antd";
import AppLayout from "@/components/layout/AppLayout";
import ProtectedRoute from "@/components/auth/ProtectedRoute";
import Login from "@/pages/Login";
import Dashboard from "@/pages/Dashboard";
import UserList from "@/pages/users/UserList";
import UserDetail from "@/pages/users/UserDetail";
import Deposits from "@/pages/finance/Deposits";
import Withdrawals from "@/pages/finance/Withdrawals";
import Transactions from "@/pages/finance/Transactions";
import Bets from "@/pages/sports/Bets";
import Games from "@/pages/casino/Games";
import Sessions from "@/pages/casino/Sessions";
import CampaignList from "@/pages/bonuses/CampaignList";
import FraudAlerts from "@/pages/risk/FraudAlerts";
import AuditLog from "@/pages/risk/AuditLog";
import Health from "@/pages/system/Health";
import Config from "@/pages/system/Config";
import NotFound from "@/pages/NotFound";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
      staleTime: 30000,
    },
  },
});

const theme = {
  token: {
    colorPrimary: "#1677ff",
    borderRadius: 6,
  },
};

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ConfigProvider theme={theme}>
        <AntApp>
          <BrowserRouter>
            <Routes>
              <Route path="/login" element={<Login />} />
              <Route
                path="/"
                element={
                  <ProtectedRoute>
                    <AppLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<Navigate to="/dashboard" replace />} />
                <Route path="dashboard" element={<Dashboard />} />
                <Route
                  path="users"
                  element={
                    <ProtectedRoute permission="user.view">
                      <UserList />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="users/:id"
                  element={
                    <ProtectedRoute permission="user.view">
                      <UserDetail />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="finance/deposits"
                  element={
                    <ProtectedRoute permission="transaction.view">
                      <Deposits />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="finance/withdrawals"
                  element={
                    <ProtectedRoute permission="transaction.view">
                      <Withdrawals />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="finance/transactions"
                  element={
                    <ProtectedRoute permission="transaction.view">
                      <Transactions />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="sports/bets"
                  element={
                    <ProtectedRoute permission="bet.view">
                      <Bets />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="casino/games"
                  element={
                    <ProtectedRoute permission="reports.view">
                      <Games />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="casino/sessions"
                  element={
                    <ProtectedRoute permission="reports.view">
                      <Sessions />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="bonuses"
                  element={
                    <ProtectedRoute permission="bonus.create">
                      <CampaignList />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="risk/alerts"
                  element={
                    <ProtectedRoute permission="fraud.review">
                      <FraudAlerts />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="risk/audit-log"
                  element={
                    <ProtectedRoute permission="reports.view">
                      <AuditLog />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="system/health"
                  element={
                    <ProtectedRoute permission="system.config">
                      <Health />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="system/config"
                  element={
                    <ProtectedRoute permission="system.config">
                      <Config />
                    </ProtectedRoute>
                  }
                />
                <Route path="*" element={<NotFound />} />
              </Route>
            </Routes>
          </BrowserRouter>
        </AntApp>
      </ConfigProvider>
    </QueryClientProvider>
  );
}

export default App;
