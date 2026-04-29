-- Migration 012: Support Ticket System
-- Phase 6: Tickets, Messages, Links, SLA Config

-- ============================================================
-- Support Tickets
-- ============================================================
CREATE TABLE IF NOT EXISTS support_tickets (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject                 TEXT NOT NULL,
    category                TEXT NOT NULL CHECK (category IN ('payment', 'bonus', 'technical', 'account', 'kyc', 'general')),
    priority                TEXT NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    status                  TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'pending_player', 'pending_internal', 'resolved', 'closed')),
    assigned_to             UUID REFERENCES admin_users(id),
    created_via             TEXT NOT NULL CHECK (created_via IN ('chat', 'email', 'manual')),
    source_chat_id          TEXT,
    sla_first_response_at   TIMESTAMPTZ,
    first_response_at       TIMESTAMPTZ,
    sla_resolve_at          TIMESTAMPTZ,
    resolved_at             TIMESTAMPTZ,
    closed_at               TIMESTAMPTZ,
    created_at              TIMESTAMPTZ DEFAULT now(),
    updated_at              TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_tickets_player ON support_tickets(player_id);
CREATE INDEX idx_tickets_status ON support_tickets(status) WHERE status NOT IN ('resolved', 'closed');
CREATE INDEX idx_tickets_assigned ON support_tickets(assigned_to) WHERE status = 'open';
CREATE INDEX idx_tickets_category ON support_tickets(category, priority);
CREATE INDEX idx_tickets_created ON support_tickets(created_at DESC);

-- ============================================================
-- Ticket Messages
-- ============================================================
CREATE TABLE IF NOT EXISTS ticket_messages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id   UUID NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    author_type TEXT NOT NULL CHECK (author_type IN ('player', 'admin')),
    author_id   UUID NOT NULL,
    is_internal BOOLEAN DEFAULT false,
    body        TEXT NOT NULL,
    attachments JSONB DEFAULT '[]',
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_messages_ticket ON ticket_messages(ticket_id, created_at DESC);

-- ============================================================
-- Ticket Links (to transactions, bonuses, bets)
-- ============================================================
CREATE TABLE IF NOT EXISTS ticket_links (
    ticket_id       UUID NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    entity_type     TEXT NOT NULL CHECK (entity_type IN ('withdrawal', 'deposit', 'bonus', 'bet', 'chargeback')),
    entity_id       UUID NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (ticket_id, entity_type, entity_id)
);

CREATE INDEX idx_ticket_links_ticket ON ticket_links(ticket_id);
CREATE INDEX idx_ticket_links_entity ON ticket_links(entity_type, entity_id);

-- ============================================================
-- SLA Configuration (per category)
-- ============================================================
CREATE TABLE IF NOT EXISTS sla_config (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category            TEXT NOT NULL UNIQUE CHECK (category IN ('payment', 'bonus', 'kyc', 'technical', 'account', 'general')),
    first_response_minutes INT NOT NULL DEFAULT 240,
    resolution_minutes  INT NOT NULL DEFAULT 2880,
    active              BOOLEAN DEFAULT true,
    created_at          TIMESTAMPTZ DEFAULT now(),
    updated_at          TIMESTAMPTZ DEFAULT now()
);

-- Insert default SLA configs
INSERT INTO sla_config (category, first_response_minutes, resolution_minutes)
VALUES
    ('payment', 30, 240),
    ('bonus', 120, 1440),
    ('kyc', 60, 1440),
    ('technical', 240, 2880),
    ('account', 240, 2880),
    ('general', 240, 2880)
ON CONFLICT (category) DO NOTHING;

-- ============================================================
-- Ticket Auto-Assignment Rules
-- ============================================================
CREATE TABLE IF NOT EXISTS ticket_assignment_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category        TEXT,
    priority        TEXT,
    assigned_team   TEXT NOT NULL,
    default_agent_id UUID REFERENCES admin_users(id),
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- ============================================================
-- Triggers
-- ============================================================
CREATE TRIGGER trg_support_tickets_updated
    BEFORE UPDATE ON support_tickets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_sla_config_updated
    BEFORE UPDATE ON sla_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
