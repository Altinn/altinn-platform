// Package servicebus connects the probe to one queue in an Azure Service Bus
// namespace.
package servicebus

import (
	"context"
	"fmt"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-messaging-canary/internal/probe"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// sender and receiver are the parts of the SDK that Messenger uses. The tests
// replace them.
type sender interface {
	SendMessage(ctx context.Context, message *azservicebus.Message, options *azservicebus.SendMessageOptions) error
}

type receiver interface {
	ReceiveMessages(ctx context.Context, maxMessages int, options *azservicebus.ReceiveMessagesOptions) ([]*azservicebus.ReceivedMessage, error)
	CompleteMessage(ctx context.Context, message *azservicebus.ReceivedMessage, options *azservicebus.CompleteMessageOptions) error
}

// messageTTL is the time that a message of the canary can stay in the queue.
// When send works and receive does not, the queue would grow by one message per
// round trip. With this limit the left-over messages go away by themselves.
const messageTTL = 10 * time.Minute

// Messenger sends to one queue and receives from it.
type Messenger struct {
	client   *azservicebus.Client
	sender   sender
	receiver receiver
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
	client, err := azservicebus.NewClient(endpoint, credential, options)
	if err != nil {
		return nil, fmt.Errorf("create the Azure Service Bus client: %w", err)
	}
	s, err := client.NewSender(queue, nil)
	if err != nil {
		return nil, fmt.Errorf("create the sender for queue %q: %w", queue, err)
	}
	r, err := client.NewReceiverForQueue(queue, &azservicebus.ReceiverOptions{ReceiveMode: azservicebus.ReceiveModePeekLock})
	if err != nil {
		return nil, fmt.Errorf("create the receiver for queue %q: %w", queue, err)
	}
	return &Messenger{client: client, sender: s, receiver: r, emptyPause: 100 * time.Millisecond}, nil
}

// Send puts one message with this body on the queue.
func (m *Messenger) Send(ctx context.Context, body []byte) error {
	ttl := messageTTL
	return m.sender.SendMessage(ctx, &azservicebus.Message{Body: body, TimeToLive: &ttl}, nil)
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
