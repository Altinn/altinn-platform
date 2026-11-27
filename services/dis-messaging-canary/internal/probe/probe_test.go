package probe

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var errBroken = errors.New("broken")

// fakeMessenger is a queue in memory.
type fakeMessenger struct {
	queue                            []string // bodies that wait in the queue
	sent                             []string // every body that Send got
	locked                           int      // messages that are received and not completed
	sendErr, receiveErr, completeErr error
	sendDelay                        time.Duration
	dropSent                         bool // a sent message never arrives
}

func (f *fakeMessenger) Send(_ context.Context, body []byte) error {
	time.Sleep(f.sendDelay)
	f.sent = append(f.sent, string(body))
	if f.sendErr == nil && !f.dropSent {
		f.queue = append(f.queue, string(body))
	}
	return f.sendErr
}

func (f *fakeMessenger) Receive(ctx context.Context) (Message, error) {
	if f.receiveErr != nil {
		return Message{}, f.receiveErr
	}
	if len(f.queue) == 0 {
		<-ctx.Done()
		return Message{}, ctx.Err()
	}
	msg := Message{Body: []byte(f.queue[0])}
	f.queue = f.queue[1:]
	f.locked++
	return msg, nil
}

func (f *fakeMessenger) Complete(context.Context, Message) error {
	if f.completeErr == nil {
		f.locked--
	}
	return f.completeErr
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		messenger fakeMessenger
		wantErr   error
		wantStage string // the stage that fails; empty when the round trip succeeds
		wantStale float64
	}{
		{name: "success"},
		{name: "send fails", messenger: fakeMessenger{sendErr: errBroken}, wantErr: errBroken, wantStage: stageSend},
		{name: "receive fails", messenger: fakeMessenger{receiveErr: errBroken}, wantErr: errBroken, wantStage: stageReceive},
		{name: "complete fails", messenger: fakeMessenger{completeErr: errBroken}, wantErr: errBroken, wantStage: stageComplete},
		{name: "timeout", messenger: fakeMessenger{dropSent: true}, wantErr: context.DeadlineExceeded, wantStage: stageReceive},
		{name: "send is slower than the timeout", messenger: fakeMessenger{sendDelay: 25 * time.Second}, wantErr: context.DeadlineExceeded, wantStage: stageSend},
		{name: "stale message", messenger: fakeMessenger{queue: []string{"left-over-from-team-a.orders"}}, wantStale: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				reg := prometheus.NewRegistry()
				p := New(&tt.messenger, NewMetrics(reg), 30*time.Second, 20*time.Second)
				want := zero()
				want[roundTripsName], want[staleName] = 1, tt.wantStale
				if tt.wantStage == "" {
					want[durationName], want[lastSuccessName] = 1, float64(time.Now().Unix())
				} else {
					want[failures(tt.wantStage)] = 1
				}
				if err := p.roundTrip(t.Context()); !errors.Is(err, tt.wantErr) {
					t.Errorf("error: got %v, want %v", err, tt.wantErr)
				}
				if got := values(t, reg); !maps.Equal(got, want) {
					t.Errorf("metrics:\n got %v\nwant %v", got, want)
				}
				if left := len(tt.messenger.queue) + tt.messenger.locked; tt.wantStage == "" && left != 0 {
					t.Errorf("messages left in the queue after a success: got %d, want 0", left)
				}
			})
		})
	}
}

// The message never arrives, so every round trip ends at its timeout of 20 s.
// Round trips start at 0, 30, 60 and 90 s. The canary stops at 100 s: the last
// round trip is cut off and must not be recorded. Every round trip sends its
// own ID.
func TestRunStartsOneRoundTripPerInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reg := prometheus.NewRegistry()
		messenger := &fakeMessenger{dropSent: true}
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Second)
		defer cancel()
		New(messenger, NewMetrics(reg), 30*time.Second, 20*time.Second).Run(ctx)
		want := zero()
		want[roundTripsName], want[failures(stageReceive)] = 3, 3
		if got := values(t, reg); !maps.Equal(got, want) {
			t.Errorf("metrics:\n got %v\nwant %v", got, want)
		}
		slices.Sort(messenger.sent)
		if ids := slices.Compact(messenger.sent); len(ids) != 4 {
			t.Errorf("IDs of 4 round trips: got %d different IDs, want 4", len(ids))
		}
	})
}
