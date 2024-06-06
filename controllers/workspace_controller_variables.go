// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"

	tfc "github.com/hashicorp/go-tfe"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	appv1alpha2 "github.com/hashicorp/terraform-cloud-operator/api/v1alpha2"
)

// variableValueID calculates a hash of a variable.
func variableValueID(v tfc.Variable) string {
	hash := sha256.New()
	hv := appv1alpha2.Variable{
		Name:        v.Key,
		Description: v.Description,
		HCL:         v.HCL,
		Sensitive:   v.Sensitive,
		Value:       v.Value,
	}
	gob.NewEncoder(hash).Encode(hv)

	return hex.EncodeToString(hash.Sum(nil))
}

func createWorkspaceVariable(ctx context.Context, w *workspaceInstance, variable tfc.Variable) error {
	w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("creating %s variable %s", variable.Category, variable.Key))
	v, err := w.tfClient.Client.Variables.Create(ctx, w.instance.Status.WorkspaceID, tfc.VariableCreateOptions{
		Key:         &variable.Key,
		Value:       &variable.Value,
		Description: &variable.Description,
		Category:    &variable.Category,
		HCL:         &variable.HCL,
		Sensitive:   &variable.Sensitive,
	})
	if err != nil {
		w.log.Error(err, "Reconcile Variables", "msg", fmt.Sprintf("failed to create %s variable %s", variable.Category, variable.Key))
		return err
	}

	w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("successfully created %s variable %s", variable.Category, variable.Key))
	w.instance.Status.AddOrUpdateVariableStatus(appv1alpha2.VariableStatus{
		Name:      v.Key,
		ID:        v.ID,
		VersionID: v.VersionID,
		ValueID:   variableValueID(variable),
		Category:  string(v.Category),
	})

	return nil
}

func updateWorkspaceVariable(ctx context.Context, w *workspaceInstance, specVariable, workspaceVariable tfc.Variable) error {
	// If a workspace variable is marked as sensitive, it cannot be updated to become non-sensitive.
	// In such cases, the variable must be removed and a new one created.
	// This condition addresses the scenario where a variable is non-sensitive in the specification but sensitive in the workspace.
	// | spec.Sensitive  | workspace.Sensitive | AND |
	// |     0(!= 1)     |          0          |  0  |
	// |     0(!= 1)     |          1          |  1  |
	// |     1(!= 0)     |          0          |  0  |
	// |     1(!= 0)     |          1          |  0  |
	if !specVariable.Sensitive && workspaceVariable.Sensitive {
		w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("updating %s variable %s due to sensitivity changes", specVariable.Category, specVariable.Key))
		if err := deleteWorkspaceVariable(ctx, w, workspaceVariable); err != nil {
			return err
		}
		if err := createWorkspaceVariable(ctx, w, workspaceVariable); err != nil {
			return err
		}
		w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("successfully updated %s variable %s", specVariable.Category, specVariable.Key))

		return nil
	}

	vID := variableValueID(specVariable)
	statusVariable := w.instance.Status.GetVariableStatus(appv1alpha2.VariableStatus{
		Name:     specVariable.Key,
		Category: string(specVariable.Category),
	})
	// Update a variable if one of three conditions is true:
	// - A variable is not in Status, indicating that the variable is present in the workspace but not managed by the operator yet.
	// - A variable's Status and workspace VersionID do not match, indicating that the variable has been updated outside of the operator.
	// - A variable's Status and workspace ValueID do not match, indicating that the variable has been updated via the spec.
	if statusVariable == nil || statusVariable.VersionID != workspaceVariable.VersionID || statusVariable.ValueID != vID {
		w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("updating %s variable %s", specVariable.Category, specVariable.Key))
		v, err := w.tfClient.Client.Variables.Update(ctx, w.instance.Status.WorkspaceID, workspaceVariable.ID, tfc.VariableUpdateOptions{
			Key:         &specVariable.Key,
			Value:       &specVariable.Value,
			Description: &specVariable.Description,
			Category:    &specVariable.Category,
			HCL:         &specVariable.HCL,
			Sensitive:   &specVariable.Sensitive,
		})
		if err != nil {
			w.log.Error(err, "Reconcile Variables", "msg", fmt.Sprintf("failed to update %s variable %s", specVariable.Category, specVariable.Key))
			return err
		}

		w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("successfully updated %s variable %s", specVariable.Category, specVariable.Key))
		w.instance.Status.AddOrUpdateVariableStatus(appv1alpha2.VariableStatus{
			Name:      v.Key,
			ID:        v.ID,
			VersionID: v.VersionID,
			ValueID:   vID,
			Category:  string(v.Category),
		})
	}

	return nil
}

func deleteWorkspaceVariable(ctx context.Context, w *workspaceInstance, variable tfc.Variable) error {
	w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("deleteing %s variable %s", variable.Category, variable.Key))
	err := w.tfClient.Client.Variables.Delete(ctx, w.instance.Status.WorkspaceID, variable.ID)
	if err != nil {
		w.log.Error(err, "Reconcile Variables", "msg", fmt.Sprintf("failed to delete %s variable %s", variable.Category, variable.Key))
		return err
	}

	w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("successfully deleted %s variable %s", variable.Category, variable.Key))
	w.instance.Status.DeleteVariableStatus(appv1alpha2.VariableStatus{Name: variable.Key, Category: string(variable.Category)})

	return nil
}

