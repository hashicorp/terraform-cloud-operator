// // Copyright (c) HashiCorp, Inc.
// // SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	tfc "github.com/hashicorp/go-tfe"
	corev1 "k8s.io/api/core/v1"
)

func (r *ProjectReconciler) getInstanceTeamAccess(ctx context.Context, p *projectInstance) (map[string]*tfc.TeamProjectAccess, error) {
	o := map[string]*tfc.TeamProjectAccess{}

	if p.instance.Spec.TeamAccess == nil {
		return o, nil
	}

	teams, err := r.getTeams(ctx, p)
	if err != nil {
		return o, err
	}

	for _, ta := range p.instance.Spec.TeamAccess {
		tID, err := getTeamID(teams, ta.Team)
		if err != nil {
			p.log.Error(err, "Reconcile Team Access", "msg", "failed to get team ID")
			r.Recorder.Event(&p.instance, corev1.EventTypeWarning, "ReconcileTeamAccess", "Failed to get team ID")
			return o, err
		}

		o[tID] = &tfc.TeamProjectAccess{
			Access: ta.Access,
			Team: &tfc.Team{
				ID: tID,
			},
		}
		if ta.Access == tfc.TeamProjectAccessCustom {
			o[tID].WorkspaceAccess = &tfc.TeamProjectAccessWorkspacePermissions{
				WorkspaceRunsPermission:          ta.Custom.Runs,
				WorkspaceSentinelMocksPermission: ta.Custom.SentinelMocks,
				WorkspaceStateVersionsPermission: ta.Custom.StateVersions,
				WorkspaceVariablesPermission:     ta.Custom.Variables,
				WorkspaceCreatePermission:        ta.Custom.CreateWorkspace,
				WorkspaceLockingPermission:       ta.Custom.LockWorkspace,
				WorkspaceMovePermission:          ta.Custom.MoveWorkspace,
				WorkspaceDeletePermission:        ta.Custom.DeleteWorkspace,
				WorkspaceRunTasksPermission:      ta.Custom.RunTasks,
			}
			o[tID].ProjectAccess = &tfc.TeamProjectAccessProjectPermissions{
				ProjectSettingsPermission: ta.Custom.ProjectAccess,
				ProjectTeamsPermission:    ta.Custom.TeamManagement,
			}
		}
	}

	return o, nil
}

func (r *ProjectReconciler) getWorkspaceTeamAccess(ctx context.Context, p *projectInstance) (map[string]*tfc.TeamProjectAccess, error) {
	o := map[string]*tfc.TeamProjectAccess{}

	listOpts := tfc.TeamProjectAccessListOptions{ProjectID: p.instance.Status.ID}
	for {
		t, err := p.tfClient.Client.TeamProjectAccess.List(ctx, listOpts)
		if err != nil {
			return o, err
		}
		for _, ta := range t.Items {
			o[ta.Team.ID] = ta
		}
		if t.NextPage == 0 {
			break
		}
		listOpts.PageNumber = t.NextPage
	}
	return o, nil
}

func (r *ProjectReconciler) getTeams(ctx context.Context, p *projectInstance) (map[string]*tfc.Team, error) {
	teams := make(map[string]*tfc.Team)

	fTeams := []string{}
	for _, t := range p.instance.Spec.TeamAccess {
		if t.Team.Name != "" {
			fTeams = append(fTeams, t.Team.Name)
		}
	}

	listOpts := &tfc.TeamListOptions{
		Names: fTeams,
	}
	for {
		tl, err := p.tfClient.Client.Teams.List(ctx, p.instance.Spec.Organization, listOpts)
		if err != nil {
			return teams, err
		}
		for _, t := range tl.Items {
			teams[t.Name] = t
		}
		if tl.NextPage == 0 {
			break
		}
		listOpts.PageNumber = tl.NextPage
	}

	return teams, nil
}

