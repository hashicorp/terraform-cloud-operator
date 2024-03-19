// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package v1alpha2

import (
	"testing"

	tfc "github.com/hashicorp/go-tfe"
)

func TestValidateWorkspaceSpecAgentPool(t *testing.T) {
	t.Parallel()

	successCases := map[string]Workspace{
		"HasOnlyID": {
			Spec: WorkspaceSpec{
				AgentPool: &WorkspaceAgentPool{
					ID: "this",
				},
			},
		},
		"HasOnlyName": {
			Spec: WorkspaceSpec{
				AgentPool: &WorkspaceAgentPool{
					Name: "this",
				},
			},
		},
	}

	for n, c := range successCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecAgentPool(); len(errs) != 0 {
				t.Errorf("Unexpected validation errors: %v", errs)
			}
		})
	}

	errorCases := map[string]Workspace{
		"HasIDandName": {
			Spec: WorkspaceSpec{
				AgentPool: &WorkspaceAgentPool{
					ID:   "this",
					Name: "this",
				},
			},
		},
		"HasEmptyIDandName": {
			Spec: WorkspaceSpec{
				AgentPool: &WorkspaceAgentPool{},
			},
		},
	}

	for n, c := range errorCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecAgentPool(); len(errs) == 0 {
				t.Error("Unexpected failure, at least one error is expected")
			}
		})
	}
}

func TestValidateWorkspaceSpecExecutionMode(t *testing.T) {
	t.Parallel()

	successCases := map[string]Workspace{
		"AgentWithAgentPoolWithID": {
			Spec: WorkspaceSpec{
				ExecutionMode: "agent",
				AgentPool: &WorkspaceAgentPool{
					ID: "this",
				},
			},
		},
		"AgentWithAgentPoolWithName": {
			Spec: WorkspaceSpec{
				ExecutionMode: "agent",
				AgentPool: &WorkspaceAgentPool{
					Name: "this",
				},
			},
		},
	}

	for n, c := range successCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecExecutionMode(); len(errs) != 0 {
				t.Errorf("Unexpected validation errors: %v", errs)
			}
		})
	}

	errorCases := map[string]Workspace{
		"AgentWithoutAgentPool": {
			Spec: WorkspaceSpec{
				ExecutionMode: "agent",
			},
		},
	}

	for n, c := range errorCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecExecutionMode(); len(errs) == 0 {
				t.Error("Unexpected failure, at least one error is expected")
			}
		})
	}
}

