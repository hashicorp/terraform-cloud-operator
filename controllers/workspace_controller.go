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
	"strings"

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
)

type TerraformCloudClient struct {
	Client *tfc.Client
}

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}

type workspaceInstance struct {
	instance appv1alpha2.Workspace

	log      logr.Logger
	tfClient TerraformCloudClient
}

// +kubebuilder:rbac:groups=app.terraform.io,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.terraform.io,resources=workspaces/finalizers,verbs=update
// +kubebuilder:rbac:groups=app.terraform.io,resources=workspaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=create;list;update;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=create;list;update;watch

func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	w := workspaceInstance{}

	w.log = log.Log.WithValues("workspace", req.NamespacedName)
	w.log.Info("Workspace Controller", "msg", "new reconciliation event")

	err := r.Client.Get(ctx, req.NamespacedName, &w.instance)
	if err != nil {
		// 'Not found' error occurs when an object is removed from the Kubernetes
		// No actions are required in this case
		if errors.IsNotFound(err) {
			w.log.Info("Workspace Controller", "msg", "the object is removed no further action is required")
			return doNotRequeue()
		}
		w.log.Error(err, "Workspace Controller", "msg", "get instance object")
		return requeueAfter(requeueInterval)
	}

	// Migration Validation
	if controllerutil.ContainsFinalizer(&w.instance, workspaceFinalizerAlpha1) {
		w.log.Error(err, "Migration", "msg", fmt.Sprintf("spec contains old finalizer %s", workspaceFinalizerAlpha1))
		return doNotRequeue()
	}

	w.log.Info("Spec Validation", "msg", "validating instance object spec")
	if err := w.instance.ValidateSpec(); err != nil {
		w.log.Error(err, "Spec Validation", "msg", "spec is invalid, exit from reconciliation")
		r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "SpecValidation", err.Error())
		return doNotRequeue()
	}
	w.log.Info("Spec Validation", "msg", "spec is valid")

	if needToAddFinalizer(&w.instance, workspaceFinalizer) {
		err := r.addFinalizer(ctx, &w.instance)
		if err != nil {
			w.log.Error(err, "Workspace Controller", "msg", fmt.Sprintf("failed to add finalizer %s to the object", workspaceFinalizer))
			r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "AddFinalizer", "Failed to add finalizer %s to the object", workspaceFinalizer)
			return requeueOnErr(err)
		}
		w.log.Info("Workspace Controller", "msg", fmt.Sprintf("successfully added finalizer %s to the object", workspaceFinalizer))
		r.Recorder.Eventf(&w.instance, corev1.EventTypeNormal, "AddFinalizer", "Successfully added finalizer %s to the object", workspaceFinalizer)
	}

	err = r.getTerraformClient(ctx, &w)
	if err != nil {
		w.log.Error(err, "Workspace Controller", "msg", "failed to get terraform cloud client")
		r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "TerraformClient", "Failed to get Terraform Client")
		return requeueAfter(requeueInterval)
	}

	err = r.reconcileWorkspace(ctx, &w)
	if err != nil {
		w.log.Error(err, "Workspace Controller", "msg", "reconcile workspace")
		r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Failed to reconcile workspace")
		return requeueAfter(requeueInterval)
	}
	w.log.Info("Workspace Controller", "msg", "successfully reconcilied workspace")

	if w.instance.Spec.InitialRun == true {
		if workspaceWaitForUploadModule(&w.instance) {
			w.log.Info("Module Controller", "msg", "waiting for configuration version to be uploaded")
			return requeueAfter(requeueConfigurationUploadInterval)
		}

		if r.waitForUploadWorkspace(ctx, &w) {
			w.log.Info("Module Controller", "msg", "waiting for configuration version to be uploaded")
			return requeueAfter(requeueConfigurationUploadInterval)
		}

		if workspaceNeedNewRun(&w.instance) {
			w.log.Info("Module Controller", "msg", "new config version is available, need a new run")
			return requeueAfter(requeueNewRunInterval)
		}

		if workspaceWaitRunToFinish(&w.instance) {
			w.log.Info("Module Controller", "msg", "waiting for run to finish")
			return requeueAfter(requeueRunStatusInterval)
		}
	}

	r.Recorder.Eventf(&w.instance, corev1.EventTypeNormal, "ReconcileWorkspace", "Successfully reconcilied workspace ID %s", w.instance.Status.WorkspaceID)

	return doNotRequeue()
}

