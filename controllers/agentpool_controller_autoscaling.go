// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	tfc "github.com/hashicorp/go-tfe"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	appv1alpha2 "github.com/hashicorp/terraform-cloud-operator/api/v1alpha2"
)

func computeRequiredAgents(ctx context.Context, ap *agentPoolInstance) (int32, error) {
	required := 0
	runStatuses := strings.Join([]string{
		string(tfc.RunPlanQueued),
		string(tfc.RunApplyQueued),
		string(tfc.RunApplying),
		string(tfc.RunPlanning),
	}, ",")
	// NOTE:
	// - Two maps are used here to simplify target workspace searching by ID, name, and wildcard.
	workspaceNames := map[string]struct{}{}
	workspaceIDs := map[string]struct{}{}

	pageNumber := 1
	for {
		workspaceList, err := ap.tfClient.Client.Workspaces.List(ctx, ap.instance.Spec.Organization, &tfc.WorkspaceListOptions{
			CurrentRunStatus: runStatuses,
			ListOptions: tfc.ListOptions{
				PageSize:   maxPageSize,
				PageNumber: pageNumber,
			},
		})
		if err != nil {
			return 0, err
		}
		for _, ws := range workspaceList.Items {
			if ws.AgentPool.ID == ap.instance.Status.AgentPoolID {
				workspaceNames[ws.Name] = struct{}{}
				workspaceIDs[ws.ID] = struct{}{}
			}
		}
		if workspaceList.NextPage == 0 {
			break
		}
		pageNumber = workspaceList.NextPage
	}

	if ap.instance.Spec.AgentDeploymentAutoscaling.TargetWorkspaces == nil {
		return int32(len(workspaceNames)), nil
	}

	for _, t := range *ap.instance.Spec.AgentDeploymentAutoscaling.TargetWorkspaces {
		switch {
		case t.Name != "":
			if _, ok := workspaceNames[t.Name]; ok {
				required++
				delete(workspaceNames, t.Name)
			}
		case t.ID != "":
			if _, ok := workspaceIDs[t.ID]; ok {
				required++
			}
		case t.WildcardName != "":
			prefix := strings.HasPrefix(t.WildcardName, "*")
			suffix := strings.HasSuffix(t.WildcardName, "*")
			wn := strings.Trim(t.WildcardName, "*")
			for w := range workspaceNames {
				match := false
				switch {
				case prefix && suffix:
					match = strings.Contains(w, wn)
				case prefix:
					match = strings.HasPrefix(w, wn)
				case suffix:
					match = strings.HasSuffix(w, wn)
				}
				if match {
					required++
					delete(workspaceNames, w)
				}
			}
		}
	}

	return int32(required), nil
}

func computeDesiredReplicas(requiredAgents, minReplicas, maxReplicas int32) int32 {
	if requiredAgents <= minReplicas {
		return minReplicas
	} else if requiredAgents >= maxReplicas {
		return maxReplicas
	}
	return requiredAgents
}

func getAgentDeploymentNamespacedName(ap *agentPoolInstance) types.NamespacedName {
	return types.NamespacedName{
		Namespace: ap.instance.Namespace,
		Name:      agentPoolDeploymentName(&ap.instance),
	}
}

func (r *AgentPoolReconciler) getAgentDeploymentReplicas(ctx context.Context, ap *agentPoolInstance) (int32, error) {
	deployment := appsv1.Deployment{}
	err := r.Client.Get(ctx, getAgentDeploymentNamespacedName(ap), &deployment)
	if err != nil {
		return 0, err
	}
	return *deployment.Spec.Replicas, nil
}

func (r *AgentPoolReconciler) scaleAgentDeployment(ctx context.Context, ap *agentPoolInstance, target *int32) error {
	deployment := appsv1.Deployment{}
	err := r.Client.Get(ctx, getAgentDeploymentNamespacedName(ap), &deployment)
	if err != nil {
		return err
	}
	deployment.Spec.Replicas = target
	return r.Client.Update(ctx, &deployment)
}

func (r *AgentPoolReconciler) reconcileAgentAutoscaling(ctx context.Context, ap *agentPoolInstance) error {
	if ap.instance.Spec.AgentDeploymentAutoscaling == nil {
		return nil
	}

	ap.log.Info("Reconcile Agent Autoscaling", "msg", "new reconciliation event")

	if s := ap.instance.Status.AgentDeploymentAutoscalingStatus; s != nil && s.LastScalingEvent != nil {
		lastScalingEventSeconds := int(time.Since(s.LastScalingEvent.Time).Seconds())
		cooldownPeriodSeconds := int(*ap.instance.Spec.AgentDeploymentAutoscaling.CooldownPeriodSeconds)
		if lastScalingEventSeconds <= cooldownPeriodSeconds {
			ap.log.Info("Reconcile Agent Autoscaling", "msg", "autoscaler is within the cooldown period, skipping")
			return nil
		}
	}

	requiredAgents, err := computeRequiredAgents(ctx, ap)
	if err != nil {
		ap.log.Error(err, "Reconcile Agent Autoscaling", "msg", "Failed to get agents needed")
		r.Recorder.Eventf(&ap.instance, corev1.EventTypeWarning, "AutoscaleAgentPoolDeployment", "Autoscaling failed: %v", err.Error())
		return err
	}
	ap.log.Info("Reconcile Agent Autoscaling", "msg", fmt.Sprintf("%d workspaces have pending runs", requiredAgents))

	currentReplicas, err := r.getAgentDeploymentReplicas(ctx, ap)
	if err != nil {
		ap.log.Error(err, "Reconcile Agent Autoscaling", "msg", "Failed to get current replicas")
		r.Recorder.Eventf(&ap.instance, corev1.EventTypeWarning, "AutoscaleAgentPoolDeployment", "Autoscaling failed: %v", err.Error())
		return err
	}

	minReplicas := *ap.instance.Spec.AgentDeploymentAutoscaling.MinReplicas
	maxReplicas := *ap.instance.Spec.AgentDeploymentAutoscaling.MaxReplicas
	desiredReplicas := computeDesiredReplicas(requiredAgents, minReplicas, maxReplicas)
	if desiredReplicas != currentReplicas {
		scalingEvent := fmt.Sprintf("Scaling agent deployment from %v to %v replicas", currentReplicas, desiredReplicas)
		ap.log.Info("Reconcile Agent Autoscaling", "msg", scalingEvent)
		r.Recorder.Event(&ap.instance, corev1.EventTypeNormal, "AutoscaleAgentPoolDeployment", scalingEvent)
		err := r.scaleAgentDeployment(ctx, ap, &desiredReplicas)
		if err != nil {
			ap.log.Error(err, "Reconcile Agent Autoscaling", "msg", "Failed to scale agent deployment")
			r.Recorder.Eventf(&ap.instance, corev1.EventTypeWarning, "AutoscaleAgentPoolDeployment", "Autoscaling failed: %v", err.Error())
			return err
		}
		ap.instance.Status.AgentDeploymentAutoscalingStatus = &appv1alpha2.AgentDeploymentAutoscalingStatus{
			DesiredReplicas: &desiredReplicas,
			LastScalingEvent: &metav1.Time{
				Time: time.Now(),
			},
		}
	}
	return nil
}
