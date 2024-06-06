// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"context"
	"fmt"

	tfc "github.com/hashicorp/go-tfe"
)

func (r *WorkspaceReconciler) getAgentPoolIDByName(ctx context.Context, w *workspaceInstance) (string, error) {
	agentPoolName := w.instance.Spec.AgentPool.Name

	listOpts := &tfc.AgentPoolListOptions{
		Query: agentPoolName,
	}
	for {
		agentPoolIDs, err := w.tfClient.Client.AgentPools.List(ctx, w.instance.Spec.Organization, listOpts)
		if err != nil {
			return "", err
		}
		for _, a := range agentPoolIDs.Items {
			if a.Name == agentPoolName {
				return a.ID, nil
			}
		}
		if agentPoolIDs.NextPage == 0 {
			break
		}
		listOpts.PageNumber = agentPoolIDs.NextPage
	}

	return "", fmt.Errorf("agent pool ID not found for agent pool name %q", agentPoolName)
}

func (r *WorkspaceReconciler) getAgentPoolID(ctx context.Context, w *workspaceInstance) (string, error) {
	specAgentPool := w.instance.Spec.AgentPool

	if specAgentPool == nil {
		return "", fmt.Errorf("'spec.agentPool' is not set")
	}

	if specAgentPool.Name != "" {
		w.log.Info("Reconcile Agent Pool", "msg", "getting agent pool ID by name")
		return r.getAgentPoolIDByName(ctx, w)
	}

	w.log.Info("Reconcile Agent Pool", "msg", "getting agent pool ID from the spec.AgentPool.ID")
	return specAgentPool.ID, nil
}
