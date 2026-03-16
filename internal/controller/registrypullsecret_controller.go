package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	pullsecretsv1alpha1 "github.com/mwognicki/pull-secrets-operator/api/pullsecrets/v1alpha1"
	"github.com/mwognicki/pull-secrets-operator/internal/sync"
)

// RegistryPullSecretReconciler reconciles RegistryPullSecret resources into target Secrets.
type RegistryPullSecretReconciler struct {
	client.Client
}

type registryPullSecretReconcileStatus struct {
	desiredSecretCount int32
	appliedSecretCount int32
	deletedSecretCount int32
}

type registryPullSecretControllerBuilder interface {
	For(object client.Object, opts ...ctrlbuilder.ForOption) registryPullSecretControllerBuilder
	Watches(
		object client.Object,
		eventHandler handler.TypedEventHandler[client.Object, reconcile.Request],
		opts ...ctrlbuilder.WatchesOption,
	) registryPullSecretControllerBuilder
	Complete(reconciler reconcile.TypedReconciler[reconcile.Request]) error
}

type registryPullSecretControllerBuilderAdapter struct {
	builder *ctrlbuilder.TypedBuilder[reconcile.Request]
}

func (a registryPullSecretControllerBuilderAdapter) For(object client.Object, opts ...ctrlbuilder.ForOption) registryPullSecretControllerBuilder {
	a.builder = a.builder.For(object, opts...)
	return a
}

func (a registryPullSecretControllerBuilderAdapter) Watches(
	object client.Object,
	eventHandler handler.TypedEventHandler[client.Object, reconcile.Request],
	opts ...ctrlbuilder.WatchesOption,
) registryPullSecretControllerBuilder {
	a.builder = a.builder.Watches(object, eventHandler, opts...)
	return a
}

func (a registryPullSecretControllerBuilderAdapter) Complete(reconciler reconcile.TypedReconciler[reconcile.Request]) error {
	return a.builder.Complete(reconciler)
}

var newRegistryPullSecretControllerBuilder = func(mgr ctrl.Manager) registryPullSecretControllerBuilder {
	return registryPullSecretControllerBuilderAdapter{
		builder: ctrl.NewControllerManagedBy(mgr),
	}
}

