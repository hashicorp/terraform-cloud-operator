// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	tfc "github.com/hashicorp/go-tfe"
)

func runTasksDifference(a, b map[string]*tfc.WorkspaceRunTask) map[string]*tfc.WorkspaceRunTask {
	d := make(map[string]*tfc.WorkspaceRunTask)

	for k, v := range a {
		if _, ok := b[k]; !ok {
			d[k] = v
		}
	}

	return d
}

func getRunTasksToCreate(spec, ws map[string]*tfc.WorkspaceRunTask) map[string]*tfc.WorkspaceRunTask {
	return runTasksDifference(spec, ws)
}

func getRunTasksToUpdate(spec, ws map[string]*tfc.WorkspaceRunTask) map[string]*tfc.WorkspaceRunTask {
	o := make(map[string]*tfc.WorkspaceRunTask)

	if len(spec) == 0 || len(ws) == 0 {
		return o
	}

	for ik, iv := range spec {
		if wv, ok := ws[ik]; ok {
			iv.ID = wv.ID
			if !cmp.Equal(iv, wv, cmpopts.IgnoreFields(tfc.WorkspaceRunTask{}, "Workspace")) {
				o[ik] = iv
			}
		}
	}

	return o
}

func getRunTasksToDelete(spec, ws map[string]*tfc.WorkspaceRunTask) map[string]*tfc.WorkspaceRunTask {
	return runTasksDifference(ws, spec)
}

func (r *WorkspaceReconciler) createRunTasks(ctx context.Context, w *workspaceInstance, create map[string]*tfc.WorkspaceRunTask) error {
	for _, rt := range create {
		_, err := w.tfClient.Client.WorkspaceRunTasks.Create(ctx, w.instance.Status.WorkspaceID, tfc.WorkspaceRunTaskCreateOptions{
			Type:             "workspace-task",
			EnforcementLevel: rt.EnforcementLevel,
			RunTask:          rt.RunTask,
			Stage:            &rt.Stage,
		})
		if err != nil {
			w.log.Error(err, "Reconcile Run Tasks", "msg", "failed to create a new run task")
			return err
		}
	}

	return nil
}

func (r *WorkspaceReconciler) updateRunTasks(ctx context.Context, w *workspaceInstance, update map[string]*tfc.WorkspaceRunTask) error {
	for _, v := range update {
		w.log.Info("Reconcile Run Tasks", "msg", "updating run task")
		_, err := w.tfClient.Client.WorkspaceRunTasks.Update(ctx, w.instance.Status.WorkspaceID, v.ID, tfc.WorkspaceRunTaskUpdateOptions{
			Type:             "workspace-task",
			EnforcementLevel: v.EnforcementLevel,
			Stage:            &v.Stage,
		})
		if err != nil {
			w.log.Error(err, "Reconcile Run Tasks", "msg", "failed to update run task")
			return err
		}
	}

	return nil
}

func (r *WorkspaceReconciler) deleteRunTasks(ctx context.Context, w *workspaceInstance, delete map[string]*tfc.WorkspaceRunTask) error {
	for _, d := range delete {
		err := w.tfClient.Client.WorkspaceRunTasks.Delete(ctx, w.instance.Status.WorkspaceID, d.ID)
		if err != nil {
			w.log.Error(err, "Reconcile Run Tasks", "msg", "failed to delete run task")
			return err
		}
	}

	return nil
}

func hasRunTaskName(w *workspaceInstance) bool {
	for _, rt := range w.instance.Spec.RunTasks {
		if rt.Name != "" {
			return true
		}
	}

	return false
}

func (r *WorkspaceReconciler) getInstanceRunTasks(ctx context.Context, w *workspaceInstance) (map[string]*tfc.WorkspaceRunTask, error) {
	o := map[string]*tfc.WorkspaceRunTask{}

	rl := make(map[string]string)
	if hasRunTaskName(w) {
		listOpts := &tfc.RunTaskListOptions{}
		for {
			rt, err := w.tfClient.Client.RunTasks.List(ctx, w.instance.Spec.Organization, listOpts)
			if err != nil {
				return o, err
			}
			for _, t := range rt.Items {
				rl[t.Name] = t.ID
			}
			if rt.NextPage == 0 {
				break
			}
			listOpts.PageNumber = rt.NextPage
		}
	}

	for _, rt := range w.instance.Spec.RunTasks {
		id := rt.ID
		if rt.Name != "" {
			id = rl[rt.Name]
		}
		o[id] = &tfc.WorkspaceRunTask{
			EnforcementLevel: tfc.TaskEnforcementLevel(rt.EnforcementLevel),
			Stage:            tfc.Stage(rt.Stage),
			RunTask:          &tfc.RunTask{ID: id},
		}
	}

	return o, nil
}

func (r *WorkspaceReconciler) getWorkspaceRunTasks(ctx context.Context, w *workspaceInstance) (map[string]*tfc.WorkspaceRunTask, error) {
	o := map[string]*tfc.WorkspaceRunTask{}

	listOpts := &tfc.WorkspaceRunTaskListOptions{}
	for {
		wrt, err := w.tfClient.Client.WorkspaceRunTasks.List(ctx, w.instance.Status.WorkspaceID, listOpts)
		if err != nil {
			return o, err
		}
		for _, rt := range wrt.Items {
			o[rt.RunTask.ID] = &tfc.WorkspaceRunTask{
				ID:               rt.ID,
				EnforcementLevel: rt.EnforcementLevel,
				Stage:            rt.Stage,
				RunTask:          &tfc.RunTask{ID: rt.RunTask.ID},
			}
		}
		if wrt.NextPage == 0 {
			break
		}
		listOpts.PageNumber = wrt.NextPage
	}

	return o, nil
}

func (r *WorkspaceReconciler) reconcileRunTasks(ctx context.Context, w *workspaceInstance) error {
	w.log.Info("Reconcile Run Tasks", "msg", "new reconciliation event")

	spec, err := r.getInstanceRunTasks(ctx, w)
	if err != nil {
		w.log.Error(err, "Reconcile Run Tasks", "msg", "failed to get instance run tasks")
		return err
	}

	ws, err := r.getWorkspaceRunTasks(ctx, w)
	if err != nil {
		w.log.Error(err, "Reconcile Run Tasks", "msg", "failed to get workspace run tasks")
		return err
	}

	create := getRunTasksToCreate(spec, ws)
	if len(create) > 0 {
		w.log.Info("Reconcile Run Tasks", "msg", fmt.Sprintf("creating %d run tasks", len(create)))
		err := r.createRunTasks(ctx, w, create)
		if err != nil {
			w.log.Error(err, "Reconcile Run Tasks", "msg", "failed to create a run task")
			return err
		}
	}

	update := getRunTasksToUpdate(spec, ws)
	if len(update) > 0 {
		w.log.Info("Reconcile Run Tasks", "msg", fmt.Sprintf("updating %d run tasks", len(update)))
		err := r.updateRunTasks(ctx, w, update)
		if err != nil {
			w.log.Error(err, "Reconcile Run Tasks", "msg", "failed to update a run task")
			return err
		}
	}

	delete := getRunTasksToDelete(spec, ws)
	if len(delete) > 0 {
		w.log.Info("Reconcile Run Tasks", "msg", fmt.Sprintf("deleting %d run tasks", len(delete)))
		err := r.deleteRunTasks(ctx, w, delete)
		if err != nil {
			w.log.Error(err, "Reconcile Run Tasks", "msg", "failed to delete a run task")
			return err
		}
	}

	return nil
}