func TestValidateWorkspaceSpecNotifications(t *testing.T) {
	t.Parallel()

	token := "token"
	url := "https://example.com"
	successCases := map[string]Workspace{
		"OnlyEmailAddresses": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name: "this",
						Type: tfc.NotificationDestinationTypeEmail,
						EmailAddresses: []string{
							"user@mail.com",
						},
					},
				},
			},
		},
		"OnlyEmailUsers": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name: "this",
						Type: tfc.NotificationDestinationTypeEmail,
						EmailUsers: []string{
							"user@mail.com",
						},
					},
				},
			},
		},
		"EmailAddressesAndEmailUsers": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name: "this",
						Type: tfc.NotificationDestinationTypeEmail,
						EmailAddresses: []string{
							"user@mail.com",
						},
						EmailUsers: []string{
							"user@mail.com",
						},
					},
				},
			},
		},
		"OnlyGeneric": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name:  "this",
						Type:  tfc.NotificationDestinationTypeGeneric,
						Token: token,
						URL:   url,
					},
				},
			},
		},
		"OnlyMicrosoftTeams": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name: "this",
						Type: tfc.NotificationDestinationTypeMicrosoftTeams,
						URL:  url,
					},
				},
			},
		},
		"OnlySlack": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name: "this",
						Type: tfc.NotificationDestinationTypeSlack,
						URL:  url,
					},
				},
			},
		},
		"AllTypes": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name: "thisA",
						Type: tfc.NotificationDestinationTypeEmail,
						EmailAddresses: []string{
							"user@mail.com",
						},
					},
					{
						Name: "thisB",
						Type: tfc.NotificationDestinationTypeEmail,
						EmailUsers: []string{
							"user@mail.com",
						},
					},
					{
						Name: "thisC",
						Type: tfc.NotificationDestinationTypeEmail,
						EmailAddresses: []string{
							"user@mail.com",
						},
						EmailUsers: []string{
							"user@mail.com",
						},
					},
					{
						Name:  "thisD",
						Type:  tfc.NotificationDestinationTypeGeneric,
						Token: token,
						URL:   url,
					},
					{
						Name: "thisE",
						Type: tfc.NotificationDestinationTypeMicrosoftTeams,
						URL:  url,
					},
					{
						Name: "thisF",
						Type: tfc.NotificationDestinationTypeSlack,
						URL:  url,
					},
				},
			},
		},
	}

	for n, c := range successCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecNotifications(); len(errs) != 0 {
				t.Errorf("Unexpected validation errors: %v", errs)
			}
		})
	}

	errorCases := map[string]Workspace{
		"EmailWithoutEmails": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name: "this",
						Type: tfc.NotificationDestinationTypeEmail,
					},
				},
			},
		},
		"EmailWithUrl": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name: "this",
						Type: tfc.NotificationDestinationTypeEmail,
						URL:  url,
					},
				},
			},
		},
		"EmailWithToken": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name:  "this",
						Type:  tfc.NotificationDestinationTypeEmail,
						Token: token,
					},
				},
			},
		},
		"GenericWithoutToken": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name: "this",
						Type: tfc.NotificationDestinationTypeGeneric,
						URL:  url,
					},
				},
			},
		},
		"GenericWithoutURL": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name:  "this",
						Type:  tfc.NotificationDestinationTypeGeneric,
						Token: token,
					},
				},
			},
		},
		"GenericWithEmailAddresses": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name:  "this",
						Type:  tfc.NotificationDestinationTypeGeneric,
						Token: token,
						URL:   url,
						EmailAddresses: []string{
							"user@mail.com",
						},
					},
				},
			},
		},
		"GenericWithEmailUsers": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name:  "this",
						Type:  tfc.NotificationDestinationTypeGeneric,
						Token: token,
						URL:   url,
						EmailUsers: []string{
							"user@mail.com",
						},
					},
				},
			},
		},
		"MicrosoftTeamsWithToken": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name:  "this",
						Type:  tfc.NotificationDestinationTypeMicrosoftTeams,
						URL:   url,
						Token: token,
					},
				},
			},
		},
		"MicrosoftTeamsWithoutURL": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name: "this",
						Type: tfc.NotificationDestinationTypeMicrosoftTeams,
					},
				},
			},
		},
		"MicrosoftTeamsWithEmailAddresses": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name:  "this",
						Type:  tfc.NotificationDestinationTypeMicrosoftTeams,
						Token: token,
						URL:   url,
						EmailAddresses: []string{
							"user@mail.com",
						},
					},
				},
			},
		},
		"MicrosoftTeamsWithEmailUsers": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name:  "this",
						Type:  tfc.NotificationDestinationTypeMicrosoftTeams,
						Token: token,
						URL:   url,
						EmailUsers: []string{
							"user@mail.com",
						},
					},
				},
			},
		},
		"SlackWithToken": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name:  "this",
						Type:  tfc.NotificationDestinationTypeMicrosoftTeams,
						URL:   url,
						Token: token,
					},
				},
			},
		},
		"SlackWithoutURL": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name: "this",
						Type: tfc.NotificationDestinationTypeSlack,
					},
				},
			},
		},
		"SlackWithEmailAddresses": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name:  "this",
						Type:  tfc.NotificationDestinationTypeMicrosoftTeams,
						Token: token,
						URL:   url,
						EmailAddresses: []string{
							"user@mail.com",
						},
					},
				},
			},
		},
		"SlackWithEmailUsers": {
			Spec: WorkspaceSpec{
				Notifications: []Notification{
					{
						Name:  "this",
						Type:  tfc.NotificationDestinationTypeMicrosoftTeams,
						Token: token,
						URL:   url,
						EmailUsers: []string{
							"user@mail.com",
						},
					},
				},
			},
		},
	}

	for n, c := range errorCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecNotifications(); len(errs) == 0 {
				t.Error("Unexpected failure, at least one error is expected")
			}
		})
	}
}

