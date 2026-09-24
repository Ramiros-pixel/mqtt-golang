package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"smarthome/backend/internal"
)

func main() {
	cfg := LoadConfig()

	db, err := internal.InitDB(cfg.MySQLDSN())
	if err != nil {
		log.Fatalf("[db] gagal koneksi MySQL: %v", err)
	}
	log.Println("[db] terhubung ke MySQL, migrasi & seeder selesai")

	hub := internal.NewHub()
	go hub.Run()

	bridge := internal.NewBridge(internal.MQTTCfg{
		URL:          cfg.MQTTURL(),
		User:         cfg.MQTTUser,
		Password:     cfg.MQTTPass,
		TopicMon:     cfg.TopicMon,
		TopicCtrl:    cfg.TopicCtrl,
		TopicShading: cfg.TopicShading,
	}, db, hub)
	bridge.Start()

	router := NewRouter(cfg, db, hub, bridge)
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("[http] backend jalan di http://localhost:%d", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[http] %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("[shutdown] menghentikan server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	hub.Stop()
	bridge.Stop()
}