import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ConfigProvider, App as AntApp } from "antd";
import AppLayout from "@/components/layout/AppLayout";
import ProtectedRoute from "@/components/auth/ProtectedRoute";
import Login from "@/pages/Login";
import Dashboard from "@/pages/Dashboard";
import UserList from "@/pages/users/UserList";
import UserDetail from "@/pages/users/UserDetail";
import PlayerMerge from "@/pages/users/PlayerMerge";
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
import Affiliates from "@/pages/affiliates/Affiliates";
import AffiliateDetail from "@/pages/affiliates/AffiliateDetail";
import AffiliatePayouts from "@/pages/affiliates/AffiliatePayouts";
import AffiliateFraudFlags from "@/pages/affiliates/AffiliateFraudFlags";

// KYC Pages
import KycQueue from "@/pages/kyc/KycQueue";
import SofRequests from "@/pages/kyc/SofRequests";
import ExpiryMonitor from "@/pages/kyc/ExpiryMonitor";
import ScreeningQueue from "@/pages/kyc/ScreeningQueue";
import KycTeamDashboard from "@/pages/kyc/TeamDashboard";
import RgDashboard from "@/pages/kyc/RgDashboard";

// Payment Pages
import Chargebacks from "@/pages/finance/Chargebacks";
import BalanceSheet from "@/pages/finance/BalanceSheet";
import CryptoWallets from "@/pages/finance/CryptoWallets";
import P2PQueue from "@/pages/finance/P2PQueue";
import Reconciliation from "@/pages/finance/Reconciliation";

// Support Pages
import TicketList from "@/pages/support/TicketList";
import TicketDetail from "@/pages/support/TicketDetail";
import NewTicket from "@/pages/support/NewTicket";
import SupportTeamDashboard from "@/pages/support/TeamDashboard";
import SlaConfig from "@/pages/support/SlaConfig";

// Sportsbook Pages
import SportsEvents from "@/pages/sports/SportsEvents";
import TradingTerminal from "@/pages/sports/TradingTerminal";

// Casino Pages
import RtpConfig from "@/pages/casino/RtpConfig";

// CRM Pages
import Campaigns from "@/pages/crm/Campaigns";
import Segments from "@/pages/crm/Segments";
import Templates from "@/pages/crm/Templates";

// Risk Pages
import RuleBuilder from "@/pages/risk/RuleBuilder";
import Screening from "@/pages/risk/Screening";

// Reports Pages
import FinancialReports from "@/pages/reports/FinancialReports";
import PlayerAnalytics from "@/pages/reports/PlayerAnalytics";
import ComplianceReports from "@/pages/reports/ComplianceReports";
import GameAnalytics from "@/pages/reports/GameAnalytics";

// Regulatory Pages
import RegulatoryDashboard from "@/pages/regulatory/RegulatoryDashboard";
import ReportGenerator from "@/pages/regulatory/ReportGenerator";
import SARManagement from "@/pages/regulatory/SARManagement";
import ComplaintsLog from "@/pages/regulatory/ComplaintsLog";
import TaxConfiguration from "@/pages/regulatory/TaxConfiguration";
import PlayerFundsReconciliation from "@/pages/regulatory/PlayerFundsReconciliation";