func TestValidateWorkspaceSpecRemoteStateSharing(t *testing.T) {
	t.Parallel()

	successCases := map[string]Workspace{
		"HasOnlyAllWorkspaces": {
			Spec: WorkspaceSpec{
				RemoteStateSharing: &RemoteStateSharing{
					AllWorkspaces: true,
				},
			},
		},
		"HasOnlyWorkspacesWithID": {
			Spec: WorkspaceSpec{
				RemoteStateSharing: &RemoteStateSharing{
					Workspaces: []*ConsumerWorkspace{
						{
							ID: "this",
						},
						{
							ID: "self",
						},
					},
				},
			},
		},
		"HasOnlyWorkspacesWithName": {
			Spec: WorkspaceSpec{
				RemoteStateSharing: &RemoteStateSharing{
					Workspaces: []*ConsumerWorkspace{
						{
							Name: "this",
						},
						{
							Name: "self",
						},
					},
				},
			},
		},
		"HasOnlyWorkspacesWithIDandName": {
			Spec: WorkspaceSpec{
				RemoteStateSharing: &RemoteStateSharing{
					Workspaces: []*ConsumerWorkspace{
						{
							ID: "this",
						},
						{
							Name: "self",
						},
					},
				},
			},
		},
	}

	for n, c := range successCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecRemoteStateSharing(); len(errs) != 0 {
				t.Errorf("Unexpected validation errors: %v", errs)
			}
		})
	}

	errorCases := map[string]Workspace{
		"HasAllWorkspacesAndWorkspacesWithID": {
			Spec: WorkspaceSpec{
				RemoteStateSharing: &RemoteStateSharing{
					AllWorkspaces: true,
					Workspaces: []*ConsumerWorkspace{
						{
							ID: "this",
						},
						{
							ID: "self",
						},
					},
				},
			},
		},
		"HasAllWorkspacesAndWorkspacesWithName": {
			Spec: WorkspaceSpec{
				RemoteStateSharing: &RemoteStateSharing{
					AllWorkspaces: true,
					Workspaces: []*ConsumerWorkspace{
						{
							Name: "this",
						},
						{
							Name: "self",
						},
					},
				},
			},
		},
		"HasAllWorkspacesAndWorkspacesWithIDandName": {
			Spec: WorkspaceSpec{
				RemoteStateSharing: &RemoteStateSharing{
					AllWorkspaces: true,
					Workspaces: []*ConsumerWorkspace{
						{
							ID: "this",
						},
						{
							Name: "self",
						},
					},
				},
			},
		},
		"HasEmptyAllWorkspacesAndWorkspaces": {
			Spec: WorkspaceSpec{
				RemoteStateSharing: &RemoteStateSharing{},
			},
		},
		"HasDuplicateWorkspacesName": {
			Spec: WorkspaceSpec{
				RemoteStateSharing: &RemoteStateSharing{
					Workspaces: []*ConsumerWorkspace{
						{
							Name: "this",
						},
						{
							Name: "this",
						},
					},
				},
			},
		},
		"HasDuplicateWorkspacesID": {
			Spec: WorkspaceSpec{
				RemoteStateSharing: &RemoteStateSharing{
					Workspaces: []*ConsumerWorkspace{
						{
							ID: "this",
						},
						{
							ID: "this",
						},
					},
				},
			},
		},
	}

	for n, c := range errorCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecRemoteStateSharing(); len(errs) == 0 {
				// fmt.Println(errs)
				t.Error("Unexpected failure, at least one error is expected")
			}
		})
	}
}

