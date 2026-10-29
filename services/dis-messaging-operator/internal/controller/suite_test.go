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

package controller

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	servicebusv1 "github.com/Azure/azure-service-operator/v2/api/servicebus/v1api20240101"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/messaging"
)

var (
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client
	// cachedClient reads from a cache with the Queue indexes of the manager.
	cachedClient client.Client
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			filepath.Join("..", "..", "bin", "aso-crds"),
			filepath.Join("..", "..", "bin", "dis-identity-crds"),
		},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(messagingv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(servicebusv1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(authorizationv1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(identityv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())

	direct, err := client.NewWithWatch(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	k8sClient = interceptor.NewClient(direct, interceptor.Funcs{List: listByQueueName})

	queueCache, err := cache.New(cfg, cache.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(IndexQueues(ctx, queueCache)).To(Succeed())
	go func() {
		defer GinkgoRecover()
		Expect(queueCache.Start(ctx)).To(Succeed())
	}()
	cachedClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme, Cache: &client.CacheOptions{Reader: queueCache}})
	Expect(err).NotTo(HaveOccurred())
})

// listByQueueName answers a Queue list on queueNameIndex as the manager
// cache does. The API server does not know the index, so the list runs
// with the other options only, and the Queues with another key are dropped.
func listByQueueName(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
	options := (&client.ListOptions{}).ApplyOptions(opts)
	queues, ok := list.(*messagingv1alpha1.QueueList)
	if !ok || options.FieldSelector == nil {
		return c.List(ctx, list, opts...)
	}
	key, found := options.FieldSelector.RequiresExactMatch(queueNameIndex)
	if !found {
		return c.List(ctx, list, opts...)
	}
	options.FieldSelector = nil
	if err := c.List(ctx, queues, options); err != nil {
		return err
	}
	queues.Items = slices.DeleteFunc(queues.Items, func(queue messagingv1alpha1.Queue) bool {
		return messaging.NameKey(queue.Spec.Name) != key
	})
	return nil
}

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	Expect(testEnv.Stop()).To(Succeed())
})
