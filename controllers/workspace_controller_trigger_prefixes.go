// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	tfc "github.com/hashicorp/go-tfe"
	appv1alpha2 "github.com/hashicorp/terraform-cloud-operator/api/v1alpha2"
)

// HELPERS

// getTriggerPrefixes return a map that maps consist of all trigger prefixes defined in a object specification
// and values 'true' to simulate the Set structure
func getTriggerPrefixes(instance *appv1alpha2.Workspace) map[string]bool {
	prefixes := make(map[string]bool)

	if len(instance.Spec.TriggerPrefixes) == 0 {
		return prefixes
	}

	for _, t := range instance.Spec.TriggerPrefixes {
		prefixes[string(t)] = true
	}

	return prefixes
}

// getWorkspaceTriggerPrefixes return a map that maps consist of all trigger prefixes assigned to workspace
// and values 'true' to simulate the Set structure
func getWorkspaceTriggerPrefixes(workspace *tfc.Workspace) map[string]bool {
	prefixes := make(map[string]bool)

	if len(workspace.TriggerPrefixes) == 0 {
		return prefixes
	}

	for _, t := range workspace.TriggerPrefixes {
		prefixes[t] = true
	}

	return prefixes
}

// triggerPrefixesDifference returns the list of trigger prefixes that consists of the elements of leftTriggerPrefixes
// which are not elements of rightTriggerPrefixes
func triggerPrefixesDifference(leftTriggerPrefixes, rightTriggerPrefixes map[string]bool) []string {
	var d []string

	for t := range leftTriggerPrefixes {
		if !rightTriggerPrefixes[t] {
			d = append(d, t)
		}
	}

	return d
}
