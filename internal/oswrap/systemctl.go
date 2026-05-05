package oswrap

import (
	"log"
	"os/exec"
	"strings"
)

// IsActive checks if a systemctl service is currently active and running.
func IsActive(service string) bool {
	cmd := exec.Command("systemctl", "is-active", service)
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	status := strings.TrimSpace(string(out))
	return status == "active"
}

// RestartService forcefully restarts a given systemctl service.
func RestartService(service string) error {
	log.Printf("[OS] Restarting service: %s\n", service)
	cmd := exec.Command("systemctl", "restart", service)
	return cmd.Run()
}

// GetServiceStatus checks the status of a specific service and returns a standardized map
func GetServiceStatus(service string) map[string]interface{} {
	active := IsActive(service)
	statusText := "offline"
	if active {
		statusText = "online"
	}

	return map[string]interface{}{
		"service": service,
		"active":  active,
		"status":  statusText,
	}
}
