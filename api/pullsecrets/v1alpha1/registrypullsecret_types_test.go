package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRegistryPullSecretDeepCopyPreservesIndependence(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	original := &RegistryPullSecret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: GroupVersion.String(),
			Kind:       "RegistryPullSecret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "ghcr",
			Generation: 3,
		},
		Spec: RegistryPullSecretSpec{
			Credentials: &RegistryCredentials{
				Server:   "ghcr.io",
				Username: "octocat",
				Password: "s3cret",
			},
			CredentialsSecretRef: &SecretReference{
				Name:      "ghcr-creds",
				Namespace: "ops",
			},
			Namespaces: NamespaceSelection{
				Policy:           NamespaceSelectionPolicyInclusive,
				Namespaces:       []string{"team-a", "team-b"},
				TargetSecretName: "ghcr-pull-secret",
				NamespaceOverrides: []NamespaceTargetOverride{
					{Namespace: "team-a", SecretName: "team-a-ghcr"},
				},
			},
		},
		Status: RegistryPullSecretStatus{
			ObservedGeneration: 3,
			DesiredSecretCount: 2,
			LastSyncTime:       &now,
			Conditions: []metav1.Condition{
				{
					Type:   "Ready",
					Status: metav1.ConditionTrue,
					Reason: "Synced",
				},
			},
		},
	}

	copied := original.DeepCopy()
	if copied == nil {
		t.Fatal("DeepCopy() returned nil")
	}

	copied.Spec.Credentials.Server = "docker.io"
	copied.Spec.CredentialsSecretRef.Name = "dockerhub-creds"
	copied.Spec.Namespaces.Namespaces[0] = "other"
	copied.Spec.Namespaces.NamespaceOverrides[0].SecretName = "override"
	copied.Status.LastSyncTime.Time = copied.Status.LastSyncTime.Time.AddDate(1, 0, 0)
	copied.Status.Conditions[0].Reason = "Changed"

	if original.Spec.Credentials.Server != "ghcr.io" {
		t.Fatalf("original credentials mutated: %q", original.Spec.Credentials.Server)
	}
	if original.Spec.CredentialsSecretRef.Name != "ghcr-creds" {
		t.Fatalf("original secret ref mutated: %q", original.Spec.CredentialsSecretRef.Name)
	}
	if original.Spec.Namespaces.Namespaces[0] != "team-a" {
		t.Fatalf("original namespaces mutated: %#v", original.Spec.Namespaces.Namespaces)
	}
	if original.Spec.Namespaces.NamespaceOverrides[0].SecretName != "team-a-ghcr" {
		t.Fatalf("original namespace override mutated: %#v", original.Spec.Namespaces.NamespaceOverrides)
	}
	if original.Status.Conditions[0].Reason != "Synced" {
		t.Fatalf("original conditions mutated: %#v", original.Status.Conditions)
	}
}

func TestRegistryPullSecretDeepCopyNilHelpers(t *testing.T) {
	t.Parallel()

	var credentials *RegistryCredentials
	if credentials.DeepCopy() != nil {
		t.Fatal("nil RegistryCredentials.DeepCopy() should return nil")
	}

	var secretRef *SecretReference
	if secretRef.DeepCopy() != nil {
		t.Fatal("nil SecretReference.DeepCopy() should return nil")
	}

	var override *NamespaceTargetOverride
	if override.DeepCopy() != nil {
		t.Fatal("nil NamespaceTargetOverride.DeepCopy() should return nil")
	}

	var selection *NamespaceSelection
	if selection.DeepCopy() != nil {
		t.Fatal("nil NamespaceSelection.DeepCopy() should return nil")
	}

	var spec *RegistryPullSecretSpec
	if spec.DeepCopy() != nil {
		t.Fatal("nil RegistryPullSecretSpec.DeepCopy() should return nil")
	}

	var status *RegistryPullSecretStatus
	if status.DeepCopy() != nil {
		t.Fatal("nil RegistryPullSecretStatus.DeepCopy() should return nil")
	}

	var resource *RegistryPullSecret
	if resource.DeepCopy() != nil {
		t.Fatal("nil RegistryPullSecret.DeepCopy() should return nil")
	}

	var list *RegistryPullSecretList
	if list.DeepCopy() != nil {
		t.Fatal("nil RegistryPullSecretList.DeepCopy() should return nil")
	}
}

func TestRegistryPullSecretListDeepCopyObject(t *testing.T) {
	t.Parallel()

	original := &RegistryPullSecretList{
		Items: []RegistryPullSecret{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "ghcr"},
				Spec: RegistryPullSecretSpec{
					Namespaces: NamespaceSelection{
						Policy:     NamespaceSelectionPolicyInclusive,
						Namespaces: []string{"team-a"},
					},
				},
			},
		},
	}

	copiedObject := original.DeepCopyObject()
	copied, ok := copiedObject.(*RegistryPullSecretList)
	if !ok {
		t.Fatalf("DeepCopyObject() type = %T, want *RegistryPullSecretList", copiedObject)
	}

	copied.Items[0].Spec.Namespaces.Namespaces[0] = "team-b"
	if original.Items[0].Spec.Namespaces.Namespaces[0] != "team-a" {
		t.Fatalf("original list item mutated: %#v", original.Items)
	}
}
