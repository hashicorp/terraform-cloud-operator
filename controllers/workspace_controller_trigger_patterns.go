// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	tfc "github.com/hashicorp/go-tfe"
	appv1alpha2 "github.com/hashicorp/terraform-cloud-operator/api/v1alpha2"
)

// HELPERS

// getTriggerPatterns return a map that maps consist of all trigger patterns defined in a object specification
// and values 'true' to simulate the Set structure
func getTriggerPatterns(instance *appv1alpha2.Workspace) map[string]bool {
	patterns := make(map[string]bool)

	if len(instance.Spec.TriggerPatterns) == 0 {
		return patterns
	}

	for _, t := range instance.Spec.TriggerPatterns {
		patterns[string(t)] = true
	}

	return patterns
}

// getWorkspaceTriggerPatterns return a map that maps consist of all trigger patterns assigned to workspace
// and values 'true' to simulate the Set structure
func getWorkspaceTriggerPatterns(workspace *tfc.Workspace) map[string]bool {
	patterns := make(map[string]bool)

	if len(workspace.TriggerPatterns) == 0 {
		return patterns
	}

	for _, t := range workspace.TriggerPatterns {
		patterns[t] = true
	}

	return patterns
}

// triggerPatternsDifference returns the list of trigger patterns that consists of the elements of leftTriggerPatterns
// which are not elements of rightTriggerPatterns
func triggerPatternsDifference(leftTriggerPatterns, rightTriggerPatterns map[string]bool) []string {
	var d []string

	for t := range leftTriggerPatterns {
		if !rightTriggerPatterns[t] {
			d = append(d, t)
		}
	}

	return d
}
