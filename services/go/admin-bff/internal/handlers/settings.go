package handlers

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func RegisterSettingsRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger) {
	system := router.Group("/system")
	settings := system.Group("/settings")

	// System settings key-value store
	settings.Get("", func(c *fiber.Ctx) error {
		var items []models.SystemSetting
		if err := db.Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		result := make(map[string]string, len(items))
		for _, it := range items {
			result[it.Key] = it.Value
		}
		return c.JSON(fiber.Map{"data": result})
	})

	settings.Put("", func(c *fiber.Ctx) error {
		var req map[string]string
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		for k, v := range req {
			if k == "" {
				continue
			}
			var s models.SystemSetting
			err := db.Where("key = ?", k).First(&s).Error
			if err != nil {
				s = models.SystemSetting{Key: k, Value: v, UpdatedAt: time.Now()}
				if err := db.Create(&s).Error; err != nil {
					log.Error("failed to create setting", zap.String("key", k), zap.Error(err))
				}
			} else {
				s.Value = v
				s.UpdatedAt = time.Now()
				if err := db.Save(&s).Error; err != nil {
					log.Error("failed to update setting", zap.String("key", k), zap.Error(err))
				}
			}
		}
		return c.JSON(fiber.Map{"success": true})
	})

	// Admin Users Management (under /admin/users/admin)
	adminUsers := router.Group("/users/admin")

	// Audit Logs (under /admin/system/audit-logs)
	audit := system.Group("/audit-logs")

	adminUsers.Get("", func(c *fiber.Ctx) error {
		search := c.Query("search", "")
		role := c.Query("role", "")
		var items []models.AdminUser
		q := db.Model(&models.AdminUser{})
		if search != "" {
			q = q.Where("email ILIKE ? OR first_name ILIKE ? OR last_name ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
		}
		if role != "" {
			q = q.Where("role = ?", role)
		}
		if err := q.Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		resp := make([]fiber.Map, 0, len(items))
		for _, u := range items {
			name := u.Email
			if u.FirstName != nil && *u.FirstName != "" {
				if u.LastName != nil {
					name = fmt.Sprintf("%s %s", *u.FirstName, *u.LastName)
				} else {
					name = *u.FirstName
				}
			}
			resp = append(resp, fiber.Map{
				"id":           fmt.Sprintf("%d", u.ID),
				"email":        u.Email,
				"name":         name,
				"role":         u.Role,
				"locked":       u.Status != "active",
				"totp_enabled": u.TOTPEnabled,
				"last_login_at": u.LastLoginAt,
				"last_login_ip": u.LastLoginIP,
				"created_at":   u.CreatedAt,
			})
		}
		return c.JSON(fiber.Map{"data": resp})
	})

	adminUsers.Post("", func(c *fiber.Ctx) error {
		var req struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Role     string `json:"role"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if req.Email == "" || req.Password == "" {
			return c.Status(400).JSON(fiber.Map{"error": "email and password required"})
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to hash password"})
		}
		u := models.AdminUser{
			Email:        req.Email,
			PasswordHash: string(hashed),
			Role:         req.Role,
			Status:       "active",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if req.Name != "" {
			u.FirstName = &req.Name
		}
		if err := db.Create(&u).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.Status(201).JSON(fiber.Map{"data": fiber.Map{"id": u.ID}})
	})

	adminUsers.Put("/:id", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
		}
		var req struct {
			Name   string `json:"name"`
			Email  string `json:"email"`
			Role   string `json:"role"`
			Locked *bool  `json:"locked,omitempty"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var u models.AdminUser
		if err := db.First(&u, id).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		if req.Email != "" {
			u.Email = req.Email
		}
		if req.Role != "" {
			u.Role = req.Role
		}
		if req.Name != "" {
			u.FirstName = &req.Name
		}
		if req.Locked != nil {
			if *req.Locked {
				u.Status = "locked"
			} else {
				u.Status = "active"
			}
		}
		u.UpdatedAt = time.Now()
		if err := db.Save(&u).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"success": true})
	})

	audit.Get("", func(c *fiber.Ctx) error {
		adminID := c.Query("admin_id", "")
		action := c.Query("action", "")
		resourceType := c.Query("resource_type", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 {
			page = 1
		}
		if ps < 1 || ps > 200 {
			ps = 50
		}
		q := db.Model(&models.AuditLog{})
		if adminID != "" {
			q = q.Where("admin_id = ?", adminID)
		}
		if action != "" {
			q = q.Where("action = ?", action)
		}
		if resourceType != "" {
			q = q.Where("resource_type = ?", resourceType)
		}
		var total int64
		q.Count(&total)
		var items []models.AuditLog
		q.Order("created_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)
		return c.JSON(fiber.Map{
			"data": items,
			"meta": fiber.Map{"total": total, "page": page, "page_size": ps},
		})
	})

	// TOTP reset (SUPER_ADMIN only — wipes totp_secret so admin can re-enrol)
	adminUsers.Post("/:id/reset-totp", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
		}
		if err := db.Model(&models.AdminUser{}).Where("id = ?", id).Updates(map[string]any{
			"totp_secret":  nil,
			"totp_enabled": false,
			"updated_at":   time.Now(),
		}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		log.Info("TOTP reset for admin", zap.Int("admin_id", id))
		return c.JSON(fiber.Map{"success": true})
	})

	// Change admin password (SUPER_ADMIN only)
	adminUsers.Post("/:id/change-password", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
		}
		var req struct {
			Password string `json:"password"`
		}
		if err := c.BodyParser(&req); err != nil || req.Password == "" {
			return c.Status(400).JSON(fiber.Map{"error": "password required"})
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "hash failed"})
		}
		if err := db.Model(&models.AdminUser{}).Where("id = ?", id).Updates(map[string]any{
			"password_hash": string(hashed),
			"updated_at":    time.Now(),
		}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"success": true})
	})

	// IP Whitelist Management
	ipwl := system.Group("/ip-whitelist")

	ipwl.Get("", func(c *fiber.Ctx) error {
		var items []models.IPWhitelistEntry
		if err := db.Order("created_at DESC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": items})
	})

	ipwl.Post("", func(c *fiber.Ctx) error {
		var req struct {
			IPAddress string `json:"ip_address"`
			Label     string `json:"label"`
			IsGlobal  bool   `json:"is_global"`
			AdminID   *int64 `json:"admin_id"`
		}
		if err := c.BodyParser(&req); err != nil || req.IPAddress == "" {
			return c.Status(400).JSON(fiber.Map{"error": "ip_address required"})
		}
		creatorID := fmt.Sprintf("%v", c.Locals("admin_id"))
		entry := models.IPWhitelistEntry{
			IPAddress: req.IPAddress,
			IsGlobal:  req.IsGlobal,
			AdminID:   req.AdminID,
			CreatedBy: creatorID,
			CreatedAt: time.Now(),
		}
		if req.Label != "" {
			entry.Label = &req.Label
		}
		if err := db.Create(&entry).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.Status(201).JSON(fiber.Map{"data": entry})
	})

	ipwl.Delete("/:id", func(c *fiber.Ctx) error {
		if err := db.Where("id = ?", c.Params("id")).Delete(&models.IPWhitelistEntry{}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"success": true})
	})

	// Frequency Caps (A7.3 Addendum)
	settings.Get("/frequency-caps", func(c *fiber.Ctx) error {
		var cfg models.FrequencyCap
		if err := db.Order("updated_at DESC").First(&cfg).Error; err != nil {
			// Return defaults if not configured yet
			return c.JSON(fiber.Map{"data": models.FrequencyCap{
				EmailPerDay: 1, EmailPerWeek: 3,
				SMSPerDay: 0, SMSPerWeek: 2,
				PushPerDay: 3, PushPerHour: 1,
			}})
		}
		return c.JSON(fiber.Map{"data": cfg})
	})

	settings.Put("/frequency-caps", func(c *fiber.Ctx) error {
		var req models.FrequencyCap
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		req.UpdatedAt = time.Now()
		req.UpdatedBy = fmt.Sprintf("%v", c.Locals("admin_id"))
		if err := db.Save(&req).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": req})
	})
}
