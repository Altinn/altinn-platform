package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	apimv1alpha1 "github.com/Altinn/altinn-platform/services/dis-apim-operator/api/v1alpha1"
)

var _ = Describe("DefaultPredicate", func() {
	var (
		namespaceSuffix string
		pred            predicate.Predicate
	)

	BeforeEach(func() {
		namespaceSuffix = "test"
		pred = defaultPredicate(namespaceSuffix)
	})

	Context("CreateFunc", func() {
		It("should return true for matching suffix", func() {
			createEvent := event.CreateEvent{
				Object: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "resource-test",
					},
				},
			}
			Expect(pred.Create(createEvent)).To(BeTrue())
		})

		It("should return true for exact suffix match", func() {
			createEvent := event.CreateEvent{
				Object: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
					},
				},
			}
			Expect(pred.Create(createEvent)).To(BeTrue())
		})

		It("should return false for non-matching suffix", func() {
			createEvent := event.CreateEvent{
				Object: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "resource-prod",
					},
				},
			}
			Expect(pred.Create(createEvent)).To(BeFalse())
		})
		It("should return false when suffix appears at beginning", func() {
			createEvent := event.CreateEvent{
				Object: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-prod",
					},
				},
			}
			Expect(pred.Create(createEvent)).To(BeFalse())
		})
	})

	Context("DeleteFunc", func() {
		It("should return true for matching suffix", func() {
			deleteEvent := event.DeleteEvent{
				Object: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "resource-test",
					},
				},
			}
			Expect(pred.Delete(deleteEvent)).To(BeTrue())
		})

		It("should return true for exact suffix match", func() {
			deleteEvent := event.DeleteEvent{
				Object: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
					},
				},
			}
			Expect(pred.Delete(deleteEvent)).To(BeTrue())
		})

		It("should return false for non-matching suffix", func() {
			deleteEvent := event.DeleteEvent{
				Object: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "resource-prod",
					},
				},
			}
			Expect(pred.Delete(deleteEvent)).To(BeFalse())
		})
		It("should return false when suffix appears at beginning", func() {
			deleteEvent := event.DeleteEvent{
				Object: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-prod",
					},
				},
			}
			Expect(pred.Delete(deleteEvent)).To(BeFalse())
		})
	})

	Context("UpdateFunc", func() {
		It("should return true for matching suffix", func() {
			updateEvent := event.UpdateEvent{
				ObjectOld: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "resource-test",
					},
				},
				ObjectNew: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "resource-test",
					},
				},
			}
			Expect(pred.Update(updateEvent)).To(BeTrue())
		})

		It("should return true for exact suffix match", func() {
			updateEvent := event.UpdateEvent{
				ObjectOld: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
					},
				},
				ObjectNew: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
					},
				},
			}
			Expect(pred.Update(updateEvent)).To(BeTrue())
		})

		It("should return false for non-matching suffix", func() {
			updateEvent := event.UpdateEvent{
				ObjectOld: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "resource-test",
					},
				},
				ObjectNew: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "resource-prod",
					},
				},
			}
			Expect(pred.Update(updateEvent)).To(BeFalse())
		})

		It("should return false when suffix appears at beginning", func() {
			updateEvent := event.UpdateEvent{
				ObjectOld: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "resource-test",
					},
				},
				ObjectNew: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-prod",
					},
				},
			}
			Expect(pred.Update(updateEvent)).To(BeFalse())
		})
	})

	Context("GenericFunc", func() {
		It("should return true for matching suffix", func() {
			genericEvent := event.GenericEvent{
				Object: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "resource-test",
					},
				},
			}
			Expect(pred.Generic(genericEvent)).To(BeTrue())
		})

		It("should return true for exact suffix match", func() {
			genericEvent := event.GenericEvent{
				Object: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
					},
				},
			}
			Expect(pred.Generic(genericEvent)).To(BeTrue())
		})

		It("should return false for non-matching suffix", func() {
			genericEvent := event.GenericEvent{
				Object: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "resource-prod",
					},
				},
			}
			Expect(pred.Generic(genericEvent)).To(BeFalse())
		})

		It("should return false when suffix appears at beginning", func() {
			genericEvent := event.GenericEvent{
				Object: &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-prod",
					},
				},
			}
			Expect(pred.Generic(genericEvent)).To(BeFalse())
		})
	})
})