// waitForUploadModule checks if need to wait for CV upload to finish
func workspaceWaitForUploadModule(instance *appv1alpha2.Workspace) bool {
	if instance.Status.WorkspaceID == "" {
		return false
	}

	return true
}

func (r *WorkspaceReconciler) waitForUploadWorkspace(ctx context.Context, w *workspaceInstance) bool {
	workspaceVariables, err := r.getWorkspaceVariables(ctx, w)
	if err != nil {
		w.log.Error(err, "Reconcile Workspace", "msg", "failed to get workspace variables")
		r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Failed to get workspace variables")
		return false
	}
	instanceVariables, err := r.getVariablesByCategory(ctx, w, tfc.CategoryTerraform, true)
	if err != nil {
		return true
	}
	terraformWorkspaceVariables := getWorkspaceVariablesByCategory(workspaceVariables, tfc.CategoryTerraform)

	if len(getVariablesToCreate(instanceVariables, terraformWorkspaceVariables)) > 0 {
		return true
	}

	instanceVariables, err = r.getVariablesByCategory(ctx, w, tfc.CategoryEnv, true)
	if err != nil {
		return true
	}
	envWorkspaceVariables := getWorkspaceVariablesByCategory(workspaceVariables, tfc.CategoryEnv)

	if len(getVariablesToCreate(instanceVariables, envWorkspaceVariables)) > 0 {
		return true
	}

	return false
}

// needNewRun checks is a new Run is required
func workspaceNeedNewRun(instance *appv1alpha2.Workspace) bool {
	if instance.Status.Run.ConfigurationVersion == string(tfc.ConfigurationErrored) {
		return false
	}

	if instance.Status.Run.ID == "" {
		return true
	}

	if instance.Status.Run.Status == "" {
		return true
	}

	return false
}

// waitRunToFinish checks if need to wait for Run to finish
func workspaceWaitRunToFinish(instance *appv1alpha2.Workspace) bool {
	if instance.Status.Run.Status == "" {
		return false
	}

	// Run is not finished until it get one of the below statuses
	switch instance.Status.Run.Status {
	case string(tfc.RunApplied):
		return false
	case string(tfc.RunPlannedAndFinished):
		return false
	case string(tfc.RunErrored):
		return false
	case string(tfc.RunCanceled):
		return false
	case string(tfc.RunDiscarded):
		return false
	}

	return true
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha2.Workspace{}).
		WithEventFilter(predicate.Or(genericPredicates())).
		Complete(r)
}

func (r *WorkspaceReconciler) getConfigMap(ctx context.Context, name types.NamespacedName) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, name, cm)

	return cm, err
}

func (r *WorkspaceReconciler) getSecret(ctx context.Context, name types.NamespacedName) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, name, secret)

	return secret, err
}

func (r *WorkspaceReconciler) getToken(ctx context.Context, instance *appv1alpha2.Workspace) (string, error) {
	var secret *corev1.Secret

	secretName := instance.Spec.Token.SecretKeyRef.Name
	secretKey := instance.Spec.Token.SecretKeyRef.Key

	objectKey := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      secretName,
	}
	secret, err := r.getSecret(ctx, objectKey)
	if err != nil {
		return "", err
	}

	if token, ok := secret.Data[secretKey]; ok {
		return strings.TrimSuffix(string(token), "\n"), nil
	}
	return "", fmt.Errorf("token key %s does not exist in the secret %s", secretKey, secretName)
}

func (r *WorkspaceReconciler) getTerraformClient(ctx context.Context, w *workspaceInstance) error {
	token, err := r.getToken(ctx, &w.instance)
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
		w.log.Info("Reconcile Workspace", "msg", "client configured to skip TLS certificate verifications")
	}

	httpClient.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: insecure}

	config := &tfc.Config{
		Token:      token,
		HTTPClient: httpClient,
	}
	w.tfClient.Client, err = tfc.NewClient(config)

	return err
}

