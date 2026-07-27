package cloud_account_management

import (
	"context"
	"sync"
	"time"
)

const (
	GCPServiceUsageMutationsPerMinute = 90
	GCPServiceUsageMutationInterval   = time.Minute / GCPServiceUsageMutationsPerMinute
)

type ServiceUsageMutationPacer struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func NewServiceUsageMutationPacer(interval time.Duration) *ServiceUsageMutationPacer {
	return &ServiceUsageMutationPacer{interval: interval}
}

func (p *ServiceUsageMutationPacer) Wait(ctx context.Context) error {
	for {
		delay := p.waitDuration(time.Now())
		if delay <= 0 {
			return nil
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (p *ServiceUsageMutationPacer) SetCooldown(cooldown time.Duration) {
	p.setCooldownAt(time.Now(), cooldown)
}

func (p *ServiceUsageMutationPacer) waitDuration(now time.Time) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.next.After(now) {
		return p.next.Sub(now)
	}

	p.next = now.Add(p.interval)
	return 0
}

func (p *ServiceUsageMutationPacer) setCooldownAt(now time.Time, cooldown time.Duration) {
	if cooldown <= 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cooldownUntil := now.Add(cooldown)
	if cooldownUntil.After(p.next) {
		p.next = cooldownUntil
	}
}

var GCPServiceUsageMutationPacer = NewServiceUsageMutationPacer(GCPServiceUsageMutationInterval)
