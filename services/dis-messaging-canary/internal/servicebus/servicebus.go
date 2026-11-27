// Package servicebus connects the probe to one queue in an Azure Service Bus
// namespace.
package servicebus

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-messaging-canary/internal/probe"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// sender and receiver are the parts of the SDK that Messenger uses. The tests
// replace them.
type sender interface {
	SendMessage(ctx context.Context, message *azservicebus.Message, options *azservicebus.SendMessageOptions) error
	Close(ctx context.Context) error
}

type receiver interface {
	ReceiveMessages(ctx context.Context, maxMessages int, options *azservicebus.ReceiveMessagesOptions) ([]*azservicebus.ReceivedMessage, error)
	CompleteMessage(ctx context.Context, message *azservicebus.ReceivedMessage, options *azservicebus.CompleteMessageOptions) error
}

// messageTTL is the time that a message of the canary can stay in the queue.
// When send works and receive does not, the queue would grow by one message per
// round trip. With this limit the left-over messages go away by themselves.
const messageTTL = 10 * time.Minute

// tokenTimeout is the longest time for one token request, with the retries of
// the identity library. When the limit cuts off its last retry, the next round
// trip asks again.
const tokenTimeout = 10 * time.Second

// timedCredential gives every token request a time limit, and logs every token
// error. The SDK asks for a token with a context that never ends. Without the
// limit, a token endpoint that does not answer blocks a call for much longer
// than its own context. Most token requests come from the claim refresh that the
// SDK runs in the background. That refresh stops for good after a token error
// and logs nothing, so this log line is the only trace of it.
type timedCredential struct {
	credential azcore.TokenCredential
	timeout    time.Duration
}

func (c timedCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	token, err := c.credential.GetToken(ctx, options)
	if err != nil && ctx.Err() != nil {
		err = fmt.Errorf("the token request took longer than %s: %w", c.timeout, err)
	}
	if err != nil {
		slog.Warn("the token request failed", "error", err, "took", time.Since(start).String())
	}
	return token, err
}

// Messenger sends to one queue and receives from it. It is for one caller at a
// time.
type Messenger struct {
	client *azservicebus.Client
	sender sender
	// newSender makes the sender that replaces sender after a failed send.
	newSender func() (sender, error)
	receiver  receiver
	// emptyPause is the wait after a receive call that returns no message.
	emptyPause time.Duration
}

var _ probe.Messenger = (*Messenger)(nil)

// New returns a Messenger for queue in the Azure Service Bus namespace with
// the host name endpoint. New does not connect: the first round trip does. So
// an Azure Service Bus namespace that is down shows as failed round trips, and
// the canary starts.
func New(endpoint, queue string, credential azcore.TokenCredential) (*Messenger, error) {
	// The SDK tries a failed call again, as a product app does. By default it
	// waits 4 s, 12 s and 28 s between the tries. That does not fit in one round
	// trip: the time is up in a wait, and the SDK then returns only "context
	// deadline exceeded" and drops the real error. With short waits a call that
	// fails fast returns its real error in time.
	options := &azservicebus.ClientOptions{RetryOptions: azservicebus.RetryOptions{
		MaxRetries: 2, RetryDelay: time.Second, MaxRetryDelay: 2 * time.Second,
	}}
	client, err := azservicebus.NewClient(endpoint, timedCredential{credential, tokenTimeout}, options)
	if err != nil {
		return nil, fmt.Errorf("create the Azure Service Bus client: %w", err)
	}
	newSender := func() (sender, error) {
		s, err := client.NewSender(queue, nil)
		if err != nil {
			return nil, fmt.Errorf("create the sender for queue %q: %w", queue, err)
		}
		return s, nil
	}
	s, err := newSender()
	if err != nil {
		return nil, err
	}
	r, err := client.NewReceiverForQueue(queue, &azservicebus.ReceiverOptions{ReceiveMode: azservicebus.ReceiveModePeekLock})
	if err != nil {
		return nil, fmt.Errorf("create the receiver for queue %q: %w", queue, err)
	}
	return &Messenger{client: client, sender: s, newSender: newSender, receiver: r, emptyPause: 100 * time.Millisecond}, nil
}

// Send puts one message with this body on the queue.
func (m *Messenger) Send(ctx context.Context, body []byte) error {
	ttl := messageTTL
	err := m.sender.SendMessage(ctx, &azservicebus.Message{Body: body, TimeToLive: &ttl}, nil)
	if err != nil && ctx.Err() == nil {
		m.replaceSender()
	}
	return err
}

// replaceSender makes a new sender after a send that failed. The broker can
// close the link of a sender with an error that the SDK does not recover from,
// for example when the queue is deleted and made again. That sender then
// returns the same error for every later send. A new sender opens a new link
// on the next send.
//
// The old sender closes in the background. Close waits for the claim refresh of
// the sender, and that wait does not obey a context: with a token request in
// progress it can take up to tokenTimeout. Until then the client still holds
// the old sender, so Close of the client at shutdown closes it too.
func (m *Messenger) replaceSender() {
	fresh, err := m.newSender()
	if err != nil {
		slog.Error("cannot make a new sender, the canary keeps the old one", "error", err)
		return
	}
	old := m.sender
	m.sender = fresh
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), tokenTimeout)
		defer cancel()
		// The link of the old sender is often closed already. An error of
		// Close says nothing new then.
		_ = old.Close(ctx)
	}()
}

// Receive waits until one message arrives and returns it with its lock. It
// returns every error of the SDK at once and unchanged, also when the login
// fails. When ctx ends first, it returns the error of ctx.
func (m *Messenger) Receive(ctx context.Context) (probe.Message, error) {
	for {
		messages, err := m.receiver.ReceiveMessages(ctx, 1, nil)
		if err != nil {
			return probe.Message{}, err
		}
		if len(messages) > 0 {
			return probe.Message{Body: messages[0].Body, Handle: messages[0]}, nil
		}
		// The SDK returns no message and no error after a link error that it
		// can recover from. Receive again after a short wait, so that a link
		// that fails again and again does not make a busy loop.
		select {
		case <-ctx.Done():
			return probe.Message{}, ctx.Err()
		case <-time.After(m.emptyPause):
		}
	}
}

// Complete removes a message that Receive returned from the queue.
func (m *Messenger) Complete(ctx context.Context, msg probe.Message) error {
	received, ok := msg.Handle.(*azservicebus.ReceivedMessage)
	if !ok {
		return fmt.Errorf("the message handle is a %T, not a message from Receive", msg.Handle)
	}
	return m.receiver.CompleteMessage(ctx, received, nil)
}

// Close closes the connection to the Azure Service Bus namespace.
func (m *Messenger) Close(ctx context.Context) error {
	return m.client.Close(ctx)
}
