package models

import (
	"encoding/json"
	"time"
)

type SupportTicket struct {
	ID                 string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID           int64      `gorm:"not null;index" json:"player_id"`
	PlayerEmail        string     `gorm:"type:varchar(255);not null" json:"player_email"`
	PlayerUsername     string     `gorm:"type:varchar(100);not null" json:"player_username"`
	PlayerGroup        string     `gorm:"type:varchar(50);not null;default:'standard'" json:"player_group"`
	Subject            string     `gorm:"type:varchar(500);not null" json:"subject"`
	Category           string     `gorm:"type:ticket_category;not null;default:'general'" json:"category"`
	Priority           string     `gorm:"type:ticket_priority;not null;default:'normal'" json:"priority"`
	Status             string     `gorm:"type:ticket_status;not null;default:'open'" json:"status"`
	AssignedTo         *string    `gorm:"type:varchar(36)" json:"assigned_to,omitempty"`
	AssignedToName     *string    `gorm:"type:varchar(255)" json:"assigned_to_name,omitempty"`
	CreatedVia         string     `gorm:"type:ticket_created_via;not null;default:'manual'" json:"created_via"`
	SourceChatID       *string    `gorm:"type:varchar(100)" json:"source_chat_id,omitempty"`
	SLAFirstResponseAt *time.Time `json:"sla_first_response_at,omitempty"`
	FirstResponseAt    *time.Time `json:"first_response_at,omitempty"`
	SLAResolveAt       *time.Time `json:"sla_resolve_at,omitempty"`
	ResolvedAt         *time.Time `json:"resolved_at,omitempty"`
	ClosedAt           *time.Time `json:"closed_at,omitempty"`
	LastMessagePreview *string    `gorm:"type:varchar(500)" json:"last_message_preview,omitempty"`
	LastMessageAt      *time.Time `json:"last_message_at,omitempty"`
	MessageCount       int        `gorm:"not null;default:0" json:"message_count"`
	IsSLABreach        bool       `gorm:"not null;default:false" json:"is_sla_breach"`
	CreatedAt          time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

type SupportMessage struct {
	ID           string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TicketID     string          `gorm:"type:uuid;not null;index" json:"ticket_id"`
	AuthorType   string          `gorm:"type:message_author_type;not null" json:"author_type"`
	AuthorID     string          `gorm:"type:varchar(36);not null" json:"author_id"`
	AuthorName   string          `gorm:"type:varchar(255);not null" json:"author_name"`
	IsInternal   bool            `gorm:"not null;default:false" json:"is_internal"`
	Body         string          `gorm:"type:text;not null" json:"body"`
	Attachments  json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"attachments,omitempty"`
	CreatedAt    time.Time       `gorm:"not null;default:now()" json:"created_at"`
}

type TicketLink struct {
	ID             string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TicketID       string    `gorm:"type:uuid;not null;index" json:"ticket_id"`
	EntityType     string    `gorm:"type:varchar(50);not null" json:"entity_type"`
	EntityID       string    `gorm:"type:varchar(100);not null" json:"entity_id"`
	EntitySummary  *string   `gorm:"type:varchar(255)" json:"entity_summary,omitempty"`
	CreatedAt      time.Time `gorm:"not null;default:now()" json:"created_at"`
}

type SLAConfig struct {
	ID                     string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Category               string    `gorm:"type:ticket_category;not null;unique" json:"category"`
	FirstResponseMinutes   int       `gorm:"not null;default:30" json:"first_response_minutes"`
	ResolutionMinutes      int       `gorm:"not null;default:240" json:"resolution_minutes"`
	Active                 bool      `gorm:"not null;default:true" json:"active"`
	CreatedAt              time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt              time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

type AgentWorkload struct {
	ID                    string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AgentID               string    `gorm:"type:varchar(36);not null;uniqueIndex" json:"agent_id"`
	AgentName             string    `gorm:"type:varchar(255);not null" json:"agent_name"`
	OpenTickets           int       `gorm:"not null;default:0" json:"open_tickets"`
	ResolvedToday         int       `gorm:"not null;default:0" json:"resolved_today"`
	AvgResolutionMinutes  int       `gorm:"not null;default:0" json:"avg_resolution_minutes"`
	UpdatedAt             time.Time `gorm:"not null;default:now()" json:"updated_at"`
}
