// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"context"
	"fmt"

	tfc "github.com/hashicorp/go-tfe"
)

func (r *WorkspaceReconciler) getSSHKeyIDByName(ctx context.Context, w *workspaceInstance) (string, error) {
	SSHKeyName := w.instance.Spec.SSHKey.Name

	listOpts := &tfc.SSHKeyListOptions{}
	for {
		SSHKeys, err := w.tfClient.Client.SSHKeys.List(ctx, w.instance.Spec.Organization, listOpts)
		if err != nil {
			return "", err
		}
		for _, s := range SSHKeys.Items {
			if s.Name == SSHKeyName {
				return s.ID, nil
			}
		}
		if SSHKeys.NextPage == 0 {
			break
		}
		listOpts.PageNumber = SSHKeys.NextPage
	}

	return "", fmt.Errorf("ssh key ID was not found for ssh key name %q", SSHKeyName)
}

func (r *WorkspaceReconciler) getSSHKeyID(ctx context.Context, w *workspaceInstance) (string, error) {
	specSSHKey := w.instance.Spec.SSHKey

	if specSSHKey.Name != "" {
		w.log.Info("Reconcile SSH Key", "msg", "getting ssh key ID by name")
		return r.getSSHKeyIDByName(ctx, w)
	}

	w.log.Info("Reconcile SSH Key", "msg", "getting ssh key ID from the spec.sshKey.ID")
	return specSSHKey.ID, nil
}

func (r *WorkspaceReconciler) reconcileSSHKey(ctx context.Context, w *workspaceInstance, workspace *tfc.Workspace) error {
	if w.instance.Spec.SSHKey == nil {
		// Verify whether a Workspace has an SSH key and unassign it if so
		if workspace.SSHKey != nil {
			w.log.Info("Reconcile SSH Key", "msg", "unassigning the ssh key")
			_, err := w.tfClient.Client.Workspaces.UnassignSSHKey(ctx, workspace.ID)
			return err
		}

		return nil
	}

	SSHKeyID, err := r.getSSHKeyID(ctx, w)
	if err != nil {
		return err
	}

	// Assign an SSH key to a workspace if nothing is assigned
	if workspace.SSHKey == nil {
		w.log.Info("Reconcile SSH Key", "msg", "assigning the ssh key")
		_, err := w.tfClient.Client.Workspaces.AssignSSHKey(ctx, workspace.ID, tfc.WorkspaceAssignSSHKeyOptions{
			SSHKeyID: tfc.String(SSHKeyID),
		})
		return err
	}

	// Assign an SSH key to a workspace if it is different from the one in spec
	if workspace.SSHKey.ID != SSHKeyID {
		w.log.Info("Reconcile SSH Key", "msg", "assigning the ssh key")
		_, err := w.tfClient.Client.Workspaces.AssignSSHKey(ctx, workspace.ID, tfc.WorkspaceAssignSSHKeyOptions{
			SSHKeyID: tfc.String(SSHKeyID),
		})
		return err
	}

	return nil
}
