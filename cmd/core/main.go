package main

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"dashBoard2go/internal/api"
	"dashBoard2go/internal/config"
	"dashBoard2go/internal/db/migrations"
	"dashBoard2go/internal/queue"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	log.Println("Starting dashBoard2go Core API Server...")

	conf, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("FAILED: Could not load config.json. Have you run the setup? Error: %v", err)
	}

	if !conf.Installed {
		log.Fatalf("FAILED: Panel is not fully installed. Please run dashboard2go-setup.")
	}

	dbURI := fmt.Sprintf("%s?_journal_mode=WAL", conf.SQLitePath)
	db, err := sql.Open("sqlite3", dbURI)
	if err != nil {
		log.Fatalf("FAILED to open SQLite database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("FAILED to connect to SQLite database: %v", err)
	}
	log.Println("SQLite Database connected successfully (WAL Mode).")

	if err := migrations.Migrate(db); err != nil {
		log.Fatalf("FAILED to run SQLite migrations: %v", err)
	}

	log.Println("Checking and running DB migrations...")
	if err := migrations.Migrate(db); err != nil {
		log.Fatalf("FAILED to run SQLite migrations: %v", err)
	}

	q, err := queue.NewSQLiteQueue(db)
	if err != nil {
		log.Fatalf("FAILED to init queue: %v", err)
	}

	// Initialize the HTTP Router before setting up routes
	r := gin.Default()

	api.SetupRoutes(r, db, q)

	r.Static("/admin", "./web/admin")
	r.Static("/user", "./web/user")
	r.Static("/shared", "./web/shared")

	httpAddr := fmt.Sprintf(":%d", conf.PanelPortHTTP)
	httpsAddr := fmt.Sprintf(":%d", conf.PanelPortHTTPS)

	if conf.UseLetsEncryptFQDN && conf.FQDN != "" {
		certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", conf.FQDN)
		keyPath := fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", conf.FQDN)

		log.Printf("Booting HTTPS Core Server on %s (FQDN: %s)\n", httpsAddr, conf.FQDN)

		go func() {
			log.Printf("Booting HTTP Proxy Listener on %s\n", httpAddr)
			if err := r.Run(httpAddr); err != nil {
				log.Printf("HTTP Server failed: %v", err)
			}
		}()

		tlsConfig := &tls.Config{
			GetCertificate: func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				serverName := clientHello.ServerName
				if serverName == "" {
					serverName = conf.FQDN
				}

				userCertFile := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", serverName)
				userKeyFile := fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", serverName)

				if _, err := os.Stat(userCertFile); os.IsNotExist(err) {
					userCertFile = certPath
					userKeyFile = keyPath
				}

				cert, err := tls.LoadX509KeyPair(userCertFile, userKeyFile)
				if err != nil {
					return nil, err
				}
				return &cert, nil
			},
		}

		server := &http.Server{
			Addr:      httpsAddr,
			Handler:   r,
			TLSConfig: tlsConfig,
		}

		log.Printf("Booting SNI-Aware HTTPS Core Server on %s\n", httpsAddr)
		if err := server.ListenAndServeTLS("", ""); err != nil {
			log.Fatalf("Failed to run SNI TLS server: %v", err)
		}
	} else {
		log.Printf("Booting HTTP Core Server on %s (SSL Disabled)\n", httpAddr)
		if err := r.Run(httpAddr); err != nil {
			log.Fatalf("Failed to run core server: %v", err)
		}
	}
}
