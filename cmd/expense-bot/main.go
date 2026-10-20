package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"example.com/ai-expense-bot/internal/config"
	"example.com/ai-expense-bot/internal/dialog"
	"example.com/ai-expense-bot/internal/llm"
	"example.com/ai-expense-bot/internal/store"
	"example.com/ai-expense-bot/internal/telegram"
	"github.com/go-chi/chi/v5"
	"github.com/go-resty/resty/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	health := flag.Bool("healthcheck", false, "Check local readiness")
	flag.Parse()
	if *health {
		c := resty.New().SetTimeout(3 * time.Second).SetRetryCount(0)
		res, err := c.R().Get("http://127.0.0.1:8080/readyz")
		if err != nil {
			os.Exit(1)
		}
		if res.StatusCode() != 200 {
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 20*time.Second)
	pool, err := pgxpool.New(startup, cfg.DatabaseURL)
	if err != nil {
		cancel()
		return errors.New("invalid database configuration")
	}
	defer pool.Close()
	if err = pool.Ping(startup); err != nil {
		cancel()
		return errors.New("database unavailable")
	}
	repo := &store.Store{Pool: pool}
	err = repo.Migrate(startup)
	cancel()
	if err != nil {
		return errors.New("database migration failed")
	}
	model := &llm.Client{HTTP: resty.New().SetTimeout(cfg.LLMTimeout).SetRetryCount(0), Endpoint: cfg.LLMEndpoint, Key: cfg.LLMKey, Model: cfg.LLMModel}
	service := &dialog.Service{Store: repo, LLM: model, Location: cfg.Location}
	bot := &telegram.Bot{Token: cfg.TelegramToken, HTTP: resty.New().SetTimeout(40 * time.Second).SetRetryCount(0), Handler: service}
	mux := chi.NewRouter()
	mux.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if pool.Ping(ctx) != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		w.WriteHeader(200)
	})
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	botDone := make(chan struct{})
	go func() { defer close(botDone); bot.Run(ctx) }()
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	slog.Info("application started", "address", cfg.HTTPAddr)
	select {
	case <-ctx.Done():
	case err = <-serverErrors:
	}
	stop()
	shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	_ = server.Shutdown(shutdown)
	<-botDone
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
