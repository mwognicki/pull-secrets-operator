package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion identifies the API group and version for this package.
	GroupVersion = schema.GroupVersion{Group: "pullsecrets.ognicki.ooo", Version: "v1alpha1"}

	// SchemeBuilder registers the API types for this package.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds this package's types to a scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		GroupVersion,
		&PullSecretPolicy{},
		&PullSecretPolicyList{},
		&RegistryPullSecret{},
		&RegistryPullSecretList{},
	)

	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}
