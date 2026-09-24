package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"smarthome/backend/internal"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================
// JWT
// ============================================================

type tokenClaims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func generateToken(cfg *Config, u *internal.User) (string, error) {
	now := time.Now()
	claims := tokenClaims{
		UserID: u.ID,
		Email:  u.Email,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(u.ID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.TokenExpiry)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
}

func parseToken(cfg *Config, raw string) (*tokenClaims, error) {
	claims := &tokenClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("metode signing tidak valid")
		}
		return []byte(cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func extractToken(c *gin.Context) string {
	if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	// untuk WebSocket: token dikirim lewat query param ?token=...
	return c.Query("token")
}

// ============================================================
// Middleware
// ============================================================

func authMiddleware(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := extractToken(c)
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token tidak ditemukan"})
			return
		}
		claims, err := parseToken(cfg, raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token tidak valid atau kedaluwarsa"})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func adminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("role") != "super_admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "akses khusus super admin"})
			return
		}
		c.Next()
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// ============================================================
// Router
// ============================================================

func NewRouter(cfg *Config, db *sql.DB, hub *internal.Hub, bridge *internal.Bridge) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware())

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true, "broker_connected": bridge.IsConnected()})
		})

		auth := api.Group("/auth")
		{
			auth.POST("/register", registerHandler(db))
			auth.POST("/login", loginHandler(cfg, db))
			auth.GET("/me", authMiddleware(cfg), meHandler(db))
		}

		api.GET("/readings", authMiddleware(cfg), readingsHandler(db))
		api.GET("/readings/latest", authMiddleware(cfg), latestReadingHandler(db))
		api.GET("/lamp", authMiddleware(cfg), lampStatusHandler(bridge))
		api.POST("/lamp", authMiddleware(cfg), lampSetHandler(bridge))
		api.POST("/shading", authMiddleware(cfg), shadingSetHandler(bridge))
		api.GET("/status", authMiddleware(cfg), systemStatusHandler(bridge))

		admin := api.Group("/admin", authMiddleware(cfg), adminOnly())
		{
			admin.GET("/users", listUsersHandler(db))
			admin.POST("/users/:id/approve", setUserStatusHandler(db, "approved"))
			admin.POST("/users/:id/reject", setUserStatusHandler(db, "rejected"))
			admin.DELETE("/users/:id", deleteUserHandler(db))
		}
	}

	// WebSocket realtime (auth via header atau ?token=...)
	r.GET("/ws", authMiddleware(cfg), hub.ServeWS)

	return r
}

// ============================================================
// Handlers auth
// ============================================================

func registerHandler(db *sql.DB) gin.HandlerFunc {
	type req struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}
	return func(c *gin.Context) {
		var body req
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "input tidak valid: " + err.Error()})
			return
		}
		email := strings.ToLower(strings.TrimSpace(body.Email))
		hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memproses password"})
			return
		}
		res, err := db.Exec(
			`INSERT INTO users (name, email, password_hash, role, status) VALUES (?,?,?,?,?)`,
			strings.TrimSpace(body.Name), email, string(hash), "user", "pending",
		)
		if err != nil {
			if strings.Contains(err.Error(), "Duplicate entry") {
				c.JSON(http.StatusConflict, gin.H{"error": "email sudah terdaftar"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan user"})
			return
		}
		id, _ := res.LastInsertId()
		c.JSON(http.StatusCreated, gin.H{
			"id":      id,
			"name":    strings.TrimSpace(body.Name),
			"email":   email,
			"status":  "pending",
			"message": "akun berhasil dibuat, menunggu persetujuan super admin",
		})
	}
}

func loginHandler(cfg *Config, db *sql.DB) gin.HandlerFunc {
	type req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	return func(c *gin.Context) {
		var body req
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email dan password wajib diisi"})
			return
		}
		u, err := internal.FindUserByEmail(db, strings.ToLower(strings.TrimSpace(body.Email)))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "email atau password salah"})
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(body.Password)) != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "email atau password salah"})
			return
		}
		switch u.Status {
		case "pending":
			c.JSON(http.StatusForbidden, gin.H{"error": "akun masih menunggu persetujuan super admin"})
			return
		case "rejected":
			c.JSON(http.StatusForbidden, gin.H{"error": "akun ditolak, hubungi super admin"})
			return
		}
		token, err := generateToken(cfg, u)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membuat token"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token, "user": u})
	}
}

func meHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, err := internal.FindUserByID(db, c.GetInt64("user_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user tidak ditemukan"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": u})
	}
}

// ============================================================
// Handlers data sensor & lampu
// ============================================================

func readingsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := clampInt(c.DefaultQuery("limit", "100"), 1, 1000, 100)
		hours := clampInt(c.DefaultQuery("hours", "24"), 1, 24*30, 24)
		rows, err := internal.ReadingsSince(db, time.Now().Add(-time.Duration(hours)*time.Hour), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengambil data"})
			return
		}
		points := make([]gin.H, 0, len(rows))
		for _, r := range rows {
			points = append(points, gin.H{
				"time":        r.CreatedAt.UnixMilli(),
				"temperature": r.Temperature,
				"humidity":    r.Humidity,
			})
		}
		c.JSON(http.StatusOK, gin.H{"readings": points})
	}
}

func latestReadingHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		r, err := internal.LatestReading(db)
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusOK, gin.H{"reading": nil})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengambil data"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"reading": gin.H{
			"time":        r.CreatedAt.UnixMilli(),
			"temperature": r.Temperature,
			"humidity":    r.Humidity,
		}})
	}
}

func lampStatusHandler(bridge *internal.Bridge) gin.HandlerFunc {
	return func(c *gin.Context) {
		state := bridge.LampState()
		var lastMs int64
		if last := bridge.LastSeen(); !last.IsZero() {
			lastMs = last.UnixMilli()
		}
		c.JSON(http.StatusOK, gin.H{"state": state, "on": state == internal.LampOn, "last_seen": lastMs})
	}
}

func lampSetHandler(bridge *internal.Bridge) gin.HandlerFunc {
	type req struct {
		On bool `json:"on"`
	}
	return func(c *gin.Context) {
		var body req
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "payload tidak valid"})
			return
		}
		if err := bridge.PublishLamp(body.On); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"on": body.On, "published": true})
	}
}

func shadingSetHandler(bridge *internal.Bridge) gin.HandlerFunc {
	type req struct {
		Mode   string `json:"mode" binding:"required"`
		Degree int    `json:"degree"`
	}
	return func(c *gin.Context) {
		var body req
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "payload tidak valid"})
			return
		}
		if body.Mode != "manual" && body.Mode != "auto" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "mode harus 'manual' atau 'auto'"})
			return
		}
		if body.Mode == "manual" && (body.Degree < 0 || body.Degree > 180) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "degree harus 0-180"})
			return
		}
		if err := bridge.PublishShading(body.Mode, body.Degree); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"mode": body.Mode, "degree": body.Degree, "published": true})
	}
}

func systemStatusHandler(bridge *internal.Bridge) gin.HandlerFunc {
	return func(c *gin.Context) {
		var lastMs int64
		if last := bridge.LastSeen(); !last.IsZero() {
			lastMs = last.UnixMilli()
		}
		shadingMode, shadingDeg, shadingKnown := bridge.ShadingState()
		c.JSON(http.StatusOK, gin.H{
			"broker_connected": bridge.IsConnected(),
			"lamp_state":       bridge.LampState(),
			"shading_mode":     shadingMode,
			"shading_degree":   shadingDeg,
			"shading_known":    shadingKnown,
			"last_seen":        lastMs,
		})
	}
}

// ============================================================
// Handlers admin (manajemen user)
// ============================================================

func listUsersHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := c.Query("status") // opsional: pending | approved | rejected
		users, err := internal.ListUsers(db, status)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengambil daftar user"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"users": users})
	}
}

func setUserStatusHandler(db *sql.DB, status string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id tidak valid"})
			return
		}
		if id == c.GetInt64("user_id") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tidak bisa mengubah status akun sendiri"})
			return
		}
		if err := internal.SetUserStatus(db, id, status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user tidak ditemukan"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengubah status"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id, "status": status})
	}
}

func deleteUserHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id tidak valid"})
			return
		}
		if id == c.GetInt64("user_id") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tidak bisa menghapus akun sendiri"})
			return
		}
		if err := internal.DeleteUser(db, id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user tidak ditemukan"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghapus user"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id, "deleted": true})
	}
}

// ============================================================
// Util
// ============================================================

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func clampInt(raw string, min, max, def int) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}