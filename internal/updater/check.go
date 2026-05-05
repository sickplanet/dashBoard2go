package updater

import (
"database/sql"
"encoding/json"
"fmt"
"io/ioutil"
"net/http"
"strings"
"time"
)

type GitHubRelease struct {
TagName string `json:"tag_name"`
}

// Generate an Update check. TODO: Schedule this via Watchdog or Core cron logic.
func CheckForUpdates(db *sql.DB, currentVersion string) error {
client := http.Client{Timeout: 5 * time.Second}
resp, err := client.Get("https://api.github.com/repos/sickplanet/dashBoard2go/releases/latest")
if err != nil {
return err
}
defer resp.Body.Close()

if resp.StatusCode != 200 {
return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
}

body, err := ioutil.ReadAll(resp.Body)
if err != nil {
return err
}

var release GitHubRelease
if err := json.Unmarshal(body, &release); err != nil {
return err
}

latestVersion := strings.TrimPrefix(release.TagName, "v")
currentVersion = strings.TrimPrefix(currentVersion, "v") // Assuming semantic version string from VERSION file

if latestVersion != "" && latestVersion != currentVersion {
// New update available!
_, err = db.Exec("INSERT INTO system_settings (key, value) VALUES ('update_available', ?) ON CONFLICT(key) DO UPDATE SET value = ?", latestVersion, latestVersion)
if err != nil {
return fmt.Errorf("Failed to write update flag to DB: %v", err)
}
} else {
// Clear flag if up to date
_, _ = db.Exec("INSERT INTO system_settings (key, value) VALUES ('update_available', '') ON CONFLICT(key) DO UPDATE SET value = ''")
}

return nil
}
