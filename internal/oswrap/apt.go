package oswrap

import (
	"log"
	"os/exec"
)

// AptInstall runs an apt-get install command silently (unless there is an error)
func AptInstall(packages ...string) error {
	log.Printf("Installing packages via APT: %v\n", packages)

	args := append([]string{"install", "-y"}, packages...)
	cmd := exec.Command("apt-get", args...)

	// Output to stdout/err if we want to show interactive progress
	// For now we'll capture it or pipe it to the console
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	return cmd.Run()
}

// AptUpdate runs apt-get update
func AptUpdate() error {
	log.Println("Updating APT repositories...")
	cmd := exec.Command("apt-get", "update", "-y")
	return cmd.Run()
}

// SystemctlEnableAndStart enables and starts a systemd service
func SystemctlEnableAndStart(serviceName string) error {
	log.Printf("Enabling and starting service: %s\n", serviceName)
	cmd1 := exec.Command("systemctl", "enable", serviceName)
	if err := cmd1.Run(); err != nil {
		return err
	}

	cmd2 := exec.Command("systemctl", "start", serviceName)
	return cmd2.Run()
}
