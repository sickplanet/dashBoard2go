package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"dashBoard2go/internal/config"
	"dashBoard2go/internal/queue"
	"dashBoard2go/internal/wrappers/ftp"
	"dashBoard2go/internal/wrappers/mail"
	"dashBoard2go/internal/wrappers/webserver"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	log.Println("Starting dashBoard2go Worker Service...")

	conf, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("[Worker] Failed to load config.json (is the panel installed?): %v", err)
	}

	dbPath := conf.SQLitePath + "?_journal_mode=WAL"
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("[Worker] Failed to open database: %v", err)
	}
	defer db.Close()

	q, err := queue.NewSQLiteQueue(db)
	if err != nil {
		log.Fatalf("[Worker] Failed to init queue: %v", err)
	}

	log.Println("[Worker] Listening for deferred tasks in the SQLite Job Queue...")

	pollInterval := 5 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			job, err := q.Pop(ctx)
			cancel()

			if err != nil {
				if err != sql.ErrNoRows {
					log.Printf("[Worker] Queue Error: %v\n", err)
				}
				continue
			}

			if job != nil {
				log.Printf("[Worker] Found Job %d: %s. Executing...\n", job.ID, job.Action)

				err := executeJob(db, job)

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err != nil {
					log.Printf("[Worker] Job %d Failed: %v\n", job.ID, err)
					q.MarkFailed(ctx, job.ID, err.Error())
				} else {
					log.Printf("[Worker] Job %d Completed successfully.\n", job.ID)
					q.MarkComplete(ctx, job.ID)
				}
				cancel()
			}
		}
	}
}

func executeJob(db *sql.DB, job *queue.Job) error {
	ctx := context.Background()

	switch job.Action {
	case "ADD_FTP_USER":
		var p struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Quota    int    `json:"quota"`
			Dir      string `json:"dir"`
		}
		if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
			return fmt.Errorf("invalid payload: %v", err)
		}
		ftpWrapper := ftp.NewPureFTPdWrapper("vmail", "vmail")
		user := &ftp.FTPUser{
			Username: p.Username,
			Password: p.Password,
			QuotaMB:  p.Quota,
			HomeDir:  p.Dir,
		}
		return ftpWrapper.AddUser(ctx, user)

	case "CREATE_VHOST":
		var p struct {
			Domain   string `json:"domain"`
			Engine   string `json:"engine"`
			Username string `json:"username"`
		}
		if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
			return fmt.Errorf("invalid payload: %v", err)
		}

		vhostConfig := webserver.VhostConfig{
			Domain:       p.Domain,
			DocumentRoot: fmt.Sprintf("/home/dashboard2go/users/%s/web/%s/public_html", p.Username, p.Domain),
		}

		if p.Engine == "nginx" {
			w := webserver.NewNginxWrapper()
			err := w.CreateVhost(vhostConfig)
			if err == nil {
				w.EnableVhost(p.Domain)
				w.Reload()
			}
			return err
		} else {
			w := webserver.NewApacheWrapper()
			err := w.CreateVhost(vhostConfig)
			if err == nil {
				w.EnableVhost(p.Domain)
				w.Reload()
			}
			return err
		}

	case "ADD_MAIL_DOMAIN":
		var p struct {
			Domain string `json:"domain"`
		}
		if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
			return err
		}
		w := mail.NewPostfixDovecotWrapper(db)
		return w.CreateMailDomain(p.Domain)

	default:
		return fmt.Errorf("unknown action: %s", job.Action)
	}
}