func needToUpdateWorkspace(instance *appv1alpha2.Workspace, workspace *tfc.Workspace) bool {
	// generation changed
	if instance.Generation != instance.Status.ObservedGeneration {
		return true
	}
	// timestamp changed
	if workspace.UpdatedAt.Unix() != instance.Status.UpdateAt {
		return true
	}
	// agent pool added
	if instance.Spec.AgentPool != nil && workspace.AgentPool == nil {
		return true
	}
	// The TFC API VCS behavior is inconsistent:
	//   - once a VCS is attached to a Workspace, the 'UpdateAt' attribute is updated
	//   - once attached to a Workspace VCS is updated, for instance, change a branch name, the 'UpdateAt' attribute is updated
	//   - once a VCS is detached from a Workspace, the 'UpdateAt' attribute is not updated, because of that we have to have this condition here
	if instance.Spec.VersionControl != nil && workspace.VCSRepo == nil {
		return true
	}
	// Terraform version
	if instance.Spec.TerraformVersion == "" {
		if instance.Status.TerraformVersion != workspace.TerraformVersion {
			return true
		}
	}

	return false
}

// applyMethodToBool turns spec.applyMethod field into bool to align with the Workspace AutoApply field
// `spec.applyMethod: auto` is equal to `AutoApply: true`
// `spec.applyMethod: manual` is equal to `AutoApply: false`
func applyMethodToBool(applyMethod string) bool {
	return applyMethod == "auto"
}

// autoApplyToStr turns the Workspace AutoApply field into string to align with spec.applyMethod
// `AutoApply: true` is equal to `spec.applyMethod: auto`
// `AutoApply: false` is equal to `spec.applyMethod: manual`
func autoApplyToStr(autoApply bool) string {
	if autoApply {
		return "auto"
	}
	return "manual"
}

func (r *WorkspaceReconciler) addFinalizer(ctx context.Context, instance *appv1alpha2.Workspace) error {
	controllerutil.AddFinalizer(instance, workspaceFinalizer)

	return r.Update(ctx, instance)
}

func (r *WorkspaceReconciler) removeFinalizer(ctx context.Context, w *workspaceInstance) error {
	controllerutil.RemoveFinalizer(&w.instance, workspaceFinalizer)

	err := r.Update(ctx, &w.instance)
	if err != nil {
		w.log.Error(err, "Reconcile Workspace", "msg", fmt.Sprintf("failed to remove finazlier %s", workspaceFinalizer))
		r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "RemoveFinalizer", "Failed to remove finazlier %s", workspaceFinalizer)
	}

	return err
}

// STATUS
// TODO need to update this to update the spec with default values from TFC API
// change this function to updateObject?
func (r *WorkspaceReconciler) updateStatus(ctx context.Context, w *workspaceInstance, workspace *tfc.Workspace) error {
	w.instance.Status.ObservedGeneration = w.instance.Generation
	w.instance.Status.UpdateAt = workspace.UpdatedAt.Unix()
	w.instance.Status.WorkspaceID = workspace.ID
	w.instance.Status.TerraformVersion = workspace.TerraformVersion

	if workspace.CurrentRun != nil {
		w.instance.Status.Run.ID = workspace.CurrentRun.ID
		run, err := w.tfClient.Client.Runs.Read(ctx, workspace.CurrentRun.ID)
		if err != nil {
			return err
		}
		w.instance.Status.Run.Status = string(run.Status)
	}

	return r.Status().Update(ctx, &w.instance)
}