// getWorkspaceVariables returns a list of all variables associated with the workspace.
func (r *WorkspaceReconciler) getWorkspaceVariables(ctx context.Context, w *workspaceInstance, workspace *tfc.Workspace) ([]*tfc.Variable, error) {
	if workspace.Variables == nil {
		return nil, nil
	}
	var o []*tfc.Variable

	w.log.Info("Reconcile Variables", "msg", "getting workspace variables")
	listOpts := &tfc.VariableListOptions{}
	for {
		v, err := w.tfClient.Client.Variables.List(ctx, w.instance.Status.WorkspaceID, listOpts)
		if err != nil {
			w.log.Error(err, "Reconcile Variables", "msg", "failed to get workspace variables")
			return nil, err
		}
		o = append(o, v.Items...)
		if v.NextPage == 0 {
			break
		}
		listOpts.PageNumber = v.NextPage
	}
	w.log.Info("Reconcile Variables", "msg", "successfully got workspace variables")

	return o, nil
}

// getVariablesByCategory returns a map of all instance variables by type.
func (r *WorkspaceReconciler) getVariablesByCategory(ctx context.Context, w *workspaceInstance, category tfc.CategoryType) (map[string]tfc.Variable, error) {
	variables := make(map[string]tfc.Variable)
	specVariables := w.instance.Spec.TerraformVariables
	if category == tfc.CategoryEnv {
		specVariables = w.instance.Spec.EnvironmentVariables
	}

	if len(specVariables) == 0 {
		return variables, nil
	}

	for _, v := range specVariables {
		value := v.Value
		if v.ValueFrom != nil {
			var err error
			objectKey := types.NamespacedName{
				Namespace: w.instance.Namespace,
			}
			if cm := v.ValueFrom.ConfigMapKeyRef; cm != nil {
				objectKey.Name = cm.Name
				value, err = configMapKeyRef(ctx, r.Client, objectKey, cm.Key)
			}
			if s := v.ValueFrom.SecretKeyRef; s != nil {
				objectKey.Name = s.Name
				value, err = secretKeyRef(ctx, r.Client, objectKey, s.Key)
			}
			if err != nil {
				w.log.Error(err, "Reconcile Variables", "msg", fmt.Sprintf("failed to get value for the variable %s", v.Name))
				r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "ReconcileVariables", fmt.Sprintf("Failed to get value for the variable %s", v.Name))
				return nil, err
			}
		}
		variables[v.Name] = tfc.Variable{
			Key:         v.Name,
			Value:       value,
			Description: v.Description,
			Category:    category,
			HCL:         v.HCL,
			Sensitive:   v.Sensitive,
		}

	}

	return variables, nil
}

// getWorkspaceVariablesByCategory returns a map of all workspace variables by type.
func getWorkspaceVariablesByCategory(workspaceVariables []*tfc.Variable, category tfc.CategoryType) map[string]tfc.Variable {
	variables := make(map[string]tfc.Variable)

	for _, v := range workspaceVariables {
		if v.Category == category {
			variables[v.Key] = *v
		}
	}

	return variables
}

func (r *WorkspaceReconciler) reconcileVariablesByCategory(ctx context.Context, w *workspaceInstance, variables []*tfc.Variable, category tfc.CategoryType) error {
	workspaceVariables := getWorkspaceVariablesByCategory(variables, category)
	specVariables, err := r.getVariablesByCategory(ctx, w, category)
	if err != nil {
		return err
	}

	if len(specVariables) == 0 && len(workspaceVariables) == 0 {
		w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("there are no %s variables both in spec and workspace", category))
		return nil
	}

	w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("there are %d %s variables in spec", len(specVariables), category))
	w.log.Info("Reconcile Variables", "msg", fmt.Sprintf("there are %d %s variables in workspace", len(workspaceVariables), category))

	// Let's consider spec and workspace variables as two sets.
	// The comparison of these sets allows us to identify variables needing creation, deletion, or updating:
	// - The set difference of spec and workspace gives variables for creation.
	// - The set difference of workspace and spec gives variables for deletion.
	// - Intersection of spec and workspace sets gives variables for updating.
	// Iterating over the spec set and comparing it with the workspace set provides creation and update candidates.
	// Updated variables are removed from the workspace set, leaving only deletion candidates.
	for sk, sv := range specVariables {
		if wv, ok := workspaceVariables[sk]; ok {
			if err := updateWorkspaceVariable(ctx, w, sv, wv); err != nil {
				return err
			}
			delete(workspaceVariables, sk)
		} else {
			if err := createWorkspaceVariable(ctx, w, sv); err != nil {
				return err
			}
		}
	}

	for _, wv := range workspaceVariables {
		if err := deleteWorkspaceVariable(ctx, w, wv); err != nil {
			return err
		}
	}

	return nil
}

func (r *WorkspaceReconciler) reconcileVariables(ctx context.Context, w *workspaceInstance, workspace *tfc.Workspace) error {
	w.log.Info("Reconcile Variables", "msg", "new reconciliation event")

	workspaceVariables, err := r.getWorkspaceVariables(ctx, w, workspace)
	if err != nil {
		return err
	}

	if err = r.reconcileVariablesByCategory(ctx, w, workspaceVariables, tfc.CategoryTerraform); err != nil {
		return err
	}

	if err = r.reconcileVariablesByCategory(ctx, w, workspaceVariables, tfc.CategoryEnv); err != nil {
		return err
	}

	return nil
}
