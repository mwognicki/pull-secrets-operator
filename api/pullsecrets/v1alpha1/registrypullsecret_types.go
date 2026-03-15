package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NamespaceSelectionPolicy determines how Namespaces should be interpreted.
type NamespaceSelectionPolicy string

const (
	// NamespaceSelectionPolicyInclusive means only the listed namespaces are eligible.
	NamespaceSelectionPolicyInclusive NamespaceSelectionPolicy = "Inclusive"
	// NamespaceSelectionPolicyExclusive means all namespaces except the listed ones are eligible.
	NamespaceSelectionPolicyExclusive NamespaceSelectionPolicy = "Exclusive"
)

// RegistryCredentials contains the Docker registry authentication data.
type RegistryCredentials struct {
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
	Auth     string `json:"auth,omitempty"`
}

// NamespaceTargetOverride customizes the target secret name for one namespace.
type NamespaceTargetOverride struct {
	Namespace  string `json:"namespace"`
	SecretName string `json:"secretName"`
}

// NamespaceSelection defines how a registry secret should target namespaces.
type NamespaceSelection struct {
	Policy NamespaceSelectionPolicy `json:"policy"`
	// Namespaces contains either the allowed or excluded set, depending on Policy.
	Namespaces []string `json:"namespaces,omitempty"`
	// TargetSecretName is the optional default name used for replicated secrets.
	// When omitted, the operator should derive a stable human-friendly name from
	// the registry server, for example:
	// - docker.toturi.cloud -> toturi-pull-secret
	// - ghcr.io -> ghcr-pull-secret
	// - ocir.us-ashburn-1.oci.oraclecloud.com -> oraclecloud-pull-secret
	TargetSecretName string `json:"targetSecretName,omitempty"`
	// NamespaceOverrides overrides the target secret name for specific namespaces.
	NamespaceOverrides []NamespaceTargetOverride `json:"namespaceOverrides,omitempty"`
}

// RegistryPullSecretSpec defines registry credentials and replication intent.
type RegistryPullSecretSpec struct {
	Credentials RegistryCredentials `json:"credentials"`
	Namespaces  NamespaceSelection  `json:"namespaces"`
}

// RegistryPullSecretStatus is reserved for future observed state.
// TODO: populate this in the next iteration of the API/controller work.
type RegistryPullSecretStatus struct{}

// RegistryPullSecret defines credentials and namespace targeting for one registry.
// Changes to this resource should trigger prompt reconciliation so updated
// credentials, target names, and namespace rules are reflected as soon as possible.
type RegistryPullSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistryPullSecretSpec   `json:"spec,omitempty"`
	Status RegistryPullSecretStatus `json:"status,omitempty"`
}

// RegistryPullSecretList contains a list of RegistryPullSecret resources.
type RegistryPullSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RegistryPullSecret `json:"items"`
}

func (in *RegistryCredentials) DeepCopyInto(out *RegistryCredentials) {
	*out = *in
}

func (in *RegistryCredentials) DeepCopy() *RegistryCredentials {
	if in == nil {
		return nil
	}
	out := new(RegistryCredentials)
	in.DeepCopyInto(out)
	return out
}

func (in *NamespaceTargetOverride) DeepCopyInto(out *NamespaceTargetOverride) {
	*out = *in
}

func (in *NamespaceTargetOverride) DeepCopy() *NamespaceTargetOverride {
	if in == nil {
		return nil
	}
	out := new(NamespaceTargetOverride)
	in.DeepCopyInto(out)
	return out
}

func (in *NamespaceSelection) DeepCopyInto(out *NamespaceSelection) {
	*out = *in
	if in.Namespaces != nil {
		out.Namespaces = make([]string, len(in.Namespaces))
		copy(out.Namespaces, in.Namespaces)
	}
	if in.NamespaceOverrides != nil {
		out.NamespaceOverrides = make([]NamespaceTargetOverride, len(in.NamespaceOverrides))
		copy(out.NamespaceOverrides, in.NamespaceOverrides)
	}
}

func (in *NamespaceSelection) DeepCopy() *NamespaceSelection {
	if in == nil {
		return nil
	}
	out := new(NamespaceSelection)
	in.DeepCopyInto(out)
	return out
}

func (in *RegistryPullSecretSpec) DeepCopyInto(out *RegistryPullSecretSpec) {
	*out = *in
	in.Credentials.DeepCopyInto(&out.Credentials)
	in.Namespaces.DeepCopyInto(&out.Namespaces)
}

func (in *RegistryPullSecretSpec) DeepCopy() *RegistryPullSecretSpec {
	if in == nil {
		return nil
	}
	out := new(RegistryPullSecretSpec)
	in.DeepCopyInto(out)
	return out
}

func (in *RegistryPullSecretStatus) DeepCopyInto(out *RegistryPullSecretStatus) {
	*out = *in
}

func (in *RegistryPullSecretStatus) DeepCopy() *RegistryPullSecretStatus {
	if in == nil {
		return nil
	}
	out := new(RegistryPullSecretStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *RegistryPullSecret) DeepCopyInto(out *RegistryPullSecret) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *RegistryPullSecret) DeepCopy() *RegistryPullSecret {
	if in == nil {
		return nil
	}
	out := new(RegistryPullSecret)
	in.DeepCopyInto(out)
	return out
}

func (in *RegistryPullSecret) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *RegistryPullSecretList) DeepCopyInto(out *RegistryPullSecretList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]RegistryPullSecret, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *RegistryPullSecretList) DeepCopy() *RegistryPullSecretList {
	if in == nil {
		return nil
	}
	out := new(RegistryPullSecretList)
	in.DeepCopyInto(out)
	return out
}

func (in *RegistryPullSecretList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
