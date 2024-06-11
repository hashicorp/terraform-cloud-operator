// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"
	tfc "github.com/hashicorp/go-tfe"
	appv1alpha2 "github.com/hashicorp/terraform-cloud-operator/api/v1alpha2"
	"github.com/hashicorp/terraform-cloud-operator/version"
)

// AgentPoolReconciler reconciles a AgentPool object
type AgentPoolReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}

type agentPoolInstance struct {
	instance appv1alpha2.AgentPool

	log      logr.Logger
	tfClient HCPTerraformClient
}

//+kubebuilder:rbac:groups=app.terraform.io,resources=agentpools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=app.terraform.io,resources=agentpools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=app.terraform.io,resources=agentpools/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=create;list;update;watch
//+kubebuilder:rbac:groups="apps",resources=deployments,verbs=create;delete;get;list;patch;update;watch

func (r *AgentPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ap := agentPoolInstance{}

	ap.log = log.Log.WithValues("agentpool", req.NamespacedName)
	ap.log.Info("Agent Pool Controller", "msg", "new reconciliation event")

	err := r.Client.Get(ctx, req.NamespacedName, &ap.instance)
	if err != nil {
		// 'Not found' error occurs when an object is removed from the Kubernetes
		// No actions are required in this case
		if errors.IsNotFound(err) {
			ap.log.Info("Agent Pool Controller", "msg", "the object is removed no further action is required")
			return doNotRequeue()
		}
		ap.log.Error(err, "Agent Pool Controller", "msg", "get instance object")
		return requeueAfter(requeueInterval)
	}

	ap.log.Info("Spec Validation", "msg", "validating instance object spec")
	if err := ap.instance.ValidateSpec(); err != nil {
		ap.log.Error(err, "Spec Validation", "msg", "spec is invalid, exit from reconciliation")
		r.Recorder.Event(&ap.instance, corev1.EventTypeWarning, "SpecValidation", err.Error())
		return doNotRequeue()
	}
	ap.log.Info("Spec Validation", "msg", "spec is valid")

	if needToAddFinalizer(&ap.instance, agentPoolFinalizer) {
		err := r.addFinalizer(ctx, &ap.instance)
		if err != nil {
			ap.log.Error(err, "Agent Pool Controller", "msg", fmt.Sprintf("failed to add finalizer %s to the object", agentPoolFinalizer))
			r.Recorder.Eventf(&ap.instance, corev1.EventTypeWarning, "AddFinalizer", "Failed to add finalizer %s to the object", agentPoolFinalizer)
			return requeueOnErr(err)
		}
		ap.log.Info("Agent Pool Controller", "msg", fmt.Sprintf("successfully added finalizer %s to the object", agentPoolFinalizer))
		r.Recorder.Eventf(&ap.instance, corev1.EventTypeNormal, "AddFinalizer", "Successfully added finalizer %s to the object", agentPoolFinalizer)
	}

	err = r.getTerraformClient(ctx, &ap)
	if err != nil {
		ap.log.Error(err, "Agent Pool Controller", "msg", "failed to get HCP Terraform client")
		r.Recorder.Event(&ap.instance, corev1.EventTypeWarning, "TerraformClient", "Failed to get HCP Terraform Client")
		return requeueAfter(requeueInterval)
	}

	err = r.reconcileAgentPool(ctx, &ap)
	if err != nil {
		ap.log.Error(err, "Agent Pool Controller", "msg", "reconcile agent pool")
		r.Recorder.Event(&ap.instance, corev1.EventTypeWarning, "ReconcileAgentPool", "Failed to reconcile agent pool")
		return requeueAfter(requeueInterval)
	}
	ap.log.Info("Agent Pool Controller", "msg", "successfully reconcilied agent pool")
	r.Recorder.Eventf(&ap.instance, corev1.EventTypeNormal, "ReconcileAgentPool", "Successfully reconcilied agent pool ID %s", ap.instance.Status.AgentPoolID)

	return requeueAfter(AgentPoolSyncPeriod)
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha2.AgentPool{}).
		WithEventFilter(predicate.Or(genericPredicates())).
		Complete(r)
}