func teamProjectAccessDifference(a, b map[string]*tfc.TeamProjectAccess) map[string]*tfc.TeamProjectAccess {
	d := make(map[string]*tfc.TeamProjectAccess)

	for k, v := range a {
		if _, ok := b[k]; !ok {
			d[k] = v
		}
	}

	return d
}

func getTeamProjectAccessToCreate(specTeamAccess, projectTeamAccess map[string]*tfc.TeamProjectAccess) map[string]*tfc.TeamProjectAccess {
	return teamProjectAccessDifference(specTeamAccess, projectTeamAccess)
}

func getTeamProjectAccessToDelete(specTeamAccess, projectTeamAccess map[string]*tfc.TeamProjectAccess) map[string]*tfc.TeamProjectAccess {
	return teamProjectAccessDifference(projectTeamAccess, specTeamAccess)
}

func getTeamProjectAccessToUpdate(specTeamAccess, workspaceTeamAccess map[string]*tfc.TeamProjectAccess) map[string]*tfc.TeamProjectAccess {
	ta := make(map[string]*tfc.TeamProjectAccess)

	if len(specTeamAccess) == 0 || len(workspaceTeamAccess) == 0 {
		return ta
	}

	for ik, iv := range specTeamAccess {
		if wv, ok := workspaceTeamAccess[ik]; ok {
			iv.ID = wv.ID
			if iv.Access == tfc.TeamProjectAccessCustom {
				if !cmp.Equal(iv, wv, cmpopts.IgnoreFields(tfc.TeamProjectAccess{}, "ID", "Team", "Project")) {
					ta[ik] = iv
				}
			} else {
				if iv.Access != wv.Access {
					ta[ik] = iv
				}
			}
		}
	}

	return ta
}

func (r *ProjectReconciler) createTeamProjectAccess(ctx context.Context, p *projectInstance, createTeamAccess map[string]*tfc.TeamProjectAccess) error {
	projectID := p.instance.Status.ID
	for tID, v := range createTeamAccess {
		option := tfc.TeamProjectAccessAddOptions{
			Project: &tfc.Project{
				ID: projectID,
			},
			Team: &tfc.Team{
				ID: tID,
			},
			Access: v.Access,
		}

		if v.Access == tfc.TeamProjectAccessCustom {
			option.ProjectAccess = &tfc.TeamProjectAccessProjectPermissionsOptions{
				Settings: &v.ProjectAccess.ProjectSettingsPermission,
				Teams:    &v.ProjectAccess.ProjectTeamsPermission,
			}
			option.WorkspaceAccess = &tfc.TeamProjectAccessWorkspacePermissionsOptions{
				Runs:          &v.WorkspaceAccess.WorkspaceRunsPermission,
				SentinelMocks: &v.WorkspaceAccess.WorkspaceSentinelMocksPermission,
				StateVersions: &v.WorkspaceAccess.WorkspaceStateVersionsPermission,
				Variables:     &v.WorkspaceAccess.WorkspaceVariablesPermission,
				Create:        &v.WorkspaceAccess.WorkspaceCreatePermission,
				Locking:       &v.WorkspaceAccess.WorkspaceLockingPermission,
				Move:          &v.WorkspaceAccess.WorkspaceMovePermission,
				Delete:        &v.WorkspaceAccess.WorkspaceDeletePermission,
				RunTasks:      &v.WorkspaceAccess.WorkspaceRunTasksPermission,
			}
		}

		_, err := p.tfClient.Client.TeamProjectAccess.Add(ctx, option)
		if err != nil {
			p.log.Error(err, "Reconcile Team Access", "msg", "failed to create a new team access")
			return err
		}
	}

	return nil
}

func (r *ProjectReconciler) deleteTeamAccess(ctx context.Context, p *projectInstance, deleteTeamAccess map[string]*tfc.TeamProjectAccess) error {
	for _, v := range deleteTeamAccess {
		err := p.tfClient.Client.TeamProjectAccess.Remove(ctx, v.ID)
		if err != nil {
			p.log.Error(err, "Reconcile Team Access", "msg", "failed to delete team access")
			return err
		}
	}

	return nil
}

