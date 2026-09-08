/*
Copyright 2026.

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

package cache

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
)

const (
	// AuthUsername is the Valkey ACL user the operator creates for the team application.
	AuthUsername = "app"

	// AuthSecretUsernameKey and AuthSecretPasswordKey are the data keys in the
	// auth Secret. The application reads both. The controller points the
	// upstream users[].passwordSecret at the password key.
	AuthSecretUsernameKey = "username"
	AuthSecretPasswordKey = "password"
)

// AuthSecretName returns the name of the Secret that holds the cache credentials.
func AuthSecretName(cache *cachev1alpha1.Cache) string {
	return cache.Name + "-cache-auth"
}

// BuildAuthSecret maps a Cache and a generated password to the credentials Secret.
func BuildAuthSecret(cache *cachev1alpha1.Cache, password string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AuthSecretName(cache),
			Namespace: cache.Namespace,
			Labels:    Labels(cache),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			AuthSecretUsernameKey: []byte(AuthUsername),
			AuthSecretPasswordKey: []byte(password),
		},
	}
}
