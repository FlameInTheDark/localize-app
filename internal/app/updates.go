package app

import (
	"context"
	"log"
	"time"

	"github.com/FlameInTheDark/localize-app/internal/updatecheck"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	localizeReleasesURL = "https://github.com/FlameInTheDark/localize-app/releases"
	updateCheckInterval = time.Hour
	updateCheckTimeout  = 10 * time.Second
)

type updateChecker interface {
	Check(context.Context) (updatecheck.Release, bool, error)
}

func (d *Desktop) startUpdateChecks() {
	if d.updates == nil || d.ctx == nil {
		return
	}
	ctx, cancel := context.WithCancel(d.ctx)
	d.updateMu.Lock()
	previous := d.updateCancel
	d.updateCancel = cancel
	d.updateMu.Unlock()
	if previous != nil {
		previous()
	}
	go d.runUpdateChecks(ctx)
}

func (d *Desktop) stopUpdateChecks() {
	d.updateMu.Lock()
	cancel := d.updateCancel
	d.updateCancel = nil
	d.updateMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (d *Desktop) runUpdateChecks(ctx context.Context) {
	d.checkForUpdate(ctx)
	ticker := time.NewTicker(updateCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.checkForUpdate(ctx)
		}
	}
}

func (d *Desktop) checkForUpdate(ctx context.Context) {
	operationCtx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
	release, available, err := d.updates.Check(operationCtx)
	cancel()
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("check for Localize update: %v", err)
		}
		return
	}
	if !available {
		return
	}

	update := UpdateAvailability{Available: true, Version: release.Version, URL: release.URL}
	d.updateMu.Lock()
	changed := d.update != update
	d.update = update
	d.updateMu.Unlock()
	if changed {
		wailsruntime.EventsEmit(d.context(), "update:available", update)
	}
}