func TestValidateWorkspaceSpecRunTasks(t *testing.T) {
	t.Parallel()

	successCases := map[string]Workspace{
		"HasOnlyID": {
			Spec: WorkspaceSpec{
				RunTasks: []WorkspaceRunTask{
					{
						ID: "this",
					},
				},
			},
		},
		"HasOnlyName": {
			Spec: WorkspaceSpec{
				RunTasks: []WorkspaceRunTask{
					{
						Name: "this",
					},
				},
			},
		},
		"HasOneWithIDandOneWithName": {
			Spec: WorkspaceSpec{
				RunTasks: []WorkspaceRunTask{
					{
						ID: "this",
					},
					{
						Name: "this",
					},
				},
			},
		},
	}

	for n, c := range successCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecRunTasks(); len(errs) != 0 {
				t.Errorf("Unexpected validation errors: %v", errs)
			}
		})
	}

	errorCases := map[string]Workspace{
		"HasIDandName": {
			Spec: WorkspaceSpec{
				RunTasks: []WorkspaceRunTask{
					{
						ID:   "this",
						Name: "this",
					},
				},
			},
		},
		"HasEmptyIDandName": {
			Spec: WorkspaceSpec{
				RunTasks: []WorkspaceRunTask{
					{
						Name: "",
						ID:   "",
					},
				},
			},
		},
		"HasDuplicateID": {
			Spec: WorkspaceSpec{
				RunTasks: []WorkspaceRunTask{
					{
						ID: "this",
					},
					{
						ID: "this",
					},
				},
			},
		},
		"HasDuplicateName": {
			Spec: WorkspaceSpec{
				RunTasks: []WorkspaceRunTask{
					{
						Name: "this",
					},
					{
						Name: "this",
					},
				},
			},
		},
	}

	for n, c := range errorCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecRunTasks(); len(errs) == 0 {
				t.Error("Unexpected failure, at least one error is expected")
			}
		})
	}
}

func TestValidateWorkspaceSpecRunTriggers(t *testing.T) {
	t.Parallel()

	successCases := map[string]Workspace{
		"HasOnlyID": {
			Spec: WorkspaceSpec{
				RunTriggers: []RunTrigger{
					{
						ID: "this",
					},
				},
			},
		},
		"HasOnlyName": {
			Spec: WorkspaceSpec{
				RunTriggers: []RunTrigger{
					{
						Name: "this",
					},
				},
			},
		},
		"HasOneWithIDandOneWithName": {
			Spec: WorkspaceSpec{
				RunTriggers: []RunTrigger{
					{
						ID: "this",
					},
					{
						Name: "this",
					},
				},
			},
		},
	}

	for n, c := range successCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecRunTriggers(); len(errs) != 0 {
				t.Errorf("Unexpected validation errors: %v", errs)
			}
		})
	}

	errorCases := map[string]Workspace{
		"HasIDandName": {
			Spec: WorkspaceSpec{
				RunTriggers: []RunTrigger{
					{
						ID:   "this",
						Name: "this",
					},
				},
			},
		},
		"HasEmptyIDandName": {
			Spec: WorkspaceSpec{
				RunTriggers: []RunTrigger{
					{
						Name: "",
						ID:   "",
					},
				},
			},
		},
		"HasDuplicateID": {
			Spec: WorkspaceSpec{
				RunTriggers: []RunTrigger{
					{
						ID: "this",
					},
					{
						ID: "this",
					},
				},
			},
		},
		"HasDuplicateName": {
			Spec: WorkspaceSpec{
				RunTriggers: []RunTrigger{
					{
						Name: "this",
					},
					{
						Name: "this",
					},
				},
			},
		},
	}

	for n, c := range errorCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecRunTriggers(); len(errs) == 0 {
				t.Error("Unexpected failure, at least one error is expected")
			}
		})
	}
}

