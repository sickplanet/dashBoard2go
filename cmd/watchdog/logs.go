package main

import (
"bufio"
"database/sql"
"fmt"
"log"
"os"
"path/filepath"
"strings"
)

type userAlertPref struct {
Level string // "none", "errors", "warnings", "both", "all"
}

func analyzeUserLogs(db *sql.DB) {
// Query all users and their domains to locate log files
usersDir := "/home/dashboard2go/users"
files, err := os.ReadDir(usersDir)
if err != nil {
return
}

for _, userDir := range files {
if !userDir.IsDir() {
continue
}
username := userDir.Name()

// Get user's log preference from DB
pref := getAlertPreference(db, username)
if pref.Level == "none" {
continue
}

// Find log files: /home/dashboard2go/users/{username}/web/*/logs/*.log
webDir := filepath.Join(usersDir, username, "web")
domains, err := os.ReadDir(webDir)
if err != nil {
continue
}

for _, domainDir := range domains {
if !domainDir.IsDir() {
continue
}
domain := domainDir.Name()
logsPath := filepath.Join(webDir, domain, "logs")

// Check Apache/Nginx Error Logs (both standard errors and fpm errors)
processErrorLog(db, username, domain, filepath.Join(logsPath, "error.log"), pref.Level)
}
}
}

func getAlertPreference(db *sql.DB, username string) userAlertPref {
_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS user_alert_prefs (
username TEXT PRIMARY KEY,
level TEXT DEFAULT 'errors'
)`)

var level string
err := db.QueryRow("SELECT level FROM user_alert_prefs WHERE username = ?", username).Scan(&level)
if err != nil {
return userAlertPref{Level: "errors"} // default
}

return userAlertPref{Level: level}
}

func processErrorLog(db *sql.DB, username, domain, logPath, prefLevel string) {
file, err := os.Open(logPath)
if err != nil {
return
}
defer file.Close()

var alerts []string
scanner := bufio.NewScanner(file)
for scanner.Scan() {
line := scanner.Text()
lowerLine := strings.ToLower(line)

isError := strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "fatal") || strings.Contains(lowerLine, "critical")
isWarn := strings.Contains(lowerLine, "warn")

if prefLevel == "all" || prefLevel == "both" {
if isError || isWarn {
alerts = append(alerts, fmt.Sprintf("[%s] %s", domain, line))
}
} else if prefLevel == "errors" {
if isError {
alerts = append(alerts, fmt.Sprintf("[%s] %s", domain, line))
}
} else if prefLevel == "warnings" {
if isWarn {
alerts = append(alerts, fmt.Sprintf("[%s] %s", domain, line))
}
}
}

if len(alerts) > 0 {
storeAlerts(db, username, alerts)
}
}

func storeAlerts(db *sql.DB, username string, alerts []string) {
_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS user_alerts (
id INTEGER PRIMARY KEY AUTOINCREMENT,
username TEXT NOT NULL,
message TEXT NOT NULL,
created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
notified BOOLEAN DEFAULT 0
)`)

for _, alert := range alerts {
// Dedup or only insert new... using REPLACE INTO logic or checking state in prod
db.Exec("INSERT INTO user_alerts (username, message) VALUES (?, ?)", username, alert)
}
log.Printf("[Watchdog] Generated %d alerts for user %s", len(alerts), username)
}
