// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	tfc "github.com/hashicorp/go-tfe"
	appv1alpha2 "github.com/hashicorp/terraform-cloud-operator/api/v1alpha2"
)

// HELPERS

// getVariablesToCreate returns a map of variables that need to be created, i.e. exist in manifest, but absent in workspace.
func getVariablesToCreate(instanceVariables, workspaceVariables map[string]tfc.Variable) map[string]tfc.Variable {
	return varDifference(instanceVariables, workspaceVariables)
}

// getVariablesToDelete returns a map of variables that need to be deleted, i.e. exist in workspace, but absent in manifest.
func getVariablesToDelete(instanceVariables, workspaceVariables map[string]tfc.Variable) map[string]tfc.Variable {
	return varDifference(workspaceVariables, instanceVariables)
}

// getVariablesToUpdate returns a map of variables that exist in both
// the manifest and workspace but have differing values.
//
// NOTE We cannot read the value of sensitive variables from the
// Terraform Cloud API, so sensitive variables will always need to be updated.
func getVariablesToUpdate(instanceVariables, workspaceVariables map[string]tfc.Variable) map[string]tfc.Variable {
	variables := make(map[string]tfc.Variable)

	if len(instanceVariables) == 0 || len(workspaceVariables) == 0 {
		return variables
	}

	for ik, iv := range instanceVariables {
		if wv, ok := workspaceVariables[ik]; ok {
			if !isNoLongerSensitive(iv.Sensitive, wv.Sensitive) && !cmp.Equal(iv, wv, cmpopts.IgnoreFields(tfc.Variable{}, "ID", "VersionID", "Workspace")) {
				iv.ID = wv.ID
				variables[iv.Key] = iv
			}
		}
	}

	return variables
}

// getVariablesRequiringRecreate returns a map of variables requiring to recreate.
func getVariablesRequiringRecreate(instanceVariables, workspaceVariables map[string]tfc.Variable) map[string]tfc.Variable {
	variables := make(map[string]tfc.Variable)

	if len(instanceVariables) == 0 || len(workspaceVariables) == 0 {
		return variables
	}

	// When the "Sensitive" attribute changes from true to false we need to delete and recreate a variable
	for ik, iv := range instanceVariables {
		if wv, ok := workspaceVariables[ik]; ok {
			if isNoLongerSensitive(iv.Sensitive, wv.Sensitive) {
				iv.ID = wv.ID
				variables[iv.Key] = iv
			}
		}
	}

	return variables
}

// isNoLongerSensitive verifies either the attribute 'Sensitive' of the variable changed from 'true' to 'false'.
func isNoLongerSensitive(instanceSensitive, workspaceSensitive bool) bool {
	return !instanceSensitive && workspaceSensitive
}

// varDifference returns a map that contains the elements of map `a` that are not in map `b`.
// It compares only the names of the variables, not their content.
func varDifference(a, b map[string]tfc.Variable) map[string]tfc.Variable {
	variables := make(map[string]tfc.Variable)

	for k, v := range a {
		if _, ok := b[k]; !ok {
			variables[k] = v
		}
	}

	return variables
}