func (r *RegistryPullSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var registryPullSecret pullsecretsv1alpha1.RegistryPullSecret
	if err := r.Get(ctx, req.NamespacedName, &registryPullSecret); err != nil {
		if apierrors.IsNotFound(err) {
			// Source deletions are intentionally non-destructive for now. Managed
			// replicated Secrets are left in place and no finalizer-based cleanup is
			// attempted when the RegistryPullSecret itself disappears.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get RegistryPullSecret %s: %w", req.NamespacedName, err)
	}

	status, reconcileErr := r.reconcileRegistryPullSecret(ctx, logger, &registryPullSecret)
	statusErr := r.updateRegistryPullSecretStatus(ctx, &registryPullSecret, status, reconcileErr)

	return ctrl.Result{}, errors.Join(reconcileErr, statusErr)
}

func (r *RegistryPullSecretReconciler) reconcileRegistryPullSecret(
	ctx context.Context,
	logger logr.Logger,
	registryPullSecret *pullsecretsv1alpha1.RegistryPullSecret,
) (registryPullSecretReconcileStatus, error) {
	policy, err := r.getPullSecretPolicy(ctx)
	if err != nil {
		return registryPullSecretReconcileStatus{}, err
	}
	credentials, err := r.resolveRegistryCredentials(ctx, registryPullSecret)
	if err != nil {
		return registryPullSecretReconcileStatus{}, err
	}

	allNamespaces, err := r.listNamespaces(ctx)
	if err != nil {
		return registryPullSecretReconcileStatus{}, err
	}

	existingSecrets, err := r.listExistingSecrets(ctx)
	if err != nil {
		return registryPullSecretReconcileStatus{}, err
	}

	desiredSecrets, err := sync.DesiredSecrets(*registryPullSecret, credentials, policy, allNamespaces, existingSecrets)
	if err != nil {
		return registryPullSecretReconcileStatus{}, fmt.Errorf("build desired Secrets for %s: %w", client.ObjectKeyFromObject(registryPullSecret), err)
	}
	obsoleteSecrets := sync.ObsoleteSecrets(*registryPullSecret, existingSecrets, desiredSecrets)

	status := registryPullSecretReconcileStatus{
		desiredSecretCount: int32(len(desiredSecrets)),
	}

	for _, desiredSecret := range desiredSecrets {
		if !desiredSecret.NeedsApply {
			continue
		}

		if err := r.applySecret(ctx, desiredSecret.Secret); err != nil {
			return status, err
		}
		status.appliedSecretCount++

		logger.Info("applied replicated pull secret", "namespace", desiredSecret.Secret.Namespace, "name", desiredSecret.Secret.Name)
	}

	for _, obsoleteSecret := range obsoleteSecrets {
		if err := r.deleteSecret(ctx, obsoleteSecret); err != nil {
			return status, err
		}
		status.deletedSecretCount++

		logger.Info("deleted obsolete replicated pull secret", "namespace", obsoleteSecret.Namespace, "name", obsoleteSecret.Name)
	}

	return status, nil
}

func (r *RegistryPullSecretReconciler) resolveRegistryCredentials(
	ctx context.Context,
	registryPullSecret *pullsecretsv1alpha1.RegistryPullSecret,
) (pullsecretsv1alpha1.RegistryCredentials, error) {
	var sourceSecret *corev1.Secret
	if registryPullSecret.Spec.CredentialsSecretRef != nil {
		var secret corev1.Secret
		key := types.NamespacedName{
			Name:      registryPullSecret.Spec.CredentialsSecretRef.Name,
			Namespace: registryPullSecret.Spec.CredentialsSecretRef.Namespace,
		}
		if err := r.Get(ctx, key, &secret); err != nil {
			return pullsecretsv1alpha1.RegistryCredentials{}, fmt.Errorf("get credentials Secret %s: %w", key, err)
		}
		sourceSecret = &secret
	}

	credentials, err := sync.ResolveRegistryCredentials(registryPullSecret.Spec, sourceSecret)
	if err != nil {
		return pullsecretsv1alpha1.RegistryCredentials{}, fmt.Errorf("resolve credentials for %s: %w", client.ObjectKeyFromObject(registryPullSecret), err)
	}

	return credentials, nil
}

func (r *RegistryPullSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return newRegistryPullSecretControllerBuilder(mgr).
		For(&pullsecretsv1alpha1.RegistryPullSecret{}).
		Watches(
			&corev1.Secret{},
			// Only source credential Secret changes trigger reconciliation. Drift in
			// managed replica Secrets is intentionally left alone until a future
			// RegistryPullSecret reconcile, such as after operator restart.
			handler.EnqueueRequestsFromMapFunc(r.registryPullSecretsForSecret),
		).
		Complete(r)
}

func (r *RegistryPullSecretReconciler) registryPullSecretsForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	var registryPullSecretList pullsecretsv1alpha1.RegistryPullSecretList
	if err := r.List(ctx, &registryPullSecretList); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0)
	for _, registryPullSecret := range registryPullSecretList.Items {
		ref := registryPullSecret.Spec.CredentialsSecretRef
		if ref == nil {
			continue
		}
		if ref.Name != secret.Name || ref.Namespace != secret.Namespace {
			continue
		}

		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&registryPullSecret),
		})
	}

	return requests
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

func (r *RegistryPullSecretReconciler) deleteSecret(ctx context.Context, secret *corev1.Secret) error {
	if err := r.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete Secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	return nil
}

func (r *RegistryPullSecretReconciler) updateRegistryPullSecretStatus(
	ctx context.Context,
	registryPullSecret *pullsecretsv1alpha1.RegistryPullSecret,
	status registryPullSecretReconcileStatus,
	reconcileErr error,
) error {
	now := metav1.Now()
	registryPullSecret.Status.ObservedGeneration = registryPullSecret.Generation
	registryPullSecret.Status.DesiredSecretCount = status.desiredSecretCount
	registryPullSecret.Status.AppliedSecretCount = status.appliedSecretCount
	registryPullSecret.Status.DeletedSecretCount = status.deletedSecretCount
	registryPullSecret.Status.LastSyncTime = &now

	condition := metav1.Condition{
		Type:               "Ready",
		ObservedGeneration: registryPullSecret.Generation,
		LastTransitionTime: now,
	}
	if reconcileErr != nil {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "SyncFailed"
		condition.Message = reconcileErr.Error()
	} else {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "Synced"
		condition.Message = "RegistryPullSecret reconciled successfully"
	}
	apimeta.SetStatusCondition(&registryPullSecret.Status.Conditions, condition)

	if err := r.Status().Update(ctx, registryPullSecret); err != nil {
		return fmt.Errorf("update RegistryPullSecret status %s: %w", client.ObjectKeyFromObject(registryPullSecret), err)
	}

	return nil
}