func (r *ProjectReconciler) updateTeamAccess(ctx context.Context, p *projectInstance, updateTeamAccess map[string]*tfc.TeamProjectAccess) error {
	for _, v := range updateTeamAccess {
		p.log.Info("Reconcile Team Access", "msg", "updating team access")
		option := tfc.TeamProjectAccessUpdateOptions{
			Access: &v.Access,
		}

		if v.Access == tfc.TeamProjectAccessCustom {
			option.ProjectAccess = &tfc.TeamProjectAccessProjectPermissionsOptions{
				Settings: &v.ProjectAccess.ProjectSettingsPermission,
				Teams:    &v.ProjectAccess.ProjectTeamsPermission,
			}
			option.WorkspaceAccess = &tfc.TeamProjectAccessWorkspacePermissionsOptions{
				Runs:          &v.WorkspaceAccess.WorkspaceRunsPermission,
				SentinelMocks: &v.WorkspaceAccess.WorkspaceSentinelMocksPermission,
				StateVersions: &v.WorkspaceAccess.WorkspaceStateVersionsPermission,
				Variables:     &v.WorkspaceAccess.WorkspaceVariablesPermission,
				Create:        &v.WorkspaceAccess.WorkspaceCreatePermission,
				Locking:       &v.WorkspaceAccess.WorkspaceLockingPermission,
				Move:          &v.WorkspaceAccess.WorkspaceMovePermission,
				Delete:        &v.WorkspaceAccess.WorkspaceDeletePermission,
				RunTasks:      &v.WorkspaceAccess.WorkspaceRunTasksPermission,
			}
		}

		_, err := p.tfClient.Client.TeamProjectAccess.Update(ctx, v.ID, option)
		if err != nil {
			p.log.Error(err, "Reconcile Team Access", "msg", "failed to update team access")
			return err
		}
	}

	return nil
}

func (r *ProjectReconciler) reconcileTeamAccess(ctx context.Context, p *projectInstance) error {
	p.log.Info("Reconcile Team Access", "msg", "new reconciliation event")

	specTeamAccess, err := r.getInstanceTeamAccess(ctx, p)
	if err != nil {
		p.log.Error(err, "Reconcile Team Access", "msg", "failed to get instance team access")
		return err
	}

	projectTeamAccess, err := r.getWorkspaceTeamAccess(ctx, p)
	if err != nil {
		p.log.Error(err, "Reconcile Team Access", "msg", "failed to get project team access")
		return err
	}

	createTeamAccess := getTeamProjectAccessToCreate(specTeamAccess, projectTeamAccess)
	if len(createTeamAccess) > 0 {
		p.log.Info("Reconcile Team Access", "msg", fmt.Sprintf("creating %d team accesses", len(createTeamAccess)))
		err := r.createTeamProjectAccess(ctx, p, createTeamAccess)
		if err != nil {
			return err
		}
	}

	updateTeamAccess := getTeamProjectAccessToUpdate(specTeamAccess, projectTeamAccess)
	if len(updateTeamAccess) > 0 {
		p.log.Info("Reconcile Team Access", "msg", fmt.Sprintf("updating %d team accesses", len(updateTeamAccess)))
		err := r.updateTeamAccess(ctx, p, updateTeamAccess)
		if err != nil {
			return err
		}
	}

	deleteTeamAccess := getTeamProjectAccessToDelete(specTeamAccess, projectTeamAccess)
	if len(deleteTeamAccess) > 0 {
		p.log.Info("Reconcile Team Access", "msg", fmt.Sprintf("deleting %d team accesses", len(deleteTeamAccess)))
		err := r.deleteTeamAccess(ctx, p, deleteTeamAccess)
		if err != nil {
			return err
		}
	}

	return nil
}