func (r *WorkspaceReconciler) createWorkspaceVariables(ctx context.Context, w *workspaceInstance, variables map[string]tfc.Variable, category tfc.CategoryType) error {
	for _, v := range variables {
		_, err := w.tfClient.Client.Variables.Create(ctx, w.instance.Status.WorkspaceID, tfc.VariableCreateOptions{
			Key:         &v.Key,
			Description: &v.Description,
			Value:       &v.Value,
			HCL:         &v.HCL,
			Sensitive:   &v.Sensitive,
			Category:    &category,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// getWorkspaceVariables returns a list of all variables associated with the workspace.
func (r *WorkspaceReconciler) getWorkspaceVariables(ctx context.Context, w *workspaceInstance) ([]*tfc.Variable, error) {
	v, err := w.tfClient.Client.Variables.List(ctx, w.instance.Status.WorkspaceID, &tfc.VariableListOptions{})
	if err != nil {
		return []*tfc.Variable{}, err
	}
	return v.Items, nil
}

func (r *WorkspaceReconciler) updateWorkspaceVariables(ctx context.Context, w *workspaceInstance, variables map[string]tfc.Variable) error {
	for _, v := range variables {
		_, err := w.tfClient.Client.Variables.Update(ctx, w.instance.Status.WorkspaceID, v.ID, tfc.VariableUpdateOptions{
			Key:         &v.Key,
			Description: &v.Description,
			Value:       &v.Value,
			HCL:         &v.HCL,
			Sensitive:   &v.Sensitive,
			Category:    &v.Category,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// updateWorkspaceSensitiveVariables treats a special case when the attribute 'Sensitive' of the variable changed from 'true' to 'false'.
// In this case, we have to delete the variable and create a new one.
func (r *WorkspaceReconciler) updateWorkspaceSensitiveVariables(ctx context.Context, w *workspaceInstance, variables map[string]tfc.Variable, category tfc.CategoryType) error {
	err := r.deleteWorkspaceVariables(ctx, w, variables)
	if err != nil {
		return err
	}
	err = r.createWorkspaceVariables(ctx, w, variables, category)
	if err != nil {
		return err
	}

	return nil
}

func (r *WorkspaceReconciler) deleteWorkspaceVariables(ctx context.Context, w *workspaceInstance, variables map[string]tfc.Variable) error {
	workspaceID := w.instance.Status.WorkspaceID
	for _, v := range variables {
		err := w.tfClient.Client.Variables.Delete(ctx, workspaceID, v.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *WorkspaceReconciler) getValueFrom(ctx context.Context, instance *appv1alpha2.Workspace, valueFrom *appv1alpha2.ValueFrom) (string, error) {
	objectKey := types.NamespacedName{
		Namespace: instance.Namespace,
	}

	cm := valueFrom.ConfigMapKeyRef
	if cm != nil {
		objectKey.Name = cm.Name
		v, err := r.getConfigMap(ctx, objectKey)
		if err != nil {
			return "", err
		}
		if k, ok := v.Data[cm.Key]; ok {
			return string(k), nil
		}
		return "", fmt.Errorf("key %s not found in ConfigMap %s", cm.Key, cm.Name)
	}

	s := valueFrom.SecretKeyRef
	if s != nil {
		objectKey.Name = s.Name
		v, err := r.getSecret(ctx, objectKey)
		if err != nil {
			return "", err
		}
		if k, ok := v.Data[s.Key]; ok {
			return string(k), nil
		}
		return "", fmt.Errorf("key %s not found in Secret %s", s.Key, s.Name)
	}

	return "", nil
}

// getVariablesByCategory returns a map of all instance variables by type.
func (r *WorkspaceReconciler) getVariablesByCategory(ctx context.Context, w *workspaceInstance, category tfc.CategoryType, shouldFail bool) (map[string]tfc.Variable, error) {
	variables := make(map[string]tfc.Variable)
	var instanceVariables []appv1alpha2.Variable
	switch category {
	case tfc.CategoryEnv:
		instanceVariables = w.instance.Spec.EnvironmentVariables
	case tfc.CategoryTerraform:
		instanceVariables = w.instance.Spec.TerraformVariables
	}

	if len(instanceVariables) == 0 {
		return variables, nil
	}

	for _, v := range instanceVariables {
		var err error
		value := v.Value
		if v.ValueFrom != nil {
			value, err = r.getValueFrom(ctx, &w.instance, v.ValueFrom)
			if err != nil {
				w.log.Error(err, "Reconcile Variables", "msg", fmt.Sprintf("failed to get value for the variable %s", v.Name))
				r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "ReconcileVariables", "Failed to get value for a variable")
				if shouldFail {
					return nil, err
				} else {
					continue
				}
			}
		}
		variables[v.Name] = tfc.Variable{
			Key:         v.Name,
			Description: v.Description,
			Value:       value,
			HCL:         v.HCL,
			Sensitive:   v.Sensitive,
			Category:    category,
		}

	}

	return variables, nil
}

// getWorkspaceVariablesByCategory returns a map of all workspace variables by type.
func getWorkspaceVariablesByCategory(workspaceVariables []*tfc.Variable, category tfc.CategoryType) map[string]tfc.Variable {
	variables := make(map[string]tfc.Variable)

	for _, v := range workspaceVariables {
		if v.Category == category {
			variables[v.Key] = tfc.Variable{
				ID:          v.ID,
				Key:         v.Key,
				Description: v.Description,
				Value:       v.Value,
				HCL:         v.HCL,
				Sensitive:   v.Sensitive,
				Category:    category,
			}
		}
	}

	return variables
}

func (r *WorkspaceReconciler) reconcileVariablesByCategory(ctx context.Context, w *workspaceInstance, variables []*tfc.Variable, category tfc.CategoryType) error {
	workspaceID := w.instance.Status.WorkspaceID
	instanceVariables, _ := r.getVariablesByCategory(ctx, w, category, false)
	workspaceVariables := getWorkspaceVariablesByCategory(variables, category)

	daleteVariables := getVariablesToDelete(instanceVariables, workspaceVariables)
	if len(daleteVariables) > 0 {
		w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("deleting %d %s variables from the workspace ID %s", len(daleteVariables), category, workspaceID))
		err := r.deleteWorkspaceVariables(ctx, w, daleteVariables)
		if err != nil {
			return err
		}
	}

	createVariables := getVariablesToCreate(instanceVariables, workspaceVariables)
	if len(createVariables) > 0 {
		w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("creating %d new %s variables to the workspace ID %s", len(createVariables), category, workspaceID))
		err := r.createWorkspaceVariables(ctx, w, createVariables, category)
		if err != nil {
			return err
		}
	}

	updateVariables := getVariablesToUpdate(instanceVariables, workspaceVariables)
	if len(updateVariables) > 0 {
		w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("updating %d %s variables in the workspace ID %s", len(updateVariables), category, workspaceID))
		err := r.updateWorkspaceVariables(ctx, w, updateVariables)
		if err != nil {
			return err
		}
	}

	recreateVariables := getVariablesRequiringRecreate(instanceVariables, workspaceVariables)
	if len(recreateVariables) > 0 {
		w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("making %d %s variables no sensitive in the workspace ID %s", len(recreateVariables), category, workspaceID))
		err := r.updateWorkspaceSensitiveVariables(ctx, w, recreateVariables, category)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *WorkspaceReconciler) reconcileVariables(ctx context.Context, w *workspaceInstance, workspace *tfc.Workspace) error {
	w.log.Info("Reconcile Variables", "msg", "new reconciliation event")

	workspaceVariables, err := r.getWorkspaceVariables(ctx, w)
	if err != nil {
		return err
	}

	// Reconcilt Terraform Variables
	if err = r.reconcileVariablesByCategory(ctx, w, workspaceVariables, tfc.CategoryTerraform); err != nil {
		return err
	}

	// Reconcilt Environment Variables
	if err = r.reconcileVariablesByCategory(ctx, w, workspaceVariables, tfc.CategoryEnv); err != nil {
		return err
	}

	return nil
}