// WORKSPACES
func (r *WorkspaceReconciler) createWorkspace(ctx context.Context, w *workspaceInstance) (*tfc.Workspace, error) {
	spec := w.instance.Spec
	options := tfc.WorkspaceCreateOptions{
		Name:             tfc.String(spec.Name),
		AllowDestroyPlan: tfc.Bool(spec.AllowDestroyPlan),
		AutoApply:        tfc.Bool(applyMethodToBool(spec.ApplyMethod)),
		Description:      tfc.String(spec.Description),
		ExecutionMode:    tfc.String(spec.ExecutionMode),
		TerraformVersion: tfc.String(spec.TerraformVersion),
		WorkingDirectory: tfc.String(spec.WorkingDirectory),
	}

	if spec.ExecutionMode == "agent" {
		agentPoolID, err := r.getAgentPoolID(ctx, w)
		if err != nil {
			w.log.Error(err, "Reconcile Workspace", "msg", "failed to get agent pool ID")
			r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Failed to get agent pool ID")
			return nil, err
		}
		w.log.Info("Reconcile Workspace", "msg", fmt.Sprintf("agent pool ID %s will be used", agentPoolID))
		options.AgentPoolID = tfc.String(agentPoolID)
	}

	if spec.VersionControl != nil {
		options.VCSRepo = &tfc.VCSRepoOptions{
			OAuthTokenID: tfc.String(spec.VersionControl.OAuthTokenID),
			Identifier:   tfc.String(spec.VersionControl.Repository),
			Branch:       tfc.String(spec.VersionControl.Branch),
		}
		options.FileTriggersEnabled = tfc.Bool(false)
	}

	if spec.RemoteStateSharing != nil {
		options.GlobalRemoteState = tfc.Bool(spec.RemoteStateSharing.AllWorkspaces)
	}

	if spec.Project != nil {
		prjID, err := r.getProjectID(ctx, w)
		if err != nil {
			w.log.Error(err, "Reconcile Workspace", "msg", "failed to get project ID")
			r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Failed to get project ID")
			return nil, err
		}
		w.log.Info("Reconcile Workspace", "msg", fmt.Sprintf("project ID %s will be used", prjID))
		options.Project = &tfc.Project{ID: prjID}
	}

	workspace, err := w.tfClient.Client.Workspaces.Create(ctx, spec.Organization, options)
	if err != nil {
		return nil, err
	}

	w.instance.Status = appv1alpha2.WorkspaceStatus{
		WorkspaceID: workspace.ID,
	}

	return workspace, nil
}

func (r *WorkspaceReconciler) readWorkspace(ctx context.Context, w *workspaceInstance) (*tfc.Workspace, error) {
	return w.tfClient.Client.Workspaces.ReadByID(ctx, w.instance.Status.WorkspaceID)
}

func (r *WorkspaceReconciler) updateWorkspace(ctx context.Context, w *workspaceInstance, workspace *tfc.Workspace) (*tfc.Workspace, error) {
	updateOptions := tfc.WorkspaceUpdateOptions{}
	spec := w.instance.Spec
	status := w.instance.Status

	if spec.ExecutionMode == "agent" {
		agentPoolID, err := r.getAgentPoolID(ctx, w)
		if err != nil {
			w.log.Error(err, "Reconcile Workspace", "msg", "failed to get agent pool ID")
			r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Failed to get agent pool ID")
			return nil, err
		}
		w.log.Info("Reconcile Workspace", "msg", fmt.Sprintf("agent pool ID %s will be used", agentPoolID))
		updateOptions.AgentPoolID = tfc.String(agentPoolID)
	}

	if workspace.Name != spec.Name {
		updateOptions.Name = tfc.String(spec.Name)
	}

	if workspace.AutoApply != applyMethodToBool(spec.ApplyMethod) {
		updateOptions.AutoApply = tfc.Bool(applyMethodToBool(spec.ApplyMethod))
	}

	if workspace.AllowDestroyPlan != spec.AllowDestroyPlan {
		updateOptions.AllowDestroyPlan = tfc.Bool(spec.AllowDestroyPlan)
	}

	if workspace.Description != spec.Description {
		updateOptions.Description = tfc.String(spec.Description)
	}

	if workspace.ExecutionMode != spec.ExecutionMode {
		updateOptions.ExecutionMode = tfc.String(spec.ExecutionMode)
	}

	if spec.RemoteStateSharing != nil {
		if workspace.GlobalRemoteState != spec.RemoteStateSharing.AllWorkspaces {
			updateOptions.GlobalRemoteState = tfc.Bool(spec.RemoteStateSharing.AllWorkspaces)
		}
	}

	if spec.TerraformVersion == "" {
		if workspace.TerraformVersion != status.TerraformVersion {
			updateOptions.TerraformVersion = tfc.String(status.TerraformVersion)
		}
	} else {
		if workspace.TerraformVersion != spec.TerraformVersion {
			updateOptions.TerraformVersion = tfc.String(spec.TerraformVersion)
		}
	}

	if workspace.WorkingDirectory != spec.WorkingDirectory {
		updateOptions.WorkingDirectory = tfc.String(spec.WorkingDirectory)
		updateOptions.Name = tfc.String(spec.Name)
	}

	if workspace.ExecutionMode != spec.ExecutionMode {
		updateOptions.ExecutionMode = tfc.String(spec.ExecutionMode)
	}

	if spec.VersionControl == nil && workspace.VCSRepo != nil {
		ws, err := w.tfClient.Client.Workspaces.RemoveVCSConnectionByID(ctx, workspace.ID)
		if err != nil {
			return ws, err
		}
	}

	if spec.VersionControl != nil {
		updateOptions.VCSRepo = &tfc.VCSRepoOptions{
			OAuthTokenID: tfc.String(spec.VersionControl.OAuthTokenID),
			Identifier:   tfc.String(spec.VersionControl.Repository),
			Branch:       tfc.String(spec.VersionControl.Branch),
		}
		updateOptions.FileTriggersEnabled = tfc.Bool(false)
	}

	if spec.Project != nil {
		prjID, err := r.getProjectID(ctx, w)
		if err != nil {
			w.log.Error(err, "Reconcile Workspace", "msg", "failed to get project ID")
			r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Failed to get project ID")
			return nil, err
		}
		w.log.Info("Reconcile Workspace", "msg", fmt.Sprintf("project ID %s will be used", prjID))
		updateOptions.Project = &tfc.Project{ID: prjID}
	} else {
		// Setting up `Project` to nil(tfc.WorkspaceUpdateOptions{Project: nil}) won't move the workspace to the default project after the update.
		org, err := w.tfClient.Client.Organizations.Read(ctx, w.instance.Spec.Organization)
		if err != nil {
			w.log.Error(err, "Reconcile Workspace", "msg", "failed to get organization")
			r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Failed to get organization")
			return nil, err
		}
		w.log.Info("Reconcile Workspace", "msg", fmt.Sprintf("default project ID %s will be used", org.DefaultProject.ID))
		updateOptions.Project = &tfc.Project{ID: org.DefaultProject.ID}
	}

	return w.tfClient.Client.Workspaces.UpdateByID(ctx, w.instance.Status.WorkspaceID, updateOptions)
}

