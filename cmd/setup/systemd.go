package main

import (
	"fmt"
	"os"
	"os/exec"
)

func installSystemdServices() {
	fmt.Println("Installing Systemd Services...")

	cwd, _ := os.Getwd()

	services := map[string]string{
		"dashboard2go-core": `[Unit]
Description=dashBoard2go Core API & Web Panel
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=` + cwd + `
ExecStart=` + cwd + `/dashboard2go-core
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`,
		"dashboard2go-worker": `[Unit]
Description=dashBoard2go Background Worker Queue
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=` + cwd + `
ExecStart=` + cwd + `/dashboard2go-worker
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`,
		"dashboard2go-watchdog": `[Unit]
Description=dashBoard2go Log & Service Watchdog
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=` + cwd + `
ExecStart=` + cwd + `/dashboard2go-watchdog
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`,
	}

	for name, content := range services {
		filepath := "/etc/systemd/system/" + name + ".service"
		err := os.WriteFile(filepath, []byte(content), 0644)
		if err != nil {
			fmt.Printf("Warning: Could not write %s: %v\n", filepath, err)
		}
	}

	// Reload & Enable
	exec.Command("systemctl", "daemon-reload").Run()
	for name := range services {
		exec.Command("systemctl", "enable", name).Run()
		exec.Command("systemctl", "start", name).Run()
	}
	fmt.Println("Systemd Services Generated, Enabled, and Started.")
}
