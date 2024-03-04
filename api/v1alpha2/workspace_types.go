// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package v1alpha2

import (
	tfc "github.com/hashicorp/go-tfe"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentPool allows Terraform Cloud to communicate with isolated, private, or on-premises infrastructure.
// Only one of the fields `ID` or `Name` is allowed.
// At least one of the fields `ID` or `Name` is mandatory.
// More information:
//   - https://developer.hashicorp.com/terraform/cloud-docs/agents
type WorkspaceAgentPool struct {
	// Agent Pool ID.
	// Must match pattern: `^apool-[a-zA-Z0-9]+$`
	//
	//+kubebuilder:validation:Pattern:="^apool-[a-zA-Z0-9]+$"
	//+optional
	ID string `json:"id,omitempty"`
	// Agent Pool name.
	//
	//+kubebuilder:validation:MinLength:=1
	//+optional
	Name string `json:"name,omitempty"`
}

// ConsumerWorkspace allows access to the state for specific workspaces within the same organization.
// Only one of the fields `ID` or `Name` is allowed.
// At least one of the fields `ID` or `Name` is mandatory.
// More information:
//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/state#remote-state-access-controls
type ConsumerWorkspace struct {
	// Consumer Workspace ID.
	// Must match pattern: `^ws-[a-zA-Z0-9]+$`
	//
	//+kubebuilder:validation:Pattern:="^ws-[a-zA-Z0-9]+$"
	//+optional
	ID string `json:"id,omitempty"`
	// Consumer Workspace name.
	//
	//+kubebuilder:validation:MinLength:=1
	//+optional
	Name string `json:"name,omitempty"`
}

// RemoteStateSharing allows remote state access between workspaces.
// By default, new workspaces in Terraform Cloud do not allow other workspaces to access their state.
// More information:
//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/state#accessing-state-from-other-workspaces
type RemoteStateSharing struct {
	// Allow access to the state for all workspaces within the same organization.
	// Default: `false`.
	//
	//+kubebuilder:default:=false
	//+optional
	AllWorkspaces bool `json:"allWorkspaces,omitempty"`
	// Allow access to the state for specific workspaces within the same organization.
	//
	//+kubebuilder:validation:MinItems:=1
	//+optional
	Workspaces []*ConsumerWorkspace `json:"workspaces,omitempty"`
}

// Run tasks allow Terraform Cloud to interact with external systems at specific points in the Terraform Cloud run lifecycle.
// Only one of the fields `ID` or `Name` is allowed.
// At least one of the fields `ID` or `Name` is mandatory.
// More information:
//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings/run-tasks
type WorkspaceRunTask struct {
	// Run Task ID.
	// Must match pattern: `^task-[a-zA-Z0-9]+$`
	//
	//+kubebuilder:validation:Pattern:="^task-[a-zA-Z0-9]+$"
	//+optional
	ID string `json:"id,omitempty"`
	// Run Task Name.
	//
	//+kubebuilder:validation:MinLength:=1
	//+optional
	Name string `json:"name,omitempty"`
	// Run Task Enforcement Level. Can be one of `advisory` or `mandatory`. Default: `advisory`.
	// Must be one of the following values: `advisory`, `mandatory`
	// Default: `advisory`.
	//
	//+kubebuilder:validation:Pattern:="^(advisory|mandatory)$"
	//+kubebuilder:default:=advisory
	//+optional
	EnforcementLevel string `json:"enforcementLevel"`
	// Run Task Stage.
	// Must be one of the following values: `pre_apply`, `pre_plan`, `post_plan`.
	// Default: `post_plan`.
	//
	//+kubebuilder:validation:Pattern:="^(pre_apply|pre_plan|post_plan)$"
	//+kubebuilder:default:=post_plan
	//+optional
	Stage string `json:"stage,omitempty"`
}

// RunTrigger allows you to connect this workspace to one or more source workspaces.
// These connections allow runs to queue automatically in this workspace on successful apply of runs in any of the source workspaces.
// Only one of the fields `ID` or `Name` is allowed.
// At least one of the fields `ID` or `Name` is mandatory.
// More information:
//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings/run-triggers
type RunTrigger struct {
	// Source Workspace ID.
	// Must match pattern: `^ws-[a-zA-Z0-9]+$`
	//
	//+kubebuilder:validation:Pattern:="^ws-[a-zA-Z0-9]+$"
	//+optional
	ID string `json:"id,omitempty"`
	// Source Workspace Name.
	//
	//+kubebuilder:validation:MinLength:=1
	//+optional
	Name string `json:"name,omitempty"`
}

// Teams are groups of Terraform Cloud users within an organization.
// If a user belongs to at least one team in an organization, they are considered a member of that organization.
// Only one of the fields `ID` or `Name` is allowed.
// At least one of the fields `ID` or `Name` is mandatory.
// More information:
//   - https://developer.hashicorp.com/terraform/cloud-docs/users-teams-organizations/teams
type Team struct {
	// Team ID.
	// Must match pattern: `^team-[a-zA-Z0-9]+$`
	//
	//+kubebuilder:validation:Pattern:="^team-[a-zA-Z0-9]+$"
	//+optional
	ID string `json:"id,omitempty"`
	// Team name.
	//
	//+kubebuilder:validation:MinLength:=1
	//+optional
	Name string `json:"name,omitempty"`
}

// Custom permissions let you assign specific, finer-grained permissions to a team than the broader fixed permission sets provide.
// More information:
//   - https://developer.hashicorp.com/terraform/cloud-docs/users-teams-organizations/permissions#custom-workspace-permissions
type CustomPermissions struct {
	// Run access.
	// Must be one of the following values: `apply`, `plan`, `read`.
	// Default: `read`.
	//
	//+kubebuilder:validation:Pattern:="^(apply|plan|read)$"
	//+kubebuilder:default:=read
	//+optional
	Runs string `json:"runs,omitempty"`
	// Manage Workspace Run Tasks.
	// Default: `false`.
	//
	//+kubebuilder:validation:default:=false
	//+optional
	RunTasks bool `json:"runTasks,omitempty"`
	// Download Sentinel mocks.
	// Must be one of the following values: `none`, `read`.
	// Default: `none`.
	//
	//+kubebuilder:validation:Pattern:="^(none|read)$"
	//+kubebuilder:default:=none
	//+optional
	Sentinel string `json:"sentinel,omitempty"`
	// State access.
	// Must be one of the following values: `none`, `read`, `read-outputs`, `write`.
	// Default: `none`.
	//
	//+kubebuilder:validation:Pattern:="^(none|read|read-outputs|write)$"
	//+kubebuilder:default:=none
	//+optional
	StateVersions string `json:"stateVersions,omitempty"`
	// Variable access.
	// Must be one of the following values: `none`, `read`, `write`.
	// Default: `none`.
	//
	//+kubebuilder:validation:Pattern:="^(none|read|write)$"
	//+kubebuilder:default:=none
	//+optional
	Variables string `json:"variables,omitempty"`
	// Lock/unlock workspace.
	// Default: `false`.
	//
	//+kubebuilder:default:=false
	//+optional
	WorkspaceLocking bool `json:"workspaceLocking,omitempty"`
}

// Terraform Cloud workspaces can only be accessed by users with the correct permissions.
// You can manage permissions for a workspace on a per-team basis.
// When a workspace is created, only the owners team and teams with the "manage workspaces" permission can access it,
// with full admin permissions. These teams' access can't be removed from a workspace.
// More information:
//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings/access
type TeamAccess struct {
	// Team to grant access.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/users-teams-organizations/teams
	Team Team `json:"team"`
	// There are two ways to choose which permissions a given team has on a workspace: fixed permission sets, and custom permissions.
	// Must be one of the following values: `admin`, `custom`, `plan`, `read`, `write`.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/users-teams-organizations/permissions#workspace-permissions
	//
	//+kubebuilder:validation:Pattern:="^(admin|custom|plan|read|write)$"
	Access string `json:"access"`
	// Custom permissions let you assign specific, finer-grained permissions to a team than the broader fixed permission sets provide.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/users-teams-organizations/permissions#custom-workspace-permissions
	//
	//+optional
	Custom CustomPermissions `json:"custom,omitempty"`
}

// Token refers to a Kubernetes Secret object within the same namespace as the Workspace object
type Token struct {
	// Selects a key of a secret in the workspace's namespace
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef"`
}

// ValueFrom source for the variable's value.
// Cannot be used if value is not empty.
type ValueFrom struct {
	// Selects a key of a ConfigMap.
	//
	//+optional
	ConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`
	// Selects a key of a Secret.
	//
	//+optional
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// Variables let you customize configurations, modify Terraform's behavior, and store information like provider credentials.
// More information:
//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/variables
type Variable struct {
	// Name of the variable.
	//
	//+kubebuilder:validation:MinLength:=1
	Name string `json:"name"`
	// Description of the variable.
	//
	//+kubebuilder:validation:MinLength:=1
	//+optional
	Description string `json:"description,omitempty"`
	// Parse this field as HashiCorp Configuration Language (HCL). This allows you to interpolate values at runtime.
	// Default: `false`.
	//
	//+kubebuilder:default:=false
	//+optional
	HCL bool `json:"hcl,omitempty"`
	// Sensitive variables are never shown in the UI or API.
	// They may appear in Terraform logs if your configuration is designed to output them.
	// Default: `false`.
	//
	//+kubebuilder:default:=false
	//+optional
	Sensitive bool `json:"sensitive,omitempty"`
	// Value of the variable.
	//
	//+kubebuilder:validation:MinLength:=1
	//+optional
	Value string `json:"value,omitempty"`
	// Source for the variable's value. Cannot be used if value is not empty.
	//
	//+optional
	ValueFrom *ValueFrom `json:"valueFrom,omitempty"`
}

// VersionControl settings for the workspace's VCS repository, enabling the UI/VCS-driven run workflow.
// Omit this argument to utilize the CLI-driven and API-driven workflows, where runs are not driven by webhooks on your VCS provider.
// More information:
//   - https://developer.hashicorp.com/terraform/cloud-docs/run/ui
//   - https://developer.hashicorp.com/terraform/cloud-docs/vcs
type VersionControl struct {
	// The VCS Connection (OAuth Connection + Token) to use.
	// Must match pattern: `^ot-[a-zA-Z0-9]+$`
	//
	//+kubebuilder:validation:Pattern:="^ot-[a-zA-Z0-9]+$"
	OAuthTokenID string `json:"oAuthTokenID,omitempty"`
	// A reference to your VCS repository in the format `<organization>/<repository>` where `<organization>` and `<repository>` refer to the organization and repository in your VCS provider.
	//
	//+kubebuilder:validation:MinLength:=1
	Repository string `json:"repository,omitempty"`
	// The repository branch that Run will execute from. This defaults to the repository's default branch (e.g. main).
	//
	//+kubebuilder:validation:MinLength:=1
	//+optional
	Branch string `json:"branch,omitempty"`
}

// SSH key used to clone Terraform modules.
// Only one of the fields `ID` or `Name` is allowed.
// At least one of the fields `ID` or `Name` is mandatory.
// More information:
//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings/ssh-keys
type SSHKey struct {
	// SSH key ID.
	// Must match pattern: `^sshkey-[a-zA-Z0-9]+$`
	//
	//+kubebuilder:validation:Pattern:="^sshkey-[a-zA-Z0-9]+$"
	//+optional
	ID string `json:"id,omitempty"`
	// SSH key name.
	//
	//+kubebuilder:validation:MinLength:=1
	//+optional
	Name string `json:"name,omitempty"`
}

// Tags allows you to correlate, organize, and even filter workspaces based on the assigned tags.
// Tags must be one or more characters; can include letters, numbers, colons, hyphens, and underscores; and must begin and end with a letter or number.
// Must match pattern: ^[A-Za-z0-9][A-Za-z0-9:_-]*$
//
// +kubebuilder:validation:Pattern:="^[A-Za-z0-9][A-Za-z0-9:_-]*$"
type Tag string

// NotificationTrigger represents the different TFC notifications that can be sent as a run's progress transitions between different states.
// This must be aligned with go-tfe type `NotificationTriggerType`.
// Must be one of the following values: `run:applying`, `assessment:check_failure`, `run:completed`, `run:created`, `assessment:drifted`, `run:errored`, `assessment:failed`, `run:needs_attention`, `run:planning`.
//
// +kubebuilder:validation:Enum:="run:applying";"assessment:check_failure";"run:completed";"run:created";"assessment:drifted";"run:errored";"assessment:failed";"run:needs_attention";"run:planning"
type NotificationTrigger string

// Notifications allow you to send messages to other applications based on run and workspace events.
// More information:
//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings/notifications
type Notification struct {
	// Notification name.
	//
	//+kubebuilder:validation:MinLength:=1
	Name string `json:"name"`
	// The type of the notification.
	// Must be one of the following values: `email`, `generic`, `microsoft-teams`, `slack`.
	//
	//+kubebuilder:validation:Enum:=email;generic;microsoft-teams;slack
	Type tfc.NotificationDestinationType `json:"type"`
	// Whether the notification configuration should be enabled or not.
	// Default: `true`.
	//
	//+kubebuilder:default=true
	//+optional
	Enabled bool `json:"enabled,omitempty"`
	// The token of the notification.
	//
	//+kubebuilder:validation:MinLength:=1
	//+optional
	Token string `json:"token,omitempty"`
	// The list of run events that will trigger notifications.
	// Trigger represents the different TFC notifications that can be sent as a run's progress transitions between different states.
	// There are two categories of triggers:
	//   - Health Events: `assessment:check_failure`, `assessment:drifted`, `assessment:failed`.
	//   - Run Events: `run:applying`, `run:completed`, `run:created`, `run:errored`, `run:needs_attention`, `run:planning`.
	//
	//+kubebuilder:validation:MinItems:=1
	//+optional
	Triggers []NotificationTrigger `json:"triggers,omitempty"`
	// The URL of the notification.
	// Must match pattern: `^https?://.*`
	//
	//+kubebuilder:validation:Pattern:="^https?://.*"
	//+optional
	URL string `json:"url,omitempty"`
	// The list of email addresses that will receive notification emails.
	// It is only available for Terraform Enterprise users. It is not available in Terraform Cloud.
	//
	//+kubebuilder:validation:MinItems:=1
	//+optional
	EmailAddresses []string `json:"emailAddresses,omitempty"`
	// The list of users belonging to the organization that will receive notification emails.
	//
	//+kubebuilder:validation:MinItems:=1
	//+optional
	EmailUsers []string `json:"emailUsers,omitempty"`
}

// Projects let you organize your workspaces into groups.
// Only one of the fields `ID` or `Name` is allowed.
// At least one of the fields `ID` or `Name` is mandatory.
// More information:
//   - https://developer.hashicorp.com/terraform/tutorials/cloud/projects
type WorkspaceProject struct {
	// Project ID.
	// Must match pattern: `^prj-[a-zA-Z0-9]+$`
	//
	//+kubebuilder:validation:Pattern:="^prj-[a-zA-Z0-9]+$"
	//+optional
	ID string `json:"id,omitempty"`
	// Project name.
	//
	//+kubebuilder:validation:MinLength:=1
	//+optional
	Name string `json:"name,omitempty"`
}

// WorkspaceSpec defines the desired state of Workspace.
type WorkspaceSpec struct {
	// Workspace name.
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

	// Define either change will be applied automatically(auto) or require an operator to confirm(manual).
	// Must be one of the following values: `auto`, `manual`.
	// Default: `manual`.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings#auto-apply-and-manual-apply
	//
	//+kubebuilder:validation:Pattern:="^(auto|manual)$"
	//+kubebuilder:default=manual
	//+optional
	ApplyMethod string `json:"applyMethod,omitempty"`
	// Allows a destroy plan to be created and applied.
	// Default: `true`.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings#destruction-and-deletion
	//
	//+kubebuilder:default=true
	//+optional
	AllowDestroyPlan bool `json:"allowDestroyPlan"`
	// Workspace description.
	//
	//+kubebuilder:validation:MinLength:=1
	//+optional
	Description string `json:"description,omitempty"`
	// Terraform Cloud Agents allow Terraform Cloud to communicate with isolated, private, or on-premises infrastructure.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/agents
	//
	//+optional
	AgentPool *WorkspaceAgentPool `json:"agentPool,omitempty"`
	// Define where the Terraform code will be executed.
	// Must be one of the following values: `agent`, `local`, `remote`.
	// Default: `remote`.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings#execution-mode
	//
	//+kubebuilder:validation:Pattern:="^(agent|local|remote)$"
	//+kubebuilder:default=remote
	//+optional
	ExecutionMode string `json:"executionMode,omitempty"`
	// Define whether the workspace should start a run when created
	// Default: `false`.
	//
	//+kubebuilder:default=false
	//+optional
	InitialRun bool `json:"initialRun"`
	// Define if the initial run should be an apply or plan.
	// Must be one of the following values: `apply`, `plan`.
	// Default: `plan`.
	//
	//+kubebuilder:validation:Pattern:="^(apply|plan)$"
	//+kubebuilder:default=plan
	//+optional
	InitialRunType string `json:"initialRunType"`
	// Run tasks allow Terraform Cloud to interact with external systems at specific points in the Terraform Cloud run lifecycle.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings/run-tasks
	//
	//+kubebuilder:validation:MinItems:=1
	//+optional
	RunTasks []WorkspaceRunTask `json:"runTasks,omitempty"`
	// Workspace tags are used to help identify and group together workspaces.
	// Tags must be one or more characters; can include letters, numbers, colons, hyphens, and underscores; and must begin and end with a letter or number.
	//
	//+kubebuilder:validation:MinItems:=1
	//+optional
	Tags []Tag `json:"tags,omitempty"`
	// Terraform Cloud workspaces can only be accessed by users with the correct permissions.
	// You can manage permissions for a workspace on a per-team basis.
	// When a workspace is created, only the owners team and teams with the "manage workspaces" permission can access it,
	// with full admin permissions. These teams' access can't be removed from a workspace.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings/access
	//
	//+kubebuilder:validation:MinItems:=1
	//+optional
	TeamAccess []*TeamAccess `json:"teamAccess,omitempty"`
	// The version of Terraform to use for this workspace.
	// If not specified, the latest available version will be used.
	// Must match pattern: `^\\d{1}\\.\\d{1,2}\\.\\d{1,2}$`
	// More information:
	//   - https://www.terraform.io/cloud-docs/workspaces/settings#terraform-version
	//
	//+kubebuilder:validation:Pattern:="^\\d{1}\\.\\d{1,2}\\.\\d{1,2}$"
	//+optional
	TerraformVersion string `json:"terraformVersion,omitempty"`
	// The directory where Terraform will execute, specified as a relative path from the root of the configuration directory.
	// More information:
	//   - https://www.terraform.io/cloud-docs/workspaces/settings#terraform-working-directory
	//
	//+kubebuilder:validation:MinLength:=1
	//+optional
	WorkingDirectory string `json:"workingDirectory,omitempty"`
	// Terraform Environment variables for all plans and applies in this workspace.
	// Variables defined within a workspace always overwrite variables from variable sets that have the same type and the same key.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/variables
	//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/variables#environment-variables
	//
	//+kubebuilder:validation:MinItems:=1
	//+optional
	EnvironmentVariables []Variable `json:"environmentVariables,omitempty"`
	// Terraform variables for all plans and applies in this workspace.
	// Variables defined within a workspace always overwrite variables from variable sets that have the same type and the same key.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/variables
	//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/variables#terraform-variables
	//
	//+kubebuilder:validation:MinItems:=1
	//+optional
	TerraformVariables []Variable `json:"terraformVariables,omitempty"`
	// Remote state access between workspaces.
	// By default, new workspaces in Terraform Cloud do not allow other workspaces to access their state.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/state#accessing-state-from-other-workspaces
	//
	//+optional
	RemoteStateSharing *RemoteStateSharing `json:"remoteStateSharing,omitempty"`
	// Run triggers allow you to connect this workspace to one or more source workspaces.
	// These connections allow runs to queue automatically in this workspace on successful apply of runs in any of the source workspaces.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings/run-triggers
	//
	//+kubebuilder:validation:MinItems:=1
	//+optional
	RunTriggers []RunTrigger `json:"runTriggers,omitempty"`
	// Settings for the workspace's VCS repository, enabling the UI/VCS-driven run workflow.
	// Omit this argument to utilize the CLI-driven and API-driven workflows, where runs are not driven by webhooks on your VCS provider.
	// More information:
	//   - https://www.terraform.io/cloud-docs/run/ui
	//   - https://www.terraform.io/cloud-docs/vcs
	//
	//+optional
	VersionControl *VersionControl `json:"versionControl,omitempty"`
	// SSH key used to clone Terraform modules.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings/ssh-keys
	//
	//+optional
	SSHKey *SSHKey `json:"sshKey,omitempty"`
	// Notifications allow you to send messages to other applications based on run and workspace events.
	// More information:
	//   - https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings/notifications
	//
	//+kubebuilder:validation:MinItems:=1
	//+optional
	Notifications []Notification `json:"notifications,omitempty"`
	// Projects let you organize your workspaces into groups.
	// Default: default organization project.
	// More information:
	//   - https://developer.hashicorp.com/terraform/tutorials/cloud/projects
	//
	//+optional
	Project *WorkspaceProject `json:"project,omitempty"`
}

type RunStatus struct {
	// Current(both active and finished) Terraform Cloud run ID.
	//
	//+optional
	ID string `json:"id,omitempty"`
	// Current(both active and finished) Terraform Cloud run status.
	//
	//+optional
	Status string `json:"status,omitempty"`
	//+optional
	ConfigurationVersion string `json:"configurationVersion,omitempty"`
	// Run ID of the latest run that could update the outputs.
	//
	//+optional
	OutputRunID string `json:"outputRunID,omitempty"`
}

// WorkspaceStatus defines the observed state of Workspace.
type WorkspaceStatus struct {
	// Workspace ID that is managed by the controller.
	WorkspaceID string `json:"workspaceID"`

	// Real world state generation.
	//
	//+optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Workspace last update timestamp.
	//
	//+optional
	UpdateAt int64 `json:"updateAt,omitempty"`
	// Workspace Runs status.
	//
	//+optional
	Run RunStatus `json:"runStatus,omitempty"`
	// Workspace Terraform version.
	//
	//+kubebuilder:validation:Pattern:="^\\d{1}\\.\\d{1,2}\\.\\d{1,2}$"
	//+optional
	TerraformVersion string `json:"terraformVersion,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Workspace ID",type=string,JSONPath=`.status.workspaceID`

// Workspace is the Schema for the workspaces API
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceSpec   `json:"spec"`
	Status WorkspaceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkspaceList contains a list of Workspace
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workspace{}, &WorkspaceList{})
}
