/*
Copyright 2024 altinn.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	apimv1alpha1 "github.com/Altinn/altinn-platform/services/dis-apim-operator/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var backendlog = logf.Log.WithName("backend-resource")

// SetupBackendWebhookWithManager registers the webhook for Backend in the manager.
func SetupBackendWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&apimv1alpha1.Backend{}).
		WithDefaulter(&BackendCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-apim-dis-altinn-cloud-v1alpha1-backend,mutating=true,failurePolicy=fail,sideEffects=None,groups=apim.dis.altinn.cloud,resources=backends,verbs=create;update,versions=v1alpha1,name=mbackend-v1alpha1.kb.io,admissionReviewVersions=v1

// BackendCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Backend when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type BackendCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &BackendCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Backend.
func (d *BackendCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	backend, ok := obj.(*apimv1alpha1.Backend)

	if !ok {
		return fmt.Errorf("expected an Backend object but got %T", obj)
	}
	backendlog.Info("Defaulting for Backend", "name", backend.GetName())
	if backend.Spec.AzureResourcePrefix == nil {
		randomString, err := generateRandomString(8)
		if err != nil {
			return err
		}
		backend.Spec.AzureResourcePrefix = &randomString
	}
	return nil
}

// generateRandomString generates a random string of the given length
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}
