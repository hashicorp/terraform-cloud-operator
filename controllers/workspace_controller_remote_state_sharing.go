// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"context"
	"errors"

	tfc "github.com/hashicorp/go-tfe"
	appv1alpha2 "github.com/hashicorp/terraform-cloud-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
)

func (r *WorkspaceReconciler) getWorkspaces(ctx context.Context, w *workspaceInstance) (map[string]string, error) {
	o := make(map[string]string)

	listOpts := &tfc.WorkspaceListOptions{}
	for {
		ws, err := w.tfClient.Client.Workspaces.List(ctx, w.instance.Spec.Organization, listOpts)
		if err != nil {
			return map[string]string{}, err
		}
		for _, w := range ws.Items {
			o[w.ID] = w.ID
			o[w.Name] = w.ID
		}
		if ws.NextPage == 0 {
			break
		}
		listOpts.PageNumber = ws.NextPage
	}

	return o, nil
}

// nameOrID returns Name or ID from the passed structure where only one of them is set
func nameOrID(instanceWorkspace *appv1alpha2.ConsumerWorkspace) string {
	if instanceWorkspace.Name != "" {
		return instanceWorkspace.Name
	}

	return instanceWorkspace.ID
}

func getWorkspaceID(workspaces map[string]string, instanceWorkspace *appv1alpha2.ConsumerWorkspace) (string, error) {
	k := nameOrID(instanceWorkspace)
	if v, ok := workspaces[k]; ok {
		return v, nil
	}
	return "", errors.New("workspace ID not found")
}

func (r *WorkspaceReconciler) getInstanceRemoteStateSharing(ctx context.Context, w *workspaceInstance) ([]*tfc.Workspace, error) {
	iw := []*tfc.Workspace{}

	if len(w.instance.Spec.RemoteStateSharing.Workspaces) == 0 {
		return iw, nil
	}

	workspaces, err := r.getWorkspaces(ctx, w)
	if err != nil {
		return iw, err
	}

	for _, workspace := range w.instance.Spec.RemoteStateSharing.Workspaces {
		wID, err := getWorkspaceID(workspaces, workspace)
		if err != nil {
			w.log.Error(err, "Reconcile Remote State Sharing", "msg", "failed to get workspace ID")
			r.Recorder.Event(&w.instance, corev1.EventTypeWarning, "ReconcileRemoteStateSharing", "Failed to get workspace ID")
			return iw, err
		}
		iw = append(iw, &tfc.Workspace{ID: wID})
	}

	return iw, nil
}

func (r *WorkspaceReconciler) reconcileRemoteStateSharing(ctx context.Context, w *workspaceInstance) error {
	w.log.Info("Reconcile Remote State Sharing", "msg", "new reconciliation event")

	if w.instance.Spec.RemoteStateSharing == nil {
		return nil
	}

	instanceRemoteStateSharing, err := r.getInstanceRemoteStateSharing(ctx, w)
	if err != nil {
		w.log.Error(err, "Reconcile Remote State Sharing", "msg", "failed to get instance remote state sharing workspace sources")
		return err
	}

	if len(instanceRemoteStateSharing) > 0 {
		err = w.tfClient.Client.Workspaces.UpdateRemoteStateConsumers(ctx, w.instance.Status.WorkspaceID, tfc.WorkspaceUpdateRemoteStateConsumersOptions{
			Workspaces: instanceRemoteStateSharing,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
