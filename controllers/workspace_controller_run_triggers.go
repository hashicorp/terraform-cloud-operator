// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"context"
	"fmt"

	tfc "github.com/hashicorp/go-tfe"
)

func (r *WorkspaceReconciler) getRunTriggersSources(ctx context.Context, w *workspaceInstance) (map[string]string, error) {
	runTriggersIDs := make(map[string]string)
	runTriggersNames := []string{}

	for _, rt := range w.instance.Spec.RunTriggers {
		if rt.Name != "" {
			runTriggersNames = append(runTriggersNames, rt.Name)
		}
		if rt.ID != "" {
			runTriggersIDs[rt.ID] = ""
		}
	}

	for _, workspaceName := range runTriggersNames {
		ws, err := w.tfClient.Client.Workspaces.Read(ctx, w.instance.Spec.Organization, workspaceName)
		if err == tfc.ErrResourceNotFound {
			return nil, fmt.Errorf("cannot find ID for Workspace %s", workspaceName)
		}
		if err != nil {
			return nil, err
		}
		runTriggersIDs[ws.ID] = ""
	}

	return runTriggersIDs, nil
}

func (r *WorkspaceReconciler) getRunTriggersWorkspace(ctx context.Context, w *workspaceInstance) (map[string]string, error) {
	rrt := make(map[string]string)
	listOpts := &tfc.RunTriggerListOptions{
		RunTriggerType: tfc.RunTriggerInbound,
		Include:        []tfc.RunTriggerIncludeOpt{tfc.RunTriggerSourceable},
	}
	for {
		runTriggers, err := w.tfClient.Client.RunTriggers.List(ctx, w.instance.Status.WorkspaceID, listOpts)
		if err != nil {
			return nil, err
		}
		for _, rt := range runTriggers.Items {
			// Sourceable.ID is an ID of the sourceable Workspace
			rrt[rt.Sourceable.ID] = rt.ID
		}
		if runTriggers.NextPage == 0 {
			break
		}
		listOpts.PageNumber = runTriggers.NextPage
	}

	return rrt, nil
}

func getWorkspacesToAdd(instanceWorkspaces, runTriggersWorkspaces map[string]string) map[string]string {
	return workspaceDifference(instanceWorkspaces, runTriggersWorkspaces)
}

func getWorkspacesToDelete(instanceWorkspaces, runTriggersWorkspaces map[string]string) map[string]string {
	return workspaceDifference(runTriggersWorkspaces, instanceWorkspaces)
}

func workspaceDifference(leftWorkspace, rightWorkspace map[string]string) map[string]string {
	d := make(map[string]string)

	for kl, vl := range leftWorkspace {
		if _, ok := rightWorkspace[kl]; !ok {
			d[kl] = vl
		}
	}

	return d
}

func (r *WorkspaceReconciler) addRunTriggers(ctx context.Context, w *workspaceInstance, workspaces map[string]string) error {
	if len(workspaces) == 0 {
		return nil
	}

	for workspace := range workspaces {
		_, err := w.tfClient.Client.RunTriggers.Create(ctx, w.instance.Status.WorkspaceID, tfc.RunTriggerCreateOptions{
			Sourceable: &tfc.Workspace{ID: workspace},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *WorkspaceReconciler) removeRunTriggers(ctx context.Context, w *workspaceInstance, workspaces map[string]string) error {
	if len(workspaces) == 0 {
		return nil
	}

	for _, workspace := range workspaces {
		err := w.tfClient.Client.RunTriggers.Delete(ctx, workspace)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *WorkspaceReconciler) reconcileRunTriggers(ctx context.Context, w *workspaceInstance) error {
	w.log.Info("Reconcile Run Triggers", "msg", "new reconciliation event")

	instanceRunTriggers, err := r.getRunTriggersSources(ctx, w)
	if err != nil {
		return nil
	}
	workspaceRunTriggers, err := r.getRunTriggersWorkspace(ctx, w)
	if err != nil {
		return nil
	}

	addWorkspaces := getWorkspacesToAdd(instanceRunTriggers, workspaceRunTriggers)
	if len(addWorkspaces) > 0 {
		w.log.Info("Reconcile Run Triggers", "msg", "adding run triggers workspaces to the workspace")
		err = r.addRunTriggers(ctx, w, addWorkspaces)
		if err != nil {
			return err
		}
	}

	deleteWorkspaces := getWorkspacesToDelete(instanceRunTriggers, workspaceRunTriggers)
	if len(deleteWorkspaces) > 0 {
		w.log.Info("Reconcile Run Triggers", "msg", "deleting run triggers workspaces from the workspace")
		err = r.removeRunTriggers(ctx, w, deleteWorkspaces)
		if err != nil {
			return err
		}
	}

	return nil
}