// CMS & Settings Pages
import CmsPages from "@/pages/cms/CmsPages";
import GeneralSettings from "@/pages/settings/GeneralSettings";
import MaintenanceMode from "@/pages/settings/MaintenanceMode";
import AdminUsers from "@/pages/settings/AdminUsers";
import AuditLogViewer from "@/pages/settings/AuditLogViewer";

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
          <BrowserRouter
            future={{
              v7_startTransition: true,
              v7_relativeSplatPath: true,
            }}
          >
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
                  path="users/merge"
                  element={
                    <ProtectedRoute permission="user.merge">
                      <PlayerMerge />
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
                  path="affiliates"
                  element={
                    <ProtectedRoute permission="affiliate.manage">
                      <Affiliates />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="affiliates/:id"
                  element={
                    <ProtectedRoute permission="affiliate.manage">
                      <AffiliateDetail />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="affiliates/payouts"
                  element={
                    <ProtectedRoute permission="affiliate.manage">
                      <AffiliatePayouts />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="affiliates/fraud"
                  element={
                    <ProtectedRoute permission="affiliate.manage">
                      <AffiliateFraudFlags />
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

                {/* KYC Module */}
                <Route
                  path="kyc/queue"
                  element={
                    <ProtectedRoute permission="kyc.review">
                      <KycQueue />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="kyc/sof"
                  element={
                    <ProtectedRoute permission="kyc.sof_review">
                      <SofRequests />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="kyc/expiry"
                  element={
                    <ProtectedRoute permission="kyc.review">
                      <ExpiryMonitor />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="kyc/screening"
                  element={
                    <ProtectedRoute permission="fraud.screening">
                      <ScreeningQueue />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="kyc/team"
                  element={
                    <ProtectedRoute permission="kyc.review">
                      <KycTeamDashboard />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="kyc/rg"
                  element={
                    <ProtectedRoute permission="kyc.review">
                      <RgDashboard />
                    </ProtectedRoute>
                  }
                />

                {/* Finance Additions */}
                <Route
                  path="finance/chargebacks"
                  element={
                    <ProtectedRoute permission="transaction.view">
                      <Chargebacks />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="finance/crypto"
                  element={
                    <ProtectedRoute permission="finance.crypto_wallet">
                      <CryptoWallets />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="finance/balance-sheet"
                  element={
                    <ProtectedRoute permission="finance.balance_sheet">
                      <BalanceSheet />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="finance/p2p"
                  element={
                    <ProtectedRoute permission="transaction.view">
                      <P2PQueue />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="finance/reconciliation"
                  element={
                    <ProtectedRoute permission="transaction.view">
                      <Reconciliation />
                    </ProtectedRoute>
                  }
                />

                {/* Sportsbook Additions */}
                <Route
                  path="sports/events"
                  element={
                    <ProtectedRoute permission="sportsbook.manage">
                      <SportsEvents />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="sports/trading"
                  element={
                    <ProtectedRoute permission="sportsbook.trading_terminal">
                      <TradingTerminal />
                    </ProtectedRoute>
                  }
                />

                {/* Casino Additions */}
                <Route
                  path="casino/rtp"
                  element={
                    <ProtectedRoute permission="casino.rtp_config">
                      <RtpConfig />
                    </ProtectedRoute>
                  }
                />

                {/* Support Module */}
                <Route
                  path="support/tickets"
                  element={
                    <ProtectedRoute permission="communication.manage">
                      <TicketList />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="support/tickets/new"
                  element={
                    <ProtectedRoute permission="communication.manage">
                      <NewTicket />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="support/tickets/:id"
                  element={
                    <ProtectedRoute permission="communication.manage">
                      <TicketDetail />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="support/dashboard"
                  element={
                    <ProtectedRoute permission="communication.manage">
                      <SupportTeamDashboard />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="support/sla"
                  element={
                    <ProtectedRoute permission="system.config">
                      <SlaConfig />
                    </ProtectedRoute>
                  }
                />

                {/* CRM Module */}
                <Route
                  path="crm/campaigns"
                  element={
                    <ProtectedRoute permission="crm.campaign">
                      <Campaigns />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="crm/segments"
                  element={
                    <ProtectedRoute permission="crm.segment">
                      <Segments />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="crm/templates"
                  element={
                    <ProtectedRoute permission="communication.manage">
                      <Templates />
                    </ProtectedRoute>
                  }
                />

                {/* Risk Additions */}
                <Route
                  path="risk/rules"
                  element={
                    <ProtectedRoute permission="fraud.rule_builder">
                      <RuleBuilder />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="risk/screening"
                  element={
                    <ProtectedRoute permission="fraud.screening">
                      <Screening />
                    </ProtectedRoute>
                  }
                />

                {/* Reports Module */}
                <Route
                  path="reports/financial"
                  element={
                    <ProtectedRoute permission="reports.view">
                      <FinancialReports />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="reports/player"
                  element={
                    <ProtectedRoute permission="reports.view">
                      <PlayerAnalytics />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="reports/compliance"
                  element={
                    <ProtectedRoute permission="reports.view">
                      <ComplianceReports />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="reports/games"
                  element={
                    <ProtectedRoute permission="reports.view">
                      <GameAnalytics />
                    </ProtectedRoute>
                  }
                />

                {/* Regulatory Module */}
                <Route
                  path="regulatory"
                  element={
                    <ProtectedRoute permission="reports.view">
                      <RegulatoryDashboard />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="regulatory/generator"
                  element={
                    <ProtectedRoute permission="reports.view">
                      <ReportGenerator />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="regulatory/sar"
                  element={
                    <ProtectedRoute permission="reports.view">
                      <SARManagement />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="regulatory/complaints"
                  element={
                    <ProtectedRoute permission="reports.view">
                      <ComplaintsLog />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="regulatory/tax"
                  element={
                    <ProtectedRoute permission="reports.view">
                      <TaxConfiguration />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="regulatory/player-funds"
                  element={
                    <ProtectedRoute permission="reports.view">
                      <PlayerFundsReconciliation />
                    </ProtectedRoute>
                  }
                />

                {/* CMS Module */}
                <Route
                  path="cms"
                  element={
                    <ProtectedRoute permission="content.manage">
                      <CmsPages />
                    </ProtectedRoute>
                  }
                />

                {/* Settings Module */}
                <Route
                  path="settings/general"
                  element={
                    <ProtectedRoute permission="system.config">
                      <GeneralSettings />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="settings/maintenance"
                  element={
                    <ProtectedRoute permission="system.maintenance">
                      <MaintenanceMode />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="settings/admin-users"
                  element={
                    <ProtectedRoute permission="admin.manage">
                      <AdminUsers />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="settings/audit"
                  element={
                    <ProtectedRoute permission="audit.view">
                      <AuditLogViewer />
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
