package poller

import (
	"context"
	"fmt"
	"log"
	"time"

	"tezos-delegation-service/internal/store"
	"tezos-delegation-service/internal/tzkt"
)

type Config struct {
	Store        store.DelegationStore
	Client       tzkt.Client
	BatchSize    int
	PollInterval time.Duration
	GenesisStart time.Time
	MaxBackoff   time.Duration
	Logger       *log.Logger
}

type Poller struct {
	cfg Config
}

func NewPoller(cfg Config) *Poller {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 10000
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 15 * time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 2 * time.Minute
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	return &Poller{cfg: cfg}
}

func (p *Poller) Run(ctx context.Context) error {
	backoff := p.cfg.PollInterval

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		n, err := p.syncOnce(ctx)
		if err != nil {
			p.cfg.Logger.Printf("poller error: %v", err)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil
			}
			if backoff < p.cfg.MaxBackoff {
				backoff *= 2
				if backoff > p.cfg.MaxBackoff {
					backoff = p.cfg.MaxBackoff
				}
			}
			continue
		}

		backoff = p.cfg.PollInterval

		if n == p.cfg.BatchSize {
			continue
		}

		select {
		case <-time.After(p.cfg.PollInterval):
		case <-ctx.Done():
			return nil
		}
	}
}

func (p *Poller) syncOnce(ctx context.Context) (int, error) {
	lastTs, _, err := p.cfg.Store.GetLastSeen(ctx)
	if err != nil {
		return 0, fmt.Errorf("get last seen: %w", err)
	}

	if lastTs.IsZero() || (!p.cfg.GenesisStart.IsZero() && lastTs.Before(p.cfg.GenesisStart)) {
		lastTs = p.cfg.GenesisStart
	}

	delegations, err := p.cfg.Client.FetchDelegations(ctx, lastTs, p.cfg.BatchSize)
	if err != nil {
		return 0, fmt.Errorf("fetch delegations since %s: %w", lastTs.UTC().Format(time.RFC3339), err)
	}
	if len(delegations) == 0 {
		return 0, nil
	}

	batch := make([]store.InsertDelegation, 0, len(delegations))
	for _, d := range delegations {
		if d.Sender.Address == "" {
			continue
		}
		batch = append(batch, store.InsertDelegation{
			TzktID:    d.ID,
			Timestamp: d.Timestamp,
			Amount:    d.Amount,
			Delegator: d.Sender.Address,
			Level:     d.Level,
		})
	}

	if err := p.cfg.Store.BulkInsert(ctx, batch); err != nil {
		return 0, fmt.Errorf("bulk insert %d delegations: %w", len(batch), err)
	}

	p.cfg.Logger.Printf("poller: inserted %d delegations since %s", len(batch), lastTs.UTC().Format(time.RFC3339))
	return len(batch), nil
}