func (r *AgentPoolReconciler) getTerraformClient(ctx context.Context, ap *agentPoolInstance) error {
	nn := types.NamespacedName{
		Namespace: ap.instance.Namespace,
		Name:      ap.instance.Spec.Token.SecretKeyRef.Name,
	}
	token, err := secretKeyRef(ctx, r.Client, nn, ap.instance.Spec.Token.SecretKeyRef.Key)
	if err != nil {
		return err
	}

	httpClient := tfc.DefaultConfig().HTTPClient
	insecure := false

	if v, ok := os.LookupEnv("TFC_TLS_SKIP_VERIFY"); ok {
		insecure, err = strconv.ParseBool(v)
		if err != nil {
			return err
		}
	}

	if insecure {
		ap.log.Info("Reconcile Workspace", "msg", "client configured to skip TLS certificate verifications")
	}

	httpClient.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: insecure}

	config := &tfc.Config{
		Token:      token,
		HTTPClient: httpClient,
		Headers: http.Header{
			"User-Agent": []string{version.UserAgent},
		},
	}
	ap.tfClient.Client, err = tfc.NewClient(config)

	return err
}

func (r *AgentPoolReconciler) addFinalizer(ctx context.Context, instance *appv1alpha2.AgentPool) error {
	controllerutil.AddFinalizer(instance, agentPoolFinalizer)

	return r.Update(ctx, instance)
}

func (r *AgentPoolReconciler) removeFinalizer(ctx context.Context, ap *agentPoolInstance) error {
	controllerutil.RemoveFinalizer(&ap.instance, agentPoolFinalizer)

	err := r.Update(ctx, &ap.instance)
	if err != nil {
		ap.log.Error(err, "Reconcile Agent Pool", "msg", fmt.Sprintf("failed to remove finazlier %s", agentPoolFinalizer))
		r.Recorder.Eventf(&ap.instance, corev1.EventTypeWarning, "RemoveFinalizer", "Failed to remove finazlier %s", agentPoolFinalizer)
	}

	return err
}

func (r *AgentPoolReconciler) updateStatus(ctx context.Context, ap *agentPoolInstance, agentPool *tfc.AgentPool) error {
	if ap != nil {
		ap.instance.Status.ObservedGeneration = ap.instance.Generation
	}
	if agentPool != nil {
		ap.instance.Status.AgentPoolID = agentPool.ID
	}
	return r.Status().Update(ctx, &ap.instance)
}

func (r *AgentPoolReconciler) createAgentPool(ctx context.Context, ap *agentPoolInstance) (*tfc.AgentPool, error) {
	options := tfc.AgentPoolCreateOptions{
		Name: &ap.instance.Spec.Name,
	}
	agentPool, err := ap.tfClient.Client.AgentPools.Create(ctx, ap.instance.Spec.Organization, options)
	if err != nil {
		return nil, err
	}

	ap.instance.Status = appv1alpha2.AgentPoolStatus{
		AgentPoolID: agentPool.ID,
	}

	return agentPool, nil
}

func (r *AgentPoolReconciler) updateAgentPool(ctx context.Context, ap *agentPoolInstance, agentPool *tfc.AgentPool) (*tfc.AgentPool, error) {
	options := tfc.AgentPoolUpdateOptions{
		Name: tfc.String(ap.instance.Spec.Name),
	}
	spec := ap.instance.Spec

	if agentPool.Name != spec.Name {
		options.Name = tfc.String(spec.Name)
	}

	return ap.tfClient.Client.AgentPools.Update(ctx, ap.instance.Status.AgentPoolID, options)
}

