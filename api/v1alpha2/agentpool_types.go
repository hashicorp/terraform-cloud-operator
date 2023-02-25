// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package v1alpha2

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Agent Token is a secret token that a Terraform Cloud Agent is used to connect to the Terraform Cloud Agent Pool.
// In `spec` only the field `Name` is allowed, the rest are used in `status`.
// More infromation:
//   - https://developer.hashicorp.com/terraform/cloud-docs/agents
type AgentToken struct {
	// Agent Token name.
	//
	//+kubebuilder:validation:MinLength:=1
	Name string `json:"name"`
	// Agent Token ID.
	//
	//+kubebuilder:validation:Pattern="^at-[a-zA-Z0-9]+$"
	//+optional
	ID string `json:"id,omitempty"`
	// Timestamp of when the agent token was created.
	//
	//+optional
	CreatedAt *int64 `json:"createdAt,omitempty"`
	// Timestamp of when the agent token was last used.
	//
	//+optional
	LastUsedAt *int64 `json:"lastUsedAt,omitempty"`
}

type AgentDeployment struct {
	Replicas *int32     `json:"replicas,omitempty"`
	Spec     v1.PodSpec `json:"spec"`
}

// AgentPoolSpec defines the desired state of AgentPool.
type AgentPoolSpec struct {
	// Agent Pool name.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/agents/agent-pools
	//
	//+kubebuilder:validation:MinLength:=1
	Name string `json:"name"`
	// Organization name where the Workspace will be created.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/users-teams-organizations/organizations
	//
	//+kubebuilder:validation:MinLength:=1
	Organization string `json:"organization"`
	// API Token to be used for API calls.
	Token Token `json:"token"`

	// List of the agent tokens to generate.
	//
	//+kubebuilder:validation:MinItems:=1
	//+optional
	AgentTokens []*AgentToken `json:"agentTokens,omitempty"`

	// Agent deployment settings
	//+optional
	AgentDeployment *AgentDeployment `json:"agentDeployment,omitempty"`
}

// AgentPoolStatus defines the observed state of AgentPool.
type AgentPoolStatus struct {
	// Real world state generation.
	ObservedGeneration int64 `json:"observedGeneration"`
	// Agent Pool ID that is managed by the controller.
	AgentPoolID string `json:"agentPoolID"`
	// List of the agent tokens generated by the controller.
	//
	//+optional
	AgentTokens []*AgentToken `json:"agentTokens,omitempty"`
	// Name of the agent deployment generated by the controller.
	//
	//+optional
	AgentDeploymentName string `json:"agentDeploymentName,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AgentPool is the Schema for the agentpools API.
type AgentPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentPoolSpec   `json:"spec"`
	Status AgentPoolStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AgentPoolList contains a list of AgentPool.
type AgentPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentPool{}, &AgentPoolList{})
}
