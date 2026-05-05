package api

import (
"context"
"database/sql"
"encoding/json"
"net/http"
"time"

"dashBoard2go/internal/oswrap"
"dashBoard2go/internal/queue"

"github.com/gin-gonic/gin"
)

// SetupRoutes registers the HTTP routes for the web interface and API
func SetupRoutes(r *gin.Engine, db *sql.DB, q queue.JobQueue) {
// Root redirect depending on state/login
r.GET("/", func(c *gin.Context) {
c.Redirect(http.StatusMovedPermanently, "/user")
})

// API Group Strategy
apiGroup := r.Group("/api/v1")
{
// Admin Endpoints
admin := apiGroup.Group("/admin")
{
admin.GET("/status", func(c *gin.Context) {
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

// Create a unified Panel User (System Account + Base Directories)
admin.POST("/accounts", func(c *gin.Context) {
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

// User Endpoints
user := apiGroup.Group("/user")
{
// Add Domain/Vhost
user.POST("/vhost", func(c *gin.Context) {
var req struct {
Domain string `json:"domain"`
Engine string `json:"engine"`
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