func TestValidateWorkspaceSpecSSHKey(t *testing.T) {
	t.Parallel()

	successCases := map[string]Workspace{
		"HasOnlyID": {
			Spec: WorkspaceSpec{
				SSHKey: &SSHKey{
					ID: "this",
				},
			},
		},
		"HasOnlyName": {
			Spec: WorkspaceSpec{
				SSHKey: &SSHKey{
					Name: "this",
				},
			},
		},
	}

	for n, c := range successCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecSSHKey(); len(errs) != 0 {
				t.Errorf("Unexpected validation errors: %v", errs)
			}
		})
	}

	errorCases := map[string]Workspace{
		"HasIDandName": {
			Spec: WorkspaceSpec{
				SSHKey: &SSHKey{
					ID:   "this",
					Name: "this",
				},
			},
		},
		"HasEmptyIDandName": {
			Spec: WorkspaceSpec{
				SSHKey: &SSHKey{},
			},
		},
	}

	for n, c := range errorCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecSSHKey(); len(errs) == 0 {
				t.Error("Unexpected failure, at least one error is expected")
			}
		})
	}
}

func TestValidateWorkspaceSpecProject(t *testing.T) {
	t.Parallel()

	successCases := map[string]Workspace{
		"HasOnlyID": {
			Spec: WorkspaceSpec{
				Project: &WorkspaceProject{
					ID: "this",
				},
			},
		},
		"HasOnlyName": {
			Spec: WorkspaceSpec{
				Project: &WorkspaceProject{
					Name: "this",
				},
			},
		},
		"HasEmptyProject": {
			Spec: WorkspaceSpec{
				Project: nil,
			},
		},
	}

	for n, c := range successCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecProject(); len(errs) != 0 {
				t.Errorf("Unexpected validation errors: %v", errs)
			}
		})
	}

	errorCases := map[string]Workspace{
		"HasIDandName": {
			Spec: WorkspaceSpec{
				Project: &WorkspaceProject{
					ID:   "this",
					Name: "this",
				},
			},
		},
		"HasEmptyIDandName": {
			Spec: WorkspaceSpec{
				Project: &WorkspaceProject{},
			},
		},
	}

	for n, c := range errorCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecProject(); len(errs) == 0 {
				t.Error("Unexpected failure, at least one error is expected")
			}
		})
	}
}

func TestValidateWorkspaceSpecFileTriggers(t *testing.T) {
	t.Parallel()

	successCases := map[string]Workspace{
		"HasOnlyTriggerPatterns": {
			Spec: WorkspaceSpec{
				FileTriggersEnabled: true,
				TriggerPatterns:     []string{"path/*/workspace/*"},
			},
		},
		"HasOnlyTriggerPrefixes": {
			Spec: WorkspaceSpec{
				FileTriggersEnabled: true,
				WorkingDirectory:    "path/",
				TriggerPrefixes:     []string{"path/to/workspace/"},
			},
		},
	}

	for n, c := range successCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecFileTriggers(); len(errs) != 0 {
				t.Errorf("Unexpected validation errors: %v", errs)
			}
		})
	}

	errorCases := map[string]Workspace{
		"TriggerPrefixesWithoutWorkingDirectory": {
			Spec: WorkspaceSpec{
				FileTriggersEnabled: true,
				TriggerPrefixes:     []string{"path/to/workspace/"},
			},
		},
		"BothTriggerOptions": {
			Spec: WorkspaceSpec{
				FileTriggersEnabled: true,
				WorkingDirectory:    "path/",
				TriggerPatterns:     []string{"path/*/workspace/*"},
				TriggerPrefixes:     []string{"path/to/workspace/"},
			},
		},
		"TriggerPatternsWithoutFileTriggersEnabled": {
			Spec: WorkspaceSpec{
				TriggerPatterns: []string{"path/*/workspace/*"},
			},
		},
		"TriggerPrefixesWithoutFileTriggersEnabled": {
			Spec: WorkspaceSpec{
				TriggerPrefixes: []string{"path/to/workspace/"},
			},
		},
	}

	for n, c := range errorCases {
		t.Run(n, func(t *testing.T) {
			if errs := c.validateSpecFileTriggers(); len(errs) == 0 {
				t.Error("Unexpected failure, at least one error is expected")
			}
		})
	}
}
