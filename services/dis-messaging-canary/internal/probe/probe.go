package probe

import (
	"cmp"
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"time"
)

// Message is one received message.
type Message struct {
	Body []byte
	// Handle is what the Messenger needs to complete the message. The probe
	// does not read it.
	Handle any
}

// Messenger is the queue client that the probe needs. Every method must return
// when ctx ends: the probe cannot stop a call that hangs.
type Messenger interface {
	// Send puts one message with this body on the queue.
	Send(ctx context.Context, body []byte) error
	// Receive waits until one message arrives and returns it. When ctx ends
	// first, it returns the error of ctx.
	Receive(ctx context.Context) (Message, error)
	// Complete removes a received message from the queue.
	Complete(ctx context.Context, msg Message) error
}

// Prober does round trips and records them in the metrics.
type Prober struct {
	messenger Messenger
	metrics   *Metrics
	interval  time.Duration
	timeout   time.Duration
}

// New returns a Prober that starts one round trip per interval and gives each
// round trip at most timeout.
func New(messenger Messenger, metrics *Metrics, interval, timeout time.Duration) *Prober {
	return &Prober{messenger: messenger, metrics: metrics, interval: interval, timeout: timeout}
}

// Run does one round trip now and then one per interval, until ctx ends.
func (p *Prober) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if err := p.roundTrip(ctx); err != nil && ctx.Err() == nil {
			slog.Error("round trip failed", "error", err)
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
}

// roundTrip does one round trip and records the result. A round trip that is
// cut off because ctx ended is not recorded: the canary stops, the queue did
// not fail.
func (p *Prober) roundTrip(ctx context.Context) error {
	start := time.Now()
	stage, err := p.exchange(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	p.metrics.roundTrips.Inc()
	if err != nil {
		p.metrics.failures.WithLabelValues(stage).Inc()
		return fmt.Errorf("%s: %w", stage, err)
	}
	p.metrics.duration.Observe(time.Since(start).Seconds())
	p.metrics.lastSuccess.SetToCurrentTime()
	return nil
}

// exchange sends a message with a unique ID and receives until that message
// arrives. It returns the stage that failed and the error. A call that returns
// after the timeout fails, also when it returns no error.
//
// A message with another ID is not from this round trip. Usually it is left
// over from an earlier one: exchange completes it and counts it as stale. This
// needs one canary per queue. A second canary would take these messages.
func (p *Prober) exchange(parent context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(parent, p.timeout)
	defer cancel()
	id := rand.Text()
	if err := cmp.Or(p.messenger.Send(ctx, []byte(id)), ctx.Err()); err != nil {
		return stageSend, err
	}
	for {
		msg, err := p.messenger.Receive(ctx)
		if err = cmp.Or(err, ctx.Err()); err != nil {
			return stageReceive, err
		}
		if err := cmp.Or(p.messenger.Complete(ctx, msg), ctx.Err()); err != nil {
			return stageComplete, err
		}
		if string(msg.Body) == id {
			return "", nil
		}
		p.metrics.stale.Inc()
	}
}
