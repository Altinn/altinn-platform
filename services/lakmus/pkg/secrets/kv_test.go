package secrets

import (
	"context"
	"net/http"
	"testing"

	"github.com/Altinn/altinn-platform/services/lakmus/test/azfakes"
)

// Simulate 2 pages of results from the Key VaultsClient pager.
func TestListKeyVaults_FakesPager_AllPages(t *testing.T) {
	t.Parallel()

	kvSrv := azfakes.VaultsServerTwoPages()
	f := azfakes.NewEnv(&kvSrv, nil)
	ctx := context.Background()
	got, err := ListKeyVaults(ctx, "some-subscriptionId", f.Cred, f.ARM)
	if err != nil {
		t.Fatalf("ListKeyVaults error: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 vaults, got %d: %+v", len(got), got)
	}
}

// Simulate a forbidden error from the Key VaultsClient pager.
func TestListKeyVaults_FakesPager_Error(t *testing.T) {
	t.Parallel()

	errSrv := azfakes.VaultsServerError(http.StatusForbidden, "Forbidden")
	env := azfakes.NewEnv(&errSrv, nil)
	ctx := context.Background()

	_, err := ListKeyVaults(ctx, "sub-id", env.Cred, env.ARM)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
