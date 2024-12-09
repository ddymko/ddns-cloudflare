package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/devbytes-cloud/ddns-cloudflare/internal/dns"
	"github.com/devbytes-cloud/ddns-cloudflare/internal/http"
	"github.com/devbytes-cloud/ddns-cloudflare/internal/ip"
)

const (
	cfToken  = ""
	zoneName = ""
)

func main() {
	logger := setupLogger()
	logger.Info("starting dynamic cloudflare")

	ipChan := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())

	httpClient := &http.Client{}

	ipChecker, err := ip.NewChecker(ctx, ipChan, logger, httpClient)
	if err != nil {
		logger.Error("ipChecker failed to initialize %w", err)
		os.Exit(1)

	}

	api, err := cloudflare.NewWithAPIToken(cfToken)
	if err != nil {
		logger.Error("cloudflare failed to initialize %w", err)
		os.Exit(1)
	}

	dnsUpdater := dns.NewUpdater(ctx, api, zoneName, ipChan, logger)

	var wg sync.WaitGroup
	wg.Add(2)

	ipChecker.Start(&wg)
	dnsUpdater.Start(&wg)

	shutdown := processShutdown(cancel, logger, &wg)
	if err := shutdown(); err != nil {
		logger.Error("shutdown error", "error", err)
	}
}

func setupLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: false,
	}))
}

func processShutdown(cancel context.CancelFunc, logger *slog.Logger, wg *sync.WaitGroup) func() error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	logger = logger.With("component", "shutdown")
	return func() error {
		rcvSignal := <-signals
		logger.Info(fmt.Sprintf("signal to close received: %s", rcvSignal.String()))
		cancel()

		shutdownCTX, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		for {
			select {
			case <-shutdownCTX.Done():
				return fmt.Errorf("shutdown timeout exceeded")
			case <-done:
				logger.Info("clean shutdown")
				return nil
			}
		}
	}
}
