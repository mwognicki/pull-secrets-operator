package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
	"github.com/mwognicki/pull-secrets-operator/internal/sync"
)

// RegistryPullSecretReconciler reconciles RegistryPullSecret resources into target Secrets.
type RegistryPullSecretReconciler struct {
	client.Client
}

func (r *RegistryPullSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var registryPullSecret pullsecretsv1alpha1.RegistryPullSecret
	if err := r.Get(ctx, req.NamespacedName, &registryPullSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get RegistryPullSecret %s: %w", req.NamespacedName, err)
	}

	policy, err := r.getPullSecretPolicy(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	allNamespaces, err := r.listNamespaces(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	existingSecrets, err := r.listExistingSecrets(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	desiredSecrets, err := sync.DesiredSecrets(registryPullSecret, policy, allNamespaces, existingSecrets)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("build desired Secrets for %s: %w", req.NamespacedName, err)
	}

	for _, desiredSecret := range desiredSecrets {
		if !desiredSecret.NeedsApply {
			continue
		}

		if err := r.applySecret(ctx, desiredSecret.Secret); err != nil {
			return ctrl.Result{}, err
		}

		logger.Info("applied replicated pull secret", "namespace", desiredSecret.Secret.Namespace, "name", desiredSecret.Secret.Name)
	}

	return ctrl.Result{}, nil
}

func (r *RegistryPullSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pullsecretsv1alpha1.RegistryPullSecret{}).
		Complete(r)
}

func (r *RegistryPullSecretReconciler) getPullSecretPolicy(ctx context.Context) (pullsecretsv1alpha1.PullSecretPolicy, error) {
	var policy pullsecretsv1alpha1.PullSecretPolicy
	err := r.Get(ctx, types.NamespacedName{Name: pullsecretsv1alpha1.PullSecretPolicySingletonName}, &policy)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return pullsecretsv1alpha1.PullSecretPolicy{}, nil
		}
		return pullsecretsv1alpha1.PullSecretPolicy{}, fmt.Errorf("get PullSecretPolicy %q: %w", pullsecretsv1alpha1.PullSecretPolicySingletonName, err)
	}

	return policy, nil
}

func (r *RegistryPullSecretReconciler) listNamespaces(ctx context.Context) ([]string, error) {
	var namespaceList corev1.NamespaceList
	if err := r.List(ctx, &namespaceList); err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	namespaces := make([]string, 0, len(namespaceList.Items))
	for _, namespace := range namespaceList.Items {
		namespaces = append(namespaces, namespace.Name)
	}

	return namespaces, nil
}

func (r *RegistryPullSecretReconciler) listExistingSecrets(ctx context.Context) (map[string]*corev1.Secret, error) {
	var secretList corev1.SecretList
	if err := r.List(ctx, &secretList); err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}

	secrets := make(map[string]*corev1.Secret, len(secretList.Items))
	for i := range secretList.Items {
		secret := secretList.Items[i].DeepCopy()
		secrets[secret.Namespace+"/"+secret.Name] = secret
	}

	return secrets, nil
}

func (r *RegistryPullSecretReconciler) applySecret(ctx context.Context, desired *corev1.Secret) error {
	var existing corev1.Secret
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	switch {
	case apierrors.IsNotFound(err):
		if createErr := r.Create(ctx, desired); createErr != nil {
			return fmt.Errorf("create Secret %s/%s: %w", desired.Namespace, desired.Name, createErr)
		}
		return nil
	case err != nil:
		return fmt.Errorf("get Secret %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	existing.Labels = desired.Labels
	existing.Type = desired.Type
	existing.Data = desired.Data
	if updateErr := r.Update(ctx, &existing); updateErr != nil {
		return fmt.Errorf("update Secret %s/%s: %w", desired.Namespace, desired.Name, updateErr)
	}

	return nil
}
