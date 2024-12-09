package ip

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	http2 "net/http"
	"os"
	"sync"
	"time"

	"github.com/devbytes-cloud/dynamic-cf/internal/http"
	"github.com/mitchellh/go-homedir"
)

const (
	ipFileName = "last-known-ip"

	ipEndpoint = "https://ifconfig.me/ip"

	// checkInterval = 24 * time.Hour
	checkInterval = 10 * time.Second
)

// Checker is responsible for periodically sending IP check signals through a channel.
type Checker struct {
	// ipChan to send IP check signals if the IP has changed
	ipChan chan<- string
	// ctx to manage the lifecycle of the Checker.
	ctx context.Context
	// cancel allows the context to be canceled
	cancel context.CancelFunc
	// logger used to log
	logger *slog.Logger
	// client is the HTTP client used to make requests to the IP endpoint.
	client http.ClientInterface
	// ipEndpoint is the URL of the endpoint used to retrieve the current IP address.
	filePath string
}

// NewChecker creates a new Checker instance. .
func NewChecker(ctx context.Context, ipChan chan string, logger *slog.Logger, client http.ClientInterface) (*Checker, error) {
	ctx, cancel := context.WithCancel(ctx)
	filePath, err := getIPFilePath()
	if err != nil {
		cancel()
		return nil, err
	}

	logger = logger.With("component", "ip-checker")
	return &Checker{
		ipChan:   ipChan,
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
		client:   client,
		filePath: filePath,
	}, nil
}

// Start begins the IP checking process.
// wg: A WaitGroup to signal when the Checker has stopped.
func (c *Checker) Start(wg *sync.WaitGroup) {
	c.logger.Info("ip checker: starting")
	ticker := time.NewTicker(checkInterval)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ticker.C:
				c.logger.Info("checking currently assigned IP")

				if err := c.checkIP(); err != nil {
					c.logger.Error(err.Error())
				}

			case <-c.ctx.Done():
				c.logger.Info("ip checker: closing")
				ticker.Stop()
				return
			}
		}
	}()
}

func (c *Checker) checkIP() error {
	ip, err := c.getIP()
	if err != nil {
		return fmt.Errorf("failed to query IP: %w", err)
	}

	changed, err := c.hasIPChanged(ip)
	if err != nil {
		return fmt.Errorf("failed to check IP Change: %w", err)
	}

	if changed {
		c.ipChan <- ip
	}

	return nil
}

// getIP retrieves the current IP address from the configured endpoint.
// It returns the IP address as a string and an error if any occurred during the request.
// If the response status code is not 200, it returns an error with the status code.
func (c *Checker) getIP() (string, error) {
	resp, err := c.client.Get(ipEndpoint)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http2.StatusOK {
		return "", fmt.Errorf("received status code %d", resp.StatusCode)
	}

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(ip), nil
}

func (c *Checker) hasIPChanged(ip string) (bool, error) {
	cachedIP, err := c.lastKnownIP()
	if err != nil {
		if os.IsNotExist(err) {
			// if it doesn't exist its fine we will consider this as a change
		} else {
			return false, err
		}
	}
	changed := string(cachedIP) != ip
	if changed {
		if err := c.cacheKnownIP(ip); err != nil {
			return true, err
		}
		c.logger.Info("IP changed detected",
			"old_ip", string(cachedIP),
			"new_ip", ip)
	}
	return changed, nil
}

func (c *Checker) cacheKnownIP(ip string) error {
	c.logger.Info(fmt.Sprintf("caching last known ip (%s) in %s", ip, c.filePath))
	return os.WriteFile(c.filePath, []byte(ip), 0o600)
}

func (c *Checker) lastKnownIP() ([]byte, error) {
	return os.ReadFile(c.filePath)
}

func getIPFilePath() (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return homedir.Expand(fmt.Sprintf("%s/%s", pwd, ipFileName))
}
