package azure

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var _ = Describe("Long running operations", func() {
	When("When waiting for a long running operation", func() {

		It("should return OperationStatusSucceeded", func() {
			By("returning 200 res from poller.Poll and true poller.Done")
			p, err := runtime.NewPoller(&http.Response{StatusCode: http.StatusOK}, runtime.Pipeline{}, &runtime.NewPollerOptions[string]{
				Response: ptr.To("fake response"),
				Handler: &MockPoller[string]{
					IsDone: true,
					PollResult: http.Response{
						StatusCode: http.StatusOK,
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			status, res, _, err := StartResumeOperation(context.Background(), p)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).To(Equal(OperationStatusSucceeded))
			Expect(res).To(Equal("fake response"))
			By("returning 202 res from poller.Poll and true poller.Done")
			p, err = runtime.NewPoller(&http.Response{StatusCode: http.StatusAccepted}, runtime.Pipeline{}, &runtime.NewPollerOptions[string]{
				Response: ptr.To("fake response"),
				Handler: &MockPoller[string]{
					IsDone: true,
					PollResult: http.Response{
						StatusCode: http.StatusOK,
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			status, res, _, err = StartResumeOperation(context.Background(), p)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).To(Equal(OperationStatusSucceeded))
			Expect(res).To(Equal("fake response"))
			By("returning 201 res from poller.Poll and true poller.Done")
			p, err = runtime.NewPoller(&http.Response{StatusCode: http.StatusCreated}, runtime.Pipeline{}, &runtime.NewPollerOptions[string]{
				Response: ptr.To("fake response"),
				Handler: &MockPoller[string]{
					IsDone: true,
					PollResult: http.Response{
						StatusCode: http.StatusOK,
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			status, res, _, err = StartResumeOperation(context.Background(), p)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).To(Equal(OperationStatusSucceeded))
			Expect(res).To(Equal("fake response"))
		})
		It("should return OperationStatusInProgress", func() {
			By("returning false when Done is called")
			p, err := runtime.NewPoller(nil, runtime.Pipeline{}, &runtime.NewPollerOptions[string]{
				Handler: &MockPoller[string]{
					IsDone: false,
					PollResult: http.Response{
						StatusCode: http.StatusOK,
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			status, res, _, err := StartResumeOperation(context.Background(), p)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).To(Equal(OperationStatusInProgress))
			Expect(res).To(BeEmpty())
		})
		It("should return OperationStatusFailed", func() {
			By("returning poller.Poll returning an error")
			p, err := runtime.NewPoller(nil, runtime.Pipeline{}, &runtime.NewPollerOptions[string]{
				Handler: &MockPoller[string]{
					IsDone:    false,
					PollError: errors.New("fake error"),
					PollResult: http.Response{
						StatusCode: http.StatusOK,
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			status, res, _, err := StartResumeOperation(context.Background(), p)
			Expect(err).To(HaveOccurred())
			Expect(status).To(Equal(OperationStatusFailed))
			Expect(res).To(BeEmpty())
			By("returning poller.Poll returning a non 2xx status code")
			p, err = runtime.NewPoller(nil, runtime.Pipeline{}, &runtime.NewPollerOptions[string]{
				Handler: &MockPoller[string]{
					IsDone: false,
					PollResult: http.Response{
						StatusCode: http.StatusBadRequest,
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			status, res, _, err = StartResumeOperation(context.Background(), p)
			Expect(err).To(HaveOccurred())
			Expect(status).To(Equal(OperationStatusFailed))
			Expect(res).To(BeEmpty())
		})
	})
})

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

type MockPoller[T any] struct {
	IsDone          bool
	PollResult      http.Response
	PollError       error
	FakeResultError error
}

func (p MockPoller[T]) Poll(_ context.Context) (*http.Response, error) {
	return &p.PollResult, p.PollError
}

func (p MockPoller[T]) Done() bool {
	return p.IsDone
}

func (p MockPoller[T]) Result(_ context.Context, _ *T) error {
	if p.FakeResultError != nil {
		return p.FakeResultError
	}
	return nil
}
