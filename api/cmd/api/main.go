package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"hypervane/internal/config"
	"hypervane/internal/crypto"
	"hypervane/internal/httpapi"
	"hypervane/internal/hub"
	"hypervane/internal/poller"
	"hypervane/internal/store"
)

var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	box, err := crypto.NewBox(cfg.MasterKey)
	if err != nil {
		log.Fatalf("crypto: %v", err)
	}

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis url: %v", err)
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer rdb.Close()

	h := hub.New(rdb)
	go h.Run(ctx)

	srv := httpapi.New(cfg, st, box, h, nil)
	p := poller.New(rdb, h, srv.Clients, st)
	srv.SetPoller(p) // IOPS series is owned by the poller.
	if err := srv.RestoreClient(ctx); err != nil {
		log.Printf("restore cluster client: %v", err)
	}
	go p.Run(ctx)

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("hypervane api %s listening on %s", version, cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, done := context.WithTimeout(context.Background(), 8*time.Second)
	defer done()
	_ = httpSrv.Shutdown(shutdownCtx)
}
