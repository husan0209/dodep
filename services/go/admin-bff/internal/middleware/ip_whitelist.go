package middleware

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"gorm.io/gorm"
)

// IPWhitelist blocks requests from IPs not in the global or per-admin whitelist.
// Skips enforcement when the whitelist table is empty (development convenience).
func IPWhitelist(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Count total entries; if zero, skip enforcement (dev mode).
		var total int64
		db.Model(&models.IPWhitelistEntry{}).Count(&total)
		if total == 0 {
			return c.Next()
		}

		rawIP := c.IP()
		clientIP := net.ParseIP(strings.TrimSpace(rawIP))

		// Load global entries.
		var entries []models.IPWhitelistEntry
		db.Where("is_global = ?", true).Find(&entries)

		// Also load per-admin entries if we have an admin_id in locals.
		adminID, hasAdmin := c.Locals("admin_id").(float64)
		if hasAdmin {
			var adminEntries []models.IPWhitelistEntry
			db.Where("admin_id = ?", int64(adminID)).Find(&adminEntries)
			entries = append(entries, adminEntries...)
		}

		for _, e := range entries {
			// Support CIDR notation (e.g. 10.0.0.0/8) and plain IPs.
			if strings.Contains(e.IPAddress, "/") {
				_, network, err := net.ParseCIDR(e.IPAddress)
				if err == nil && clientIP != nil && network.Contains(clientIP) {
					return c.Next()
				}
			} else {
				allowed := net.ParseIP(strings.TrimSpace(e.IPAddress))
				if allowed != nil && clientIP != nil && allowed.Equal(clientIP) {
					return c.Next()
				}
			}
		}

		return c.Status(403).JSON(fiber.Map{
			"error": "IP_NOT_WHITELISTED",
			"message": "Access from this IP address is not permitted",
		})
	}
}
