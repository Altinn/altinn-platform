package controller

import (
	"context"
	"testing"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// testUserProvisionJobReconciler writes through the embedded client. It reads
// the Job list from a different reader, as a reconciler with a cache does.
type testUserProvisionJobReconciler struct {
	client.Client
	live client.Reader
}

func (r testUserProvisionJobReconciler) userProvisionJobReader() client.Reader { return r.live }
func (r testUserProvisionJobReconciler) userProvisionJobScheme() *runtime.Scheme {
	return r.Scheme()
}
func (r testUserProvisionJobReconciler) userProvisionJobImage() string    { return "provisioner:test" }
func (r testUserProvisionJobReconciler) userProvisionJobUseAzFakes() bool { return false }

func TestEnsureUserProvisionJobReadsJobsFromLiveReader(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := storagev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	server := testDebugJobServer()
	server.UID = "server-uid"
	builtin := []string{debugBuiltinRolePgMonitor}
	spec := debugAccessJobSpec(server, testDebugAdminIdentity(), "current-job", testDebugPrincipals(), builtin)

	newJob := func(name string, complete, inDeletion bool) *batchv1.Job {
		job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: server.Namespace,
			Labels:    userProvisionJobLabels(spec.Labels),
		}}
		if complete {
			job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
		}
		if inDeletion {
			now := metav1.Now()
			job.DeletionTimestamp = &now
			job.Finalizers = []string{metav1.FinalizerDeleteDependents}
		}
		return job
	}

	tests := []struct {
		name         string
		cached       []client.Object
		live         []client.Object
		wantComplete bool
		wantJobs     int
	}{
		{
			name:         "complete Job that is alone",
			cached:       []client.Object{newJob("current-job", true, false)},
			live:         []client.Object{newJob("current-job", true, false)},
			wantComplete: true,
			wantJobs:     1,
		},
		{
			name:     "cache does not show the deletion of the complete Job",
			cached:   []client.Object{newJob("current-job", true, false)},
			live:     []client.Object{newJob("current-job", true, true)},
			wantJobs: 1,
		},
		{
			name:     "cache does not show a Job with another name",
			live:     []client.Object{newJob("other-job", false, false)},
			wantJobs: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cached := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.cached...).Build()
			live := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.live...).Build()

			complete, err := ensureUserProvisionJobForReconciler(
				context.Background(), testUserProvisionJobReconciler{Client: cached, live: live}, spec,
			)
			if err != nil {
				t.Fatalf("ensureUserProvisionJobForReconciler: %v", err)
			}
			if complete != tt.wantComplete {
				t.Fatalf("complete = %t, want %t", complete, tt.wantComplete)
			}
			var jobs batchv1.JobList
			if err := cached.List(context.Background(), &jobs); err != nil {
				t.Fatal(err)
			}
			if len(jobs.Items) != tt.wantJobs {
				t.Fatalf("the function left %d Jobs, want %d", len(jobs.Items), tt.wantJobs)
			}
		})
	}
}
