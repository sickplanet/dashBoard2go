package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"dashBoard2go/internal/oswrap"
	"dashBoard2go/internal/queue"
	"dashBoard2go/internal/wrappers/firewall"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// SetupRoutes registers the HTTP routes for the web interface and API
func SetupRoutes(r *gin.Engine, db *sql.DB, q queue.JobQueue) {
	// Root redirect depending on state/login
	r.GET("/", func(c *gin.Context) {
		sessionToken, err := c.Cookie("dashboard_session")
		if err != nil || sessionToken == "" {
			c.Redirect(http.StatusMovedPermanently, "/login")
			return
		}

		var isAdmin bool
		err = db.QueryRow("SELECT is_admin FROM sessions JOIN panel_users ON sessions.username = panel_users.username WHERE sessions.token = ? AND sessions.expires_at > ?", sessionToken, time.Now()).Scan(&isAdmin)
		if err != nil {
			// Invalid session or expired
			c.SetCookie("dashboard_session", "", -1, "/", "", false, true)
			c.Redirect(http.StatusMovedPermanently, "/login")
			return
		}

		if isAdmin {
			c.Redirect(http.StatusMovedPermanently, "/admin")
		} else {
			c.Redirect(http.StatusMovedPermanently, "/user")
		}
	})

	r.StaticFile("/login", "./web/login.html")

	// API Group Strategy
	apiGroup := r.Group("/api/v1")
	{
		// Auth Endpoints
		auth := apiGroup.Group("/auth")
		{
			auth.POST("/login", func(c *gin.Context) {
				var req struct {
					Username string `json:"username"`
					Password string `json:"password"`
				}
				if err := c.BindJSON(&req); err != nil {
					c.JSON(400, gin.H{"error": "Invalid request format"})
					return
				}

				var hash string
				var isAdmin bool
				err := db.QueryRow("SELECT password, is_admin FROM panel_users WHERE username = ?", req.Username).Scan(&hash, &isAdmin)

				if err == sql.ErrNoRows {
					c.JSON(401, gin.H{"error": "Invalid username or password"})
					return
				} else if err != nil {
					c.JSON(500, gin.H{"error": "Internal server error"})
					return
				}

				if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
					c.JSON(401, gin.H{"error": "Invalid username or password"})
					return
				}

				// Session management
				sessionToken := uuid.New().String()
				expiresAt := time.Now().Add(24 * time.Hour)

				_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
					token TEXT PRIMARY KEY,
					username TEXT NOT NULL,
					expires_at DATETIME NOT NULL
				)`)

				_, err = db.Exec("INSERT INTO sessions (token, username, expires_at) VALUES (?, ?, ?)", sessionToken, req.Username, expiresAt)
				if err != nil {
					c.JSON(500, gin.H{"error": "Failed to create session"})
					return
				}

				c.SetCookie("dashboard_session", sessionToken, 86400, "/", "", false, true)

				c.JSON(200, gin.H{
					"message":  "Login successful",
					"is_admin": isAdmin,
				})
			})
		}

		// Admin Endpoints
		// TODO: Implement authentication/authorization middleware to protect /admin routes
		admin := apiGroup.Group("/admin")
		{
			admin.GET("/status", func(c *gin.Context) {
				// TODO: Implement actual status API logic and system health checks
				c.JSON(200, gin.H{"status": "Admin API OK"})
			})
			admin.GET("/services", func(c *gin.Context) {
				services := []string{"nginx", "apache2", "mariadb", "postgresql", "bind9", "postfix", "dovecot"}
				var statuses []map[string]interface{}
				for _, s := range services {
					statuses = append(statuses, oswrap.GetServiceStatus(s))
				}
				c.JSON(200, statuses)
			})

			admin.GET("/updates", func(c *gin.Context) {
				var update string
				err := db.QueryRow("SELECT value FROM system_settings WHERE key = 'update_available'").Scan(&update)
				if err != nil {
					c.JSON(200, gin.H{"update_available": ""})
					return
				}
				c.JSON(200, gin.H{"update_available": update})
			})

			admin.GET("/firewall", func(c *gin.Context) {
				ufw := firewall.NewUFWWrapper(db)
				systemRules, err := ufw.GetSystemRules(c.Request.Context())
				if err != nil {
					systemRules = []firewall.FirewallRule{}
				}
				dbRules, err := ufw.GetDBRules(c.Request.Context())
				if err != nil {
					dbRules = []firewall.FirewallRule{}
				}
				c.JSON(200, gin.H{"rules": systemRules, "sqlRules": dbRules})
			})

			admin.GET("/users", func(c *gin.Context) {
				rows, err := db.Query("SELECT id, username, is_admin FROM panel_users")
				if err != nil {
					c.JSON(200, []interface{}{})
					return
				}
				defer rows.Close()

				var results []map[string]interface{}
				for rows.Next() {
					var id int
					var username string
					var isAdmin bool
					if err := rows.Scan(&id, &username, &isAdmin); err == nil {
						results = append(results, map[string]interface{}{"id": id, "username": username, "is_admin": isAdmin})
					}
				}
				if results == nil {
					results = []map[string]interface{}{}
				}
				c.JSON(200, results)
			})

			// Create a unified Panel User (System Account + Base Directories)
			admin.POST("/users", func(c *gin.Context) {
				var req struct {
					Username string `json:"username"`
					Password string `json:"password"`
					QuotaMB  int    `json:"quota_mb"`
				}
				if err := c.BindJSON(&req); err != nil {
					c.JSON(400, gin.H{"error": "Invalid JSON"})
					return
				}

				payload, _ := json.Marshal(req)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				jobID, err := q.Push(ctx, "CREATE_PANEL_USER", string(payload))
				if err != nil {
					c.JSON(500, gin.H{"error": "Failed to queue job"})
					return
				}
				c.JSON(201, gin.H{"message": "User creation queued", "job_id": jobID, "system_path": "/home/dashboard2go/users/" + req.Username})
			})
		}

		// Admin Domains
		admin.GET("/domains", func(c *gin.Context) {
			rows, err := db.Query("SELECT id, username, domain, php_version, ssl_enabled FROM domains")
			if err != nil {
				c.JSON(200, gin.H{"domains": []interface{}{}})
				return
			}
			defer rows.Close()

			var results []map[string]interface{}
			for rows.Next() {
				var id int
				var username, domain, phpV string
				var ssl bool
				if err := rows.Scan(&id, &username, &domain, &phpV, &ssl); err == nil {
					results = append(results, map[string]interface{}{"id": id, "username": username, "domain": domain, "php_version": phpV, "ssl_enabled": ssl})
				}
			}
			if results == nil {
				results = []map[string]interface{}{}
			}
			c.JSON(200, gin.H{"domains": results})
		})

		// Admin Databases
		admin.GET("/databases", func(c *gin.Context) {
			rows, err := db.Query("SELECT id, username, db_name, db_user, db_host FROM databases")
			if err != nil {
				c.JSON(200, gin.H{"databases": []interface{}{}})
				return
			}
			defer rows.Close()

			var results []map[string]interface{}
			for rows.Next() {
				var id int
				var username, dbName, dbUser, dbHost string
				if err := rows.Scan(&id, &username, &dbName, &dbUser, &dbHost); err == nil {
					results = append(results, map[string]interface{}{"id": id, "username": username, "db_name": dbName, "db_user": dbUser, "db_host": dbHost})
				}
			}
			if results == nil {
				results = []map[string]interface{}{}
			}
			c.JSON(200, gin.H{"databases": results})
		})

		// Admin Emails
		admin.GET("/emails", func(c *gin.Context) {
			rows, err := db.Query("SELECT id, username, address, quota FROM mailboxes")
			if err != nil {
				c.JSON(200, gin.H{"emails": []interface{}{}})
				return
			}
			defer rows.Close()

			var results []map[string]interface{}
			for rows.Next() {
				var id, quota int
				var username, address string
				if err := rows.Scan(&id, &username, &address, &quota); err == nil {
					results = append(results, map[string]interface{}{"id": id, "username": username, "address": address, "quota": quota})
				}
			}
			if results == nil {
				results = []map[string]interface{}{}
			}
			c.JSON(200, gin.H{"emails": results})
		})

		// User Endpoints
		// TODO: Implement authentication/authorization middleware to protect /user routes
		user := apiGroup.Group("/user")
		{
			// Add Domain/Vhost
			user.POST("/domains", func(c *gin.Context) {
				var req struct {
					Domain   string `json:"domain"`
					Engine   string `json:"engine"`
					Username string `json:"username"`
				}
				if err := c.BindJSON(&req); err != nil {
					c.JSON(400, gin.H{"error": "Invalid request"})
					return
				}

				payload, _ := json.Marshal(req)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				jobID, err := q.Push(ctx, "CREATE_VHOST", string(payload))
				if err != nil {
					c.JSON(500, gin.H{"error": "Queue error"})
					return
				}
				c.JSON(202, gin.H{"message": "Virtual host creation queued", "job_id": jobID})
			})

			// Add FTP Account
			user.POST("/ftp", func(c *gin.Context) {
				var req struct {
					Username string `json:"username"`
					Password string `json:"password"`
					Quota    int    `json:"quota"`
					Dir      string `json:"dir"`
				}
				if err := c.BindJSON(&req); err != nil {
					c.JSON(400, gin.H{"error": "Invalid JSON"})
					return
				}
				payload, _ := json.Marshal(req)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				jobID, err := q.Push(ctx, "ADD_FTP_USER", string(payload))
				if err != nil {
					c.JSON(500, gin.H{"error": err.Error()})
					return
				}
				c.JSON(201, gin.H{"message": "FTP user creation queued", "job_id": jobID})
			})

			// Get Log Alerts (Watched by Watchdog module)
			user.GET("/alerts", func(c *gin.Context) {
				// TODO: Fetch authenticated username from context/session properly instead of hardcoding
				username := "demo_user"

				// Ensure table exists safely inside the API to prevent errors
				_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS user_alerts (
id INTEGER PRIMARY KEY AUTOINCREMENT,
username TEXT NOT NULL,
message TEXT NOT NULL,
created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
notified BOOLEAN DEFAULT 0
)`)

				rows, err := db.Query("SELECT message FROM user_alerts WHERE username = ? ORDER BY id DESC LIMIT 50", username)
				if err != nil {
					c.JSON(200, gin.H{"alerts": []string{"No new alerts."}})
					return
				}
				defer rows.Close()

				var alerts []string
				for rows.Next() {
					var msg string
					if err := rows.Scan(&msg); err == nil {
						alerts = append(alerts, msg)
					}
				}

				if len(alerts) == 0 {
					c.JSON(200, gin.H{"alerts": []string{"No active alerts found."}})
					return
				}

				c.JSON(200, gin.H{"alerts": alerts})
			})

			// Configure Log Alert Level
			user.POST("/alerts/config", func(c *gin.Context) {
				var req struct {
					Username string `json:"username"`
					Level    string `json:"level"` // "none", "errors", "warnings", "all"
				}
				if err := c.BindJSON(&req); err != nil {
					c.JSON(400, gin.H{"error": "Invalid input"})
					return
				}

				// Ensure table exists
				_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS user_alert_prefs (
username TEXT PRIMARY KEY,
level TEXT DEFAULT 'errors'
)`)

				// Save level to SQLite user prefs table
				_, err := db.Exec(`INSERT INTO user_alert_prefs (username, level) VALUES (?, ?) 
ON CONFLICT(username) DO UPDATE SET level = excluded.level`,
					req.Username, req.Level)

				if err != nil {
					c.JSON(500, gin.H{"error": "Database error"})
					return
				}

				c.JSON(200, gin.H{"message": "Alert preferences updated to " + req.Level})
			})
		}
	}
}