func (r *AgentPoolReconciler) deleteAgentPool(ctx context.Context, ap *agentPoolInstance) error {
	if ap.instance.Status.AgentPoolID == "" {
		ap.log.Info("Reconcile Agent Pool", "msg", fmt.Sprintf("status.agentPoolID is empty, remove finazlier %s", agentPoolFinalizer))
		return r.removeFinalizer(ctx, ap)
	}
	err := ap.tfClient.Client.AgentPools.Delete(ctx, ap.instance.Status.AgentPoolID)
	if err != nil {
		// if agent pool wasn't found, it means it was deleted from the TF Cloud bypass the operator
		// in this case, remove the finalizer and let Kubernetes remove the object permanently
		if err == tfc.ErrResourceNotFound {
			ap.log.Info("Reconcile Agent Pool", "msg", fmt.Sprintf("Agent Pool ID %s not found, remove finalizer", agentPoolFinalizer))
			return r.removeFinalizer(ctx, ap)
		}
		ap.log.Error(err, "Reconcile Agent Pool", "msg", fmt.Sprintf("failed to delete Agent Pool ID %s, retry later", agentPoolFinalizer))
		r.Recorder.Eventf(&ap.instance, corev1.EventTypeWarning, "ReconcileAgentPool", "Failed to delete Agent Pool ID %s, retry later", ap.instance.Status.AgentPoolID)
		return err
	}

	ap.log.Info("Reconcile Agent Pool", "msg", fmt.Sprintf("agent pool ID %s has been deleted, remove finazlier", ap.instance.Status.AgentPoolID))
	return r.removeFinalizer(ctx, ap)
}

func (r *AgentPoolReconciler) readAgentPool(ctx context.Context, ap *agentPoolInstance) (*tfc.AgentPool, error) {
	return ap.tfClient.Client.AgentPools.ReadWithOptions(ctx, ap.instance.Status.AgentPoolID, &tfc.AgentPoolReadOptions{
		Include: []tfc.AgentPoolIncludeOpt{
			tfc.AgentPoolWorkspaces,
		},
	})
}

func needToUpdateAgentPool(instance *appv1alpha2.AgentPool) bool {
	return instance.Generation != instance.Status.ObservedGeneration
}

