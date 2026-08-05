// Package e2e contains dis-console end-to-end tests that exercise the store
// against a real PostgreSQL running in a Kind cluster. The tests are guarded by
// the `e2e` build tag and the DISCONSOLE_TEST_DB_URI environment variable, and
// are run via `make test-e2e-kind-ci`.
//
// This untagged file keeps the package buildable under the default build tags
// so `go vet ./...` and `go test ./...` (which exclude the e2e-tagged files) do
// not fail with "build constraints exclude all Go files".
package e2e
