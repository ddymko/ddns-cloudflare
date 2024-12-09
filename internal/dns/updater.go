package dns

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cloudflare/cloudflare-go"
)

type Updater struct {
	api    *cloudflare.API
	zoneID string
	ipChan <-chan string
	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger
}

func NewUpdater(ctx context.Context, api *cloudflare.API, zoneID string, ipChan chan string, logger *slog.Logger) *Updater {
	ctx, cancel := context.WithCancel(ctx)
	logger = logger.With("component", "dns-updater")

	return &Updater{
		api:    api,
		zoneID: zoneID,
		ipChan: ipChan,
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}
}

func (u *Updater) Start(wg *sync.WaitGroup) {
	u.logger.Info("starting")
	go func() {
		zoneID, err := u.api.ZoneIDByName(u.zoneID)
		if err != nil {
			panic(err)
		}
		defer wg.Done()
		for {
			select {
			case ip := <-u.ipChan:

				records, _, err := u.api.ListDNSRecords(u.ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{Type: "A"})
				if err != nil {
					u.logger.Error(fmt.Sprintf("error retrieving dns records: %v", err))
					continue
				}

				for _, v := range records {
					_, err := u.api.UpdateDNSRecord(u.ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.UpdateDNSRecordParams{
						Content: ip,
						ID:      v.ID,
					})
					if err != nil {
						u.logger.Error(fmt.Sprintf("error updating IP on dns record %s: %v", v.ID, err))
					}
				}

			case <-u.ctx.Done():
				u.logger.Info("dns updater: closing go routine")
				return
			}
		}
	}()
}
