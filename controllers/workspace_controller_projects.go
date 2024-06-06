// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"context"
	"fmt"

	tfc "github.com/hashicorp/go-tfe"
)

func (r *WorkspaceReconciler) getProjectIDByName(ctx context.Context, w *workspaceInstance) (string, error) {
	projectName := w.instance.Spec.Project.Name

	listOpts := &tfc.ProjectListOptions{
		Name: projectName,
	}
	for {
		projectIDs, err := w.tfClient.Client.Projects.List(ctx, w.instance.Spec.Organization, listOpts)
		if err != nil {
			return "", err
		}
		for _, p := range projectIDs.Items {
			if p.Name == projectName {
				return p.ID, nil
			}
		}
		if projectIDs.NextPage == 0 {
			break
		}
		listOpts.PageNumber = projectIDs.NextPage
	}

	return "", fmt.Errorf("project ID not found for project name %q", projectName)
}

func (r *WorkspaceReconciler) getProjectID(ctx context.Context, w *workspaceInstance) (string, error) {
	specProject := w.instance.Spec.Project

	if specProject == nil {
		return "", fmt.Errorf("'spec.Project' is not set")
	}

	if specProject.Name != "" {
		w.log.Info("Reconcile Project", "msg", "getting project ID by name")
		return r.getProjectIDByName(ctx, w)
	}

	w.log.Info("Reconcile Project", "msg", "getting project ID from the spec.Project.ID")
	return specProject.ID, nil
}
