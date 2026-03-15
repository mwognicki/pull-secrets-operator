package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// PullSecretPolicySingletonName is the conventional singleton object name.
	PullSecretPolicySingletonName = "cluster"
)

// PullSecretPolicySpec defines cluster-wide operator behavior.
type PullSecretPolicySpec struct {
	// ExcludedNamespaces defines namespaces the operator must ignore globally.
	// Changes here only affect future reconciliation decisions. Existing replicated
	// secrets are left untouched when a namespace is added to or removed from this list.
	ExcludedNamespaces []string `json:"excludedNamespaces,omitempty"`
}

// PullSecretPolicyStatus is reserved for future observed state.
// TODO: populate this in the next iteration of the API/controller work.
type PullSecretPolicyStatus struct{}

// PullSecretPolicy is the cluster-wide namespace policy resource.
// The operator should treat the object named PullSecretPolicySingletonName as the
// authoritative singleton-like policy instance.
// Unlike RegistryPullSecret changes, updates here are not intended to trigger
// retroactive sync or cleanup of already replicated secrets.
type PullSecretPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PullSecretPolicySpec   `json:"spec,omitempty"`
	Status PullSecretPolicyStatus `json:"status,omitempty"`
}

// PullSecretPolicyList contains a list of PullSecretPolicy resources.
type PullSecretPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PullSecretPolicy `json:"items"`
}

func (in *PullSecretPolicySpec) DeepCopyInto(out *PullSecretPolicySpec) {
	*out = *in
	if in.ExcludedNamespaces != nil {
		out.ExcludedNamespaces = make([]string, len(in.ExcludedNamespaces))
		copy(out.ExcludedNamespaces, in.ExcludedNamespaces)
	}
}

func (in *PullSecretPolicySpec) DeepCopy() *PullSecretPolicySpec {
	if in == nil {
		return nil
	}
	out := new(PullSecretPolicySpec)
	in.DeepCopyInto(out)
	return out
}

func (in *PullSecretPolicyStatus) DeepCopyInto(out *PullSecretPolicyStatus) {
	*out = *in
}

func (in *PullSecretPolicyStatus) DeepCopy() *PullSecretPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(PullSecretPolicyStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *PullSecretPolicy) DeepCopyInto(out *PullSecretPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *PullSecretPolicy) DeepCopy() *PullSecretPolicy {
	if in == nil {
		return nil
	}
	out := new(PullSecretPolicy)
	in.DeepCopyInto(out)
	return out
}

func (in *PullSecretPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *PullSecretPolicyList) DeepCopyInto(out *PullSecretPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]PullSecretPolicy, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *PullSecretPolicyList) DeepCopy() *PullSecretPolicyList {
	if in == nil {
		return nil
	}
	out := new(PullSecretPolicyList)
	in.DeepCopyInto(out)
	return out
}

func (in *PullSecretPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
