package azure

import (
	"context"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type OperationStatus string

const (
	OperationStatusInProgress OperationStatus = "InProgress"
	OperationStatusSucceeded  OperationStatus = "Succeeded"
	OperationStatusFailed     OperationStatus = "Failed"
)

func StartResumeOperation[T any](ctx context.Context, poller *runtime.Poller[T]) (status OperationStatus, result T, resumeToken string, err error) {
	logger := log.FromContext(ctx)
	res, err := poller.Poll(ctx)
	if err != nil {
		status = OperationStatusFailed
		return
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusAccepted {
		status = OperationStatusFailed
		err = runtime.NewResponseError(res)
		return
	}
	if poller.Done() {
		status = OperationStatusSucceeded
		result, err = poller.Result(ctx)
	} else {
		status = OperationStatusInProgress
		resumeToken, err = poller.ResumeToken()
		if err != nil {
			logger.Error(err, "Failed to get resume Token")
		}
	}
	return
}