func (r *WorkspaceReconciler) deleteWorkspace(ctx context.Context, w *workspaceInstance) error {
	// if the Kubernetes object doesn't have workspace ID, it means it a workspace was never created
	// in this case, remove the finalizer and let Kubernetes remove the object permanently
	if w.instance.Status.WorkspaceID == "" {
		w.log.Info("Reconcile Workspace", "msg", fmt.Sprintf("status.WorkspaceID is empty, remove finazlier %s", workspaceFinalizer))
		return r.removeFinalizer(ctx, w)
	}
	err := w.tfClient.Client.Workspaces.DeleteByID(ctx, w.instance.Status.WorkspaceID)
	if err != nil {
		// if workspace wasn't found, it means it was deleted from the TF Cloud bypass the operator
		// in this case, remove the finalizer and let Kubernetes remove the object permanently
		if err == tfc.ErrResourceNotFound {
			w.log.Info("Reconcile Workspace", "msg", fmt.Sprintf("Workspace ID %s not found, remove finazlier", workspaceFinalizer))
			return r.removeFinalizer(ctx, w)
		}
		w.log.Error(err, "Reconcile Workspace", "msg", fmt.Sprintf("failed to delete Workspace ID %s, retry later", workspaceFinalizer))
		r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Failed to delete Workspace ID %s, retry later", w.instance.Status.WorkspaceID)
		return err
	}

	w.log.Info("Reconcile Workspace", "msg", fmt.Sprintf("workspace ID %s has been deleted, remove finazlier", w.instance.Status.WorkspaceID))
	return r.removeFinalizer(ctx, w)
}

