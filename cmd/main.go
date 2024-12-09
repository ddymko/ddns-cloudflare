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
	"github.com/devbytes-cloud/dynamic-cf/internal/dns"
	"github.com/devbytes-cloud/dynamic-cf/internal/http"
	"github.com/devbytes-cloud/dynamic-cf/internal/ip"
)

const cfToken = ""

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

	dnsUpdater := dns.NewUpdater(ctx, api, "dymko.cloud", ipChan, logger)

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

//func main() {
//	// todo add slogger
//	// todo add configmap
//	// todo add writer to write IP to disk
//	fmt.Println("system booting")
//	api, err := cloudflare.NewWithAPIToken(cfToken)
//	if err != nil {
//		panic(err)
//	}
//
//	ctx, cancel := context.WithCancel(context.Background())
//	signals := make(chan os.Signal, 1)
//	ipChange := make(chan string, 1)
//	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
//
//	// start a goroutine that checks the IP every so often
//	ticker := time.NewTicker(10 * time.Second)
//	defer ticker.Stop()
//
//	zoneID, err := api.ZoneIDByName("dymko.cloud")
//	if err != nil {
//		panic(err)
//	}
//
//	go func() {
//		for {
//			select {
//			case <-ticker.C:
//				fmt.Println("ticket on this")
//
//				rnd := rand.Intn(10)
//				fmt.Println("random number", rnd)
//				if rnd%2 == 0 {
//					fmt.Println("ok we should now send this")
//					ipChange <- "192.168.1.1"
//				}
//
//			case <-ctx.Done():
//				fmt.Println("closing ip check routine")
//				return
//			}
//		}
//	}()
//
//	fmt.Println("go routine starting...")
//	go func() {
//		for {
//			select {
//			case ip := <-ipChange:
//				records, _, err := api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{Type: "A"})
//				if err != nil {
//					fmt.Println(err)
//					continue
//				}
//
//				for _, v := range records {
//
//					if v.Type != "A" {
//						fmt.Println("Record Type is not 'A'")
//						continue
//					}
//
//					if v.Content == ip {
//						fmt.Println("IP is the same")
//						continue
//					}
//
//					fmt.Println("we in here")
//					//recordResponse, err := api.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.UpdateDNSRecordParams{
//					//	Content: "68.199.54.95",
//					//	ID:      v.ID,
//					//})
//					//if err != nil {
//					//	fmt.Println(err)
//					//}
//					//
//					//fmt.Println(recordResponse)
//
//					// fmt.Println(v)
//				}
//
//			case <-ctx.Done():
//				fmt.Println("closing up cloudflare routine")
//				return
//			}
//		}
//	}()
//
//	sig := <-signals
//	fmt.Println(fmt.Sprintf("signal got %v", sig))
//	cancel()
//	time.Sleep(5 * time.Second)
//}
