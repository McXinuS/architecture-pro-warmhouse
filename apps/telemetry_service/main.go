package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := NewStore(ctx, getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/telemetry"))
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer store.Close()

	consumer := NewConsumer(getEnv("RABBITMQ_URL", "amqp://warmhouse:warmhouse@localhost:5672/"), store)
	go consumer.Run(ctx)

	srv := &http.Server{
		Addr:    ":" + getEnv("PORT", "8083"),
		Handler: NewHandler(store).Routes(),
	}
	go func() {
		log.Printf("Telemetry service listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