func (r *AgentPoolReconciler) reconcileAgentPool(ctx context.Context, ap *agentPoolInstance) error {
	defer func() {
		agentPool, rerr := r.readAgentPool(ctx, ap) // check if it has been deleted
		if rerr != nil {
			return
		}
		serr := r.updateStatus(ctx, ap, agentPool)
		if serr != nil {
			r.Recorder.Event(&ap.instance, corev1.EventTypeWarning, "Update Status", fmt.Sprintf("Failed to update agent pool status: %s", serr))
			ap.log.Error(serr, "Failed to update agent pool status")
		}
	}()

	ap.log.Info("Reconcile Agent Pool", "msg", "reconciling agent pool")

	var agentPool *tfc.AgentPool
	var err error

	if isDeletionCandidate(&ap.instance, agentPoolFinalizer) {
		ap.log.Info("Reconcile Agent Pool", "msg", "object marked as deleted, need to delete agent pool first")
		r.Recorder.Event(&ap.instance, corev1.EventTypeNormal, "ReconcileAgentPool", "Object marked as deleted, need to delete agent pool first")
		return r.deleteAgentPool(ctx, ap)
	}

	if ap.instance.IsCreationCandidate() {
		ap.log.Info("Reconcile Agent Pool", "msg", "status.agentPoolID is empty, creating a new agent pool")
		r.Recorder.Event(&ap.instance, corev1.EventTypeNormal, "ReconcileAgentPool", "Status.AgentPoolID is empty, creating a new agent pool")
		agentPool, err = r.createAgentPool(ctx, ap)
		if err != nil {
			ap.log.Error(err, "Reconcile Agent Pool", "msg", "failed to create a new agent pool")
			r.Recorder.Event(&ap.instance, corev1.EventTypeWarning, "ReconcileAgentPool", "Failed to create a new agent pool")
			return err
		}
		ap.log.Info("Reconcile Agent Pool", "msg", fmt.Sprintf("successfully created a new agent pool with ID %s", agentPool.ID))
		r.Recorder.Eventf(&ap.instance, corev1.EventTypeNormal, "ReconcileAgentPool", "Successfully created a new agent pool with ID %s", agentPool.ID)
	}

	agentPool, err = r.readAgentPool(ctx, ap)
	if err != nil {
		if err == tfc.ErrResourceNotFound {
			ap.log.Info("Reconcile Agent Pool", "msg", "agent pool not found, creating a new agent pool with")
			r.Recorder.Eventf(&ap.instance, corev1.EventTypeWarning, "ReconcileAgentPool", "agent Pool ID %s not found, creating a new agent pool", ap.instance.Status.AgentPoolID)
			agentPool, err = r.createAgentPool(ctx, ap)
			if err != nil {
				ap.log.Error(err, "Reconcile Agent Pool", "msg", "failed to create a new agent pool")
				r.Recorder.Event(&ap.instance, corev1.EventTypeWarning, "ReconcileAgentPool", "Failed to create a new agent pool")
				return err
			}
			ap.log.Info("Reconcile Agent Pool", "msg", fmt.Sprintf("successfully created a new agent pool with ID %s", agentPool.ID))
			r.Recorder.Eventf(&ap.instance, corev1.EventTypeNormal, "ReconcileAgentPool", "Successfully created a new agent pool with ID %s", agentPool.ID)
		} else {
			ap.log.Error(err, "Reconcile Agent Pool", "msg", fmt.Sprintf("failed to read agent pool ID %s", ap.instance.Status.AgentPoolID))
			r.Recorder.Eventf(&ap.instance, corev1.EventTypeWarning, "ReconcileAgentPool", "Failed to read agent pool ID %s", ap.instance.Status.AgentPoolID)
			return err
		}
	}

	// Update Agent Pool
	if needToUpdateAgentPool(&ap.instance) {
		_, err = r.updateAgentPool(ctx, ap, agentPool)
		if err != nil {
			ap.log.Error(err, "Reconcile Agent Pool", "msg", fmt.Sprintf("failed to update agent pool ID %s", ap.instance.Status.AgentPoolID))
			r.Recorder.Eventf(&ap.instance, corev1.EventTypeWarning, "ReconcileAgentPool", "Failed to update agent pool ID %s", ap.instance.Status.AgentPoolID)
			return err
		}
		ap.log.Info("Reconcile Agent Pool", "msg", "successfully updated agent pool")
	}

	// Reconcile Agent Tokens
	err = r.reconcileAgentTokens(ctx, ap)
	if err != nil {
		ap.log.Error(err, "Reconcile Agent Tokens", "msg", fmt.Sprintf("failed to reconcile agent tokens in agent pool ID %s", ap.instance.Status.AgentPoolID))
		r.Recorder.Eventf(&ap.instance, corev1.EventTypeWarning, "ReconcileAgentTokens", "Failed to reconcile agent tokens in agent pool ID %s", ap.instance.Status.AgentPoolID)
		return err
	}
	ap.log.Info("Reconcile Agent Tokens", "msg", "successfully reconcilied agent tokens")
	r.Recorder.Eventf(&ap.instance, corev1.EventTypeNormal, "ReconcileAgentTokens", "Reconcilied agent tokens in agent pool ID %s", ap.instance.Status.AgentPoolID)

	// Reconcile Agent Deployment
	err = r.reconcileAgentDeployment(ctx, ap)
	if err != nil {
		ap.log.Error(err, "Reconcile Agent Deployment", "msg", fmt.Sprintf("failed to reconcile agent deployment in agent pool ID %s: %s", ap.instance.Status.AgentPoolID, err))
		r.Recorder.Eventf(&ap.instance, corev1.EventTypeWarning, "ReconcileAgentDeployment", "Failed to reconcile agent deployment in agent pool: %s", err)
		return err
	}
	ap.log.Info("Reconcile Agent Deployment", "msg", "successfully reconcilied agent deployment")
	r.Recorder.Eventf(&ap.instance, corev1.EventTypeNormal, "ReconcileAgentDeployment", "Reconcilied agent deployment in agent pool ID %s", ap.instance.Status.AgentPoolID)

	// Reconcile Agent Autoscaling
	err = r.reconcileAgentAutoscaling(ctx, ap)
	if err != nil {
		ap.log.Error(err, "Reconcile Agent Autoscaling", "msg", "reconcile agent autoscaling")
		r.Recorder.Eventf(&ap.instance, corev1.EventTypeWarning, "ReconcileAgentAutoscaling", "Failed to reconcile agent autoscaling in agent Pool ID%s", ap.instance.Status.AgentPoolID)
		return nil
	}
	ap.log.Info("Reconcile Agent Autoscaling", "msg", "successfully reconcilied agent autoscaling")
	r.Recorder.Eventf(&ap.instance, corev1.EventTypeNormal, "ReconcileAgentAutoscaling", "Reconcilied agent autoscaling in agent pool ID %s", ap.instance.Status.AgentPoolID)

	return nil
}