func (r *WorkspaceReconciler) reconcileWorkspace(ctx context.Context, w *workspaceInstance) error {
	w.log.Info("Reconcile Workspace", "msg", "reconciling workspace")

	var workspace *tfc.Workspace
	var err error

	defer func() {
		// Update the status with the Workspace ID. This is useful if the reconciliation failed.
		// An example here would be the case when the workspace has been created successfully,
		// but further reconciliation steps failed.
		//
		// If a Workspace creation operation failed, we don't have a workspace object
		// and thus don't update the status. An example here would be the case when the workspace name has already been taken.
		//
		// Cannot call updateStatus method since it updated multiple fields and can break reconciliation logic.
		//
		// TODO:
		// - Use conditions(https://maelvls.dev/kubernetes-conditions/)
		// - Let Objects update their own status conditions
		// - Simplify updateStatus method in a way it could be called anytime
		if workspace != nil && workspace.ID != "" {
			w.instance.Status.WorkspaceID = workspace.ID
			err = r.Status().Update(ctx, &w.instance)
			if err != nil {
				w.log.Error(err, "Workspace Controller", "msg", "update status with workspace ID")
				r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Failed to update status with workspace ID")
			}
		}
	}()

	// verify whether the Kubernetes object has been marked as deleted and if so delete the workspace
	if isDeletionCandidate(&w.instance, workspaceFinalizer) {
		w.log.Info("Reconcile Workspace", "msg", "object marked as deleted, need to delete workspace first")
		r.Recorder.Event(&w.instance, corev1.EventTypeNormal, "ReconcileWorkspace", "Object marked as deleted, need to delete workspace first")
		return r.deleteWorkspace(ctx, w)
	}

	// create a new workspace if workspace ID is unknown(means it was never created by the controller)
	// this condition will work just one time, when a new Kubernetes object is created
	if w.instance.IsCreationCandidate() {
		w.log.Info("Reconcile Workspace", "msg", "status.WorkspaceID is empty, creating a new workspace")
		r.Recorder.Event(&w.instance, corev1.EventTypeNormal, "ReconcileWorkspace", "Status.WorkspaceID is empty, creating a new workspace")
		workspace, err = r.createWorkspace(ctx, w)
		if err != nil {
			w.log.Error(err, "Reconcile Workspace", "msg", "failed to create a new workspace")
			r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Failed to create a new workspace")
			return err
		}
		w.log.Info("Reconcile Workspace", "msg", "successfully created a new workspace")
		r.Recorder.Eventf(&w.instance, corev1.EventTypeNormal, "ReconcileWorkspace", "Successfully created a new workspace with ID %s", w.instance.Status.WorkspaceID)
	}

	// read the Terraform Cloud workspace to compare it with the Kubernetes object spec
	workspace, err = r.readWorkspace(ctx, w)
	if err != nil {
		// 'ResourceNotFound' means that the TF Cloud workspace was removed from the TF Cloud bypass the operator
		if err == tfc.ErrResourceNotFound {
			w.log.Info("Reconcile Workspace", "msg", "workspace not found, creating a new workspace")
			r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Workspace ID %s not found, creating a new workspace", w.instance.Status.WorkspaceID)
			workspace, err = r.createWorkspace(ctx, w)
			if err != nil {
				w.log.Error(err, "Reconcile Workspace", "msg", "failed to create a new workspace")
				r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Failed to create a new workspace")
				return err
			}
			w.log.Info("Reconcile Workspace", "msg", "successfully created a new workspace")
			r.Recorder.Eventf(&w.instance, corev1.EventTypeNormal, "ReconcileWorkspace", "Successfully created a new workspace with ID %s", w.instance.Status.WorkspaceID)
		} else {
			w.log.Error(err, "Reconcile Workspace", "msg", fmt.Sprintf("failed to read workspace ID %s", w.instance.Status.WorkspaceID))
			r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Failed to read workspace ID %s", w.instance.Status.WorkspaceID)
			return err
		}
	}

	// update workspace if any changes have been made in the Kubernetes object spec or Terraform Cloud workspace
	if needToUpdateWorkspace(&w.instance, workspace) {
		w.log.Info("Reconcile Workspace", "msg", fmt.Sprintf("observed and desired states are not matching, need to update workspace ID %s", w.instance.Status.WorkspaceID))
		workspace, err = r.updateWorkspace(ctx, w, workspace)
		if err != nil {
			w.log.Error(err, "Reconcile Workspace", "msg", fmt.Sprintf("failed to update workspace ID %s", w.instance.Status.WorkspaceID))
			r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "ReconcileWorkspace", "Failed to update workspace ID %s", w.instance.Status.WorkspaceID)
			return err
		}
		w.log.Info("Reconcile Workspace", "msg", fmt.Sprintf("Successfully updated workspace ID %s", w.instance.Status.WorkspaceID))
		r.Recorder.Eventf(&w.instance, corev1.EventTypeNormal, "ReconcileWorkspace", "Successfully updated workspace ID %s", w.instance.Status.WorkspaceID)
	} else {
		w.log.Info("Reconcile Workspace", "msg", fmt.Sprintf("observed and desired states are matching, no need to update workspace ID %s", w.instance.Status.WorkspaceID))
	}

	// reconcile SSH key
	err = r.reconcileSSHKey(ctx, w, workspace)
	if err != nil {
		w.log.Error(err, "Reconcile SSH Key", "msg", "failed to assign ssh key ID")
		r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "ReconcileSSHKey", "Failed to assign SSH Key ID")
		return err
	} else {
		w.log.Info("Reconcile SSH Key", "msg", "successfully reconcile ssh key")
		r.Recorder.Event(&w.instance, corev1.EventTypeNormal, "ReconcileSSHKey", "Successfully reconcile SSH Key")
	}

	// Reconcile Tags
	err = r.reconcileTags(ctx, w, workspace)
	if err != nil {
		w.log.Error(err, "Reconcile Tags", "msg", "reconcile tags")
		r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "ReconcileTags", "Failed to reconcile tags in workspace ID %s", w.instance.Status.WorkspaceID)
		return err
	}
	w.log.Info("Reconcile Tags", "msg", "successfully reconcilied tags")
	r.Recorder.Eventf(&w.instance, corev1.EventTypeNormal, "ReconcileTags", "Successfully reconcilied tags in workspace ID %s", w.instance.Status.WorkspaceID)

	// Reconcile Variables
	err = r.reconcileVariables(ctx, w, workspace)
	if err != nil {
		w.log.Error(err, "Reconcile Variables", "msg", fmt.Sprintf("failed to reconcile variables in workspace ID %s", w.instance.Status.WorkspaceID))
		r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "ReconcileVariables", "Failed to reconcile variables in workspace ID %s", w.instance.Status.WorkspaceID)
		return err
	}
	w.log.Info("Reconcile Variables", "msg", "successfully reconcilied variables")
	r.Recorder.Eventf(&w.instance, corev1.EventTypeNormal, "ReconcileVariables", "Reconcilied variables in workspace ID %s", w.instance.Status.WorkspaceID)

	// Reconcile Run Triggers
	err = r.reconcileRunTriggers(ctx, w)
	if err != nil {
		w.log.Error(err, "Reconcile Run Triggers", "msg", fmt.Sprintf("failed to reconcile run triggers in workspace ID %s", w.instance.Status.WorkspaceID))
		r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "ReconcileRunTriggers", "Failed to reconcile run triggers in workspace ID %s", w.instance.Status.WorkspaceID)
		return err
	}
	w.log.Info("Reconcile Run Triggers", "msg", "successfully reconcilied run triggers")
	r.Recorder.Eventf(&w.instance, corev1.EventTypeNormal, "ReconcileRunTriggers", "Reconcilied run triggers in workspace ID %s", w.instance.Status.WorkspaceID)

	// Reconcile Outputs
	err = r.reconcileOutputs(ctx, w, workspace)
	if err != nil {
		w.log.Error(err, "Reconcile Outputs", "msg", "failed to reconcile outputs")
		r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "ReconcileOutputs", "Failed to reconcile outputs in workspace ID %s", w.instance.Status.WorkspaceID)
		return err
	}
	w.log.Info("Reconcile Outputs", "msg", "successfully reconcilied outputs")
	r.Recorder.Eventf(&w.instance, corev1.EventTypeNormal, "ReconcileOutputs", "Successfully reconcilied outputs in workspace ID %s", w.instance.Status.WorkspaceID)

	// Reconcile Team Access
	err = r.reconcileTeamAccess(ctx, w)
	if err != nil {
		w.log.Error(err, "Reconcile Team Access", "msg", fmt.Sprintf("failed to reconcile team access in workspace ID %s", w.instance.Status.WorkspaceID))
		r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "ReconcileTeamAccess", "Failed to reconcile team access in workspace ID %s", w.instance.Status.WorkspaceID)
		return err
	}
	w.log.Info("Reconcile Team Access", "msg", "successfully reconcilied team access")
	r.Recorder.Eventf(&w.instance, corev1.EventTypeNormal, "ReconcileTeamAccess", "Reconcilied team access in workspace ID %s", w.instance.Status.WorkspaceID)

	// Reconcile Remote State Sharing
	err = r.reconcileRemoteStateSharing(ctx, w)
	if err != nil {
		w.log.Error(err, "Reconcile Remote State Sharing", "msg", fmt.Sprintf("failed to reconcile remote state sharing in workspace ID %s", w.instance.Status.WorkspaceID))
		r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "ReconcileRemoteStateSharing", "Failed to reconcile remote state sharing in workspace ID %s", w.instance.Status.WorkspaceID)
		return err
	}
	w.log.Info("Reconcile Remote State Sharing", "msg", "successfully reconcilied remote state sharing")
	r.Recorder.Eventf(&w.instance, corev1.EventTypeNormal, "ReconcileRemoteStateSharing", "Reconcilied remote state sharing in workspace ID %s", w.instance.Status.WorkspaceID)

	// Reconcile Run Tasks
	err = r.reconcileRunTasks(ctx, w)
	if err != nil {
		w.log.Error(err, "Reconcile Run Tasks", "msg", fmt.Sprintf("failed to reconcile run tasks in workspace ID %s", w.instance.Status.WorkspaceID))
		r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "ReconcileRunTasks", "Failed to reconcile run tasks in workspace ID %s", w.instance.Status.WorkspaceID)
		return err
	}
	w.log.Info("Reconcile Run Tasks", "msg", "successfully reconcilied run tasks")
	r.Recorder.Eventf(&w.instance, corev1.EventTypeNormal, "ReconcileRunTasks", "Reconcilied run tasks in workspace ID %s", w.instance.Status.WorkspaceID)

	// Reconcile Notifications
	err = r.reconcileNotifications(ctx, w)
	if err != nil {
		w.log.Error(err, "Reconcile Notifications", "msg", fmt.Sprintf("failed to reconcile notifications in workspace ID %s", w.instance.Status.WorkspaceID))
		r.Recorder.Eventf(&w.instance, corev1.EventTypeWarning, "ReconcileNotifications", "Failed to reconcile notifications in workspace ID %s", w.instance.Status.WorkspaceID)
		return err
	}
	w.log.Info("Reconcile Notifications", "msg", "successfully reconcilied notifications")
	r.Recorder.Eventf(&w.instance, corev1.EventTypeNormal, "ReconcileNotifications", "Reconcilied notifications in workspace ID %s", w.instance.Status.WorkspaceID)

	if w.instance.Spec.InitialRun == true {
		if r.waitForUploadWorkspace(ctx, w) {
			w.log.Info("Reconcile Workspace", "msg", "waiting for configuration version to be uploaded")
			return r.updateStatus(ctx, w, workspace)
		}

		if workspaceNeedNewRun(&w.instance) {
			w.log.Info("Reconcile Workspace", "msg", "new config version is available, need a new run")

			planOnly := false
			if w.instance.Spec.InitialRunType == "plan" {
				planOnly = true
			}
			run, err := w.tfClient.Client.Runs.Create(ctx, tfc.RunCreateOptions{
				Message:   tfc.String("Triggered by the Kubernetes Operator"),
				Workspace: workspace,
				PlanOnly:  tfc.Bool(planOnly),
			})
			if err != nil {
				w.log.Error(err, "Reconcile Run", "msg", "failed to create a new run")
				return err
			}
			w.log.Info("Reconcile Run", "msg", "successfully created a new run")

			// It can take a while to proceed with a new run
			// To unblock a worker we return the object back to the queue
			// and validate the run status during the next reconciliation
			return r.updateStatusRun(ctx, &w.instance, workspace, run)
		}

		// checks if a new version of the Run is finished
		if workspaceWaitRunToFinish(&w.instance) {
			w.log.Info("Reconcile Run", "msg", "check the run status")
			run, err := w.tfClient.Client.Runs.Read(ctx, w.instance.Status.Run.ID)
			if err != nil {
				w.log.Error(err, "Reconcile Run", "msg", "failed to get run status")
				return err
			}
			w.log.Info("Reconcile Run", "msg", fmt.Sprintf("successfully got the run status: %s", run.Status))
			return r.updateStatusRun(ctx, &w.instance, workspace, run)
		}
	}

	return r.updateStatus(ctx, w, workspace)
}

func (r *WorkspaceReconciler) updateStatusRun(ctx context.Context, instance *appv1alpha2.Workspace, workspace *tfc.Workspace, run *tfc.Run) error {
	instance.Status.WorkspaceID = workspace.ID
	instance.Status.ObservedGeneration = instance.Generation
	instance.Status.Run = appv1alpha2.RunStatus{
		ID:                   run.ID,
		Status:               string(run.Status),
		ConfigurationVersion: run.ConfigurationVersion.ID,
	}

	return r.Status().Update(ctx, instance)
}
