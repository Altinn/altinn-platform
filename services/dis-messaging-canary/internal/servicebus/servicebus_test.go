package servicebus

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-messaging-canary/internal/probe"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// fakeLinks replaces the sender and the receiver of the SDK.
type fakeLinks struct {
	sent       []*azservicebus.Message
	sendErr    error
	closed     bool
	maxAsked   int                               // the highest maxMessages of a call of ReceiveMessages
	results    [][]*azservicebus.ReceivedMessage // one result per call of ReceiveMessages
	receiveErr error                             // the result when results is empty
	completed  []*azservicebus.ReceivedMessage
}

func (f *fakeLinks) SendMessage(_ context.Context, message *azservicebus.Message, _ *azservicebus.SendMessageOptions) error {
	f.sent = append(f.sent, message)
	return f.sendErr
}

func (f *fakeLinks) Close(context.Context) error {
	f.closed = true
	return errors.New("the link is closed already")
}

func (f *fakeLinks) ReceiveMessages(_ context.Context, maxMessages int, _ *azservicebus.ReceiveMessagesOptions) ([]*azservicebus.ReceivedMessage, error) {
	f.maxAsked = max(f.maxAsked, maxMessages)
	if len(f.results) == 0 {
		return nil, f.receiveErr
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result, nil
}

func (f *fakeLinks) CompleteMessage(_ context.Context, message *azservicebus.ReceivedMessage, _ *azservicebus.CompleteMessageOptions) error {
	f.completed = append(f.completed, message)
	return nil
}

type fakeCredential struct{ t *testing.T }

func (c fakeCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.t.Error("New asked for a token: it must not connect")
	return azcore.AccessToken{}, nil
}

// slowCredential answers after delay, or when the context ends. Like the real
// credential, its error then does not wrap the error of the context.
type slowCredential struct{ delay time.Duration }

func (c slowCredential) GetToken(ctx context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	select {
	case <-ctx.Done():
		return azcore.AccessToken{}, errors.New("unable to resolve an endpoint: " + ctx.Err().Error())
	case <-time.After(c.delay):
		return azcore.AccessToken{Token: "token-1"}, nil
	}
}

func TestTimedCredential(t *testing.T) {
	// The SDK asks for a token with a context that never ends.
	ctx := context.Background()
	token, err := timedCredential{slowCredential{0}, time.Hour}.GetToken(ctx, policy.TokenRequestOptions{})
	if err != nil || token.Token != "token-1" {
		t.Fatalf("a request in time: got %q, %v; want token-1", token.Token, err)
	}
	_, err = timedCredential{slowCredential{time.Hour}, time.Millisecond}.GetToken(ctx, policy.TokenRequestOptions{})
	if err == nil || !strings.HasPrefix(err.Error(), "the token request took longer than 1ms") {
		t.Fatalf("a request that hangs: got %v, want the time limit in the error", err)
	}
}

func TestSendReplacesTheSenderAfterAnError(t *testing.T) {
	unauthorized := &azservicebus.Error{Code: azservicebus.CodeUnauthorizedAccess}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	tests := []struct {
		name         string
		ctx          context.Context
		sendErr      error
		newSenderErr error
		wantReplaced bool
	}{
		{"an SDK error", t.Context(), unauthorized, nil, true},
		{"no error", t.Context(), nil, nil, false},
		{"the context ended", canceled, context.Canceled, nil, false},
		{"no new sender", t.Context(), unauthorized, errors.New("the client is closed"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old, fresh := &fakeLinks{sendErr: tt.sendErr}, &fakeLinks{}
			m := &Messenger{sender: old, newSender: func() (sender, error) { return fresh, tt.newSenderErr }}
			if err := m.Send(tt.ctx, []byte("id-1")); !errors.Is(err, tt.sendErr) {
				t.Fatalf("Send: got %v, want %v", err, tt.sendErr)
			}
			if replaced := m.sender == sender(fresh); replaced != tt.wantReplaced || old.closed != tt.wantReplaced {
				t.Fatalf("got replaced %v and old sender closed %v, want both %v", replaced, old.closed, tt.wantReplaced)
			}
			if err := m.Send(t.Context(), []byte("id-2")); tt.wantReplaced && (err != nil || len(fresh.sent) != 1) {
				t.Fatalf("Send after the replacement: got %v and %d messages on the new sender, want 1", err, len(fresh.sent))
			}
		})
	}
}

func TestNewDoesNotConnect(t *testing.T) {
	m, err := New("sb-team-a.servicebus.windows.net", "team-a.orders", fakeCredential{t})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := m.Close(t.Context()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestSendReceiveComplete(t *testing.T) {
	received := &azservicebus.ReceivedMessage{Body: []byte("id-1")}
	// The first two results are empty, as after a link error that the SDK
	// recovers from.
	links := &fakeLinks{results: [][]*azservicebus.ReceivedMessage{nil, {}, {received}}}
	m := &Messenger{sender: links, receiver: links}

	if err := m.Send(t.Context(), []byte("id-1")); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(links.sent) != 1 || string(links.sent[0].Body) != "id-1" {
		t.Fatalf("sent: got %v, want one message with the body id-1", links.sent)
	}
	if ttl := links.sent[0].TimeToLive; ttl == nil || *ttl != messageTTL {
		t.Fatalf("sent: got the time to live %v, want %s", ttl, messageTTL)
	}
	msg, err := m.Receive(t.Context())
	if err != nil || string(msg.Body) != "id-1" {
		t.Fatalf("Receive: got body %q, %v; want id-1", msg.Body, err)
	}
	// A second message in one call would stay locked, and nobody completes it.
	if links.maxAsked != 1 {
		t.Fatalf("Receive: asked the SDK for up to %d messages, want 1", links.maxAsked)
	}
	if err := m.Complete(t.Context(), msg); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(links.completed) != 1 || links.completed[0] != received {
		t.Fatalf("completed: got %v, want the received message", links.completed)
	}
}

func TestReceiveReturnsErrors(t *testing.T) {
	unauthorized := &azservicebus.Error{Code: azservicebus.CodeUnauthorizedAccess}
	tooMany := errors.New("Receive called the SDK again after the context ended")
	empty := [][]*azservicebus.ReceivedMessage{nil}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	tests := []struct {
		name  string
		ctx   context.Context
		links *fakeLinks
		want  error
	}{
		{"an SDK error is returned unchanged", t.Context(), &fakeLinks{receiveErr: unauthorized}, unauthorized},
		{"an SDK error after an empty result", t.Context(), &fakeLinks{results: empty, receiveErr: unauthorized}, unauthorized},
		{"the context ends in the SDK call", canceled, &fakeLinks{receiveErr: context.Canceled}, context.Canceled},
		{"the context ends after an empty result", canceled, &fakeLinks{results: empty, receiveErr: tooMany}, context.Canceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Messenger{receiver: tt.links}
			if tt.ctx.Err() != nil {
				m.emptyPause = time.Hour // only the context can end the wait
			}
			if _, err := m.Receive(tt.ctx); !errors.Is(err, tt.want) {
				t.Fatalf("Receive: got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCompleteRejectsAMessageThatIsNotFromReceive(t *testing.T) {
	links := &fakeLinks{}
	if err := (&Messenger{receiver: links}).Complete(t.Context(), probe.Message{Body: []byte("id-1")}); err == nil {
		t.Fatal("Complete: got no error, want an error")
	}
	if len(links.completed) != 0 {
		t.Fatalf("completed: got %v, want nothing", links.completed)
	}
}
