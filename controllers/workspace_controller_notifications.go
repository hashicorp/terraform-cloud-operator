// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	tfc "github.com/hashicorp/go-tfe"

	"github.com/hashicorp/terraform-cloud-operator/internal/slice"
)

func hasNotificationItem(s []tfc.NotificationConfiguration, f tfc.NotificationConfiguration) (int, bool) {
	for i, v := range s {
		if cmp.Equal(v, f, cmpopts.IgnoreFields(tfc.NotificationConfiguration{},
			"CreatedAt", "DeliveryResponses", "EmailAddresses", "EmailUsers", "Enabled", "ID", "Subscribable", "Token", "Triggers", "URL", "UpdatedAt")) {
			return i, true
		}

	}
	return -1, false
}

func notificationsDifference(a, b []tfc.NotificationConfiguration) []tfc.NotificationConfiguration {
	var d []tfc.NotificationConfiguration
	bc := make([]tfc.NotificationConfiguration, len(b))

	copy(bc, b)

	for _, av := range a {
		i, t := hasNotificationItem(bc, av)
		if t {
			bc = slice.RemoveFromSlice(bc, i)
		} else {
			d = append(d, av)
		}
	}

	return d
}

func (r *WorkspaceReconciler) getOrgMembers(ctx context.Context, w *workspaceInstance) (map[string]*tfc.User, error) {
	eu := make(map[string]*tfc.User)
	e := make([]string, 0)
	for _, n := range w.instance.Spec.Notifications {
		for _, ne := range n.EmailUsers {
			eu[ne] = nil
			e = append(e, ne)
		}
	}
	listOpts := &tfc.OrganizationMembershipListOptions{
		Emails: e,
	}
	for {
		members, err := w.tfClient.Client.OrganizationMemberships.List(ctx, w.instance.Spec.Organization, listOpts)
		if err != nil {
			return nil, err
		}
		for _, m := range members.Items {
			eu[m.Email] = &tfc.User{ID: m.User.ID}
		}
		if members.NextPage == 0 {
			break
		}
		listOpts.PageNumber = members.NextPage
	}
	return eu, nil
}

func (r *WorkspaceReconciler) getInstanceNotifications(ctx context.Context, w *workspaceInstance) ([]tfc.NotificationConfiguration, error) {
	if len(w.instance.Spec.Notifications) == 0 {
		return []tfc.NotificationConfiguration{}, nil
	}

	orgEmailUsers, err := r.getOrgMembers(ctx, w)
	if err != nil {
		return []tfc.NotificationConfiguration{}, err
	}

	o := make([]tfc.NotificationConfiguration, len(w.instance.Spec.Notifications))

	for i, n := range w.instance.Spec.Notifications {
		var eu []*tfc.User
		for _, e := range n.EmailUsers {
			if v, ok := orgEmailUsers[e]; ok && v != nil {
				eu = append(eu, v)
			}
		}
		nt := make([]string, len(n.Triggers))
		for i, t := range n.Triggers {
			nt[i] = string(t)
		}
		o[i] = tfc.NotificationConfiguration{
			Name:            n.Name,
			DestinationType: n.Type,
			URL:             n.URL,
			Enabled:         n.Enabled,
			Token:           n.Token,
			Triggers:        nt,
			EmailAddresses:  n.EmailAddresses,
			EmailUsers:      eu,
		}
	}

	return o, nil
}

func (r *WorkspaceReconciler) getWorkspaceNotifications(ctx context.Context, w *workspaceInstance) ([]tfc.NotificationConfiguration, error) {
	var o []tfc.NotificationConfiguration

	listOpts := &tfc.NotificationConfigurationListOptions{}
	for {
		wn, err := w.tfClient.Client.NotificationConfigurations.List(ctx, w.instance.Status.WorkspaceID, listOpts)
		if err != nil {
			return nil, err
		}
		for _, n := range wn.Items {
			o = append(o, tfc.NotificationConfiguration{
				ID:              n.ID,
				Name:            n.Name,
				DestinationType: n.DestinationType,
				URL:             n.URL,
				Enabled:         n.Enabled,
				Token:           n.Token,
				Triggers:        n.Triggers,
				EmailAddresses:  n.EmailAddresses,
				EmailUsers:      n.EmailUsers,
			})
		}
		if wn.NextPage == 0 {
			break
		}
		listOpts.PageNumber = wn.NextPage
	}

	return o, nil
}

func getNotificationsToCreate(spec, ws []tfc.NotificationConfiguration) []tfc.NotificationConfiguration {
	return notificationsDifference(spec, ws)
}

func getNotificationsToUpdate(spec, ws []tfc.NotificationConfiguration) []tfc.NotificationConfiguration {
	o := []tfc.NotificationConfiguration{}

	if len(spec) == 0 || len(ws) == 0 {
		return o
	}

	for _, sv := range spec {
		for _, wv := range ws {
			if sv.Name == wv.Name {
				if !cmp.Equal(sv, wv, cmpopts.IgnoreFields(tfc.NotificationConfiguration{},
					"CreatedAt", "DeliveryResponses", "ID", "Subscribable", "UpdatedAt")) {
					sv.ID = wv.ID
					o = append(o, sv)
				}
			}
		}
	}

	return o
}

func getNotificationsToDelete(spec, ws []tfc.NotificationConfiguration) []tfc.NotificationConfiguration {
	return notificationsDifference(ws, spec)
}

func (r *WorkspaceReconciler) createNotifications(ctx context.Context, w *workspaceInstance, create []tfc.NotificationConfiguration) error {
	for _, c := range create {
		w.log.Info("Reconcile Notifications", "msg", "creating notificaion")
		nt := make([]tfc.NotificationTriggerType, len(c.Triggers))
		for i, t := range c.Triggers {
			nt[i] = tfc.NotificationTriggerType(t)
		}
		_, err := w.tfClient.Client.NotificationConfigurations.Create(ctx, w.instance.Status.WorkspaceID, tfc.NotificationConfigurationCreateOptions{
			Name:            &c.Name,
			DestinationType: &c.DestinationType,
			URL:             &c.URL,
			Enabled:         &c.Enabled,
			Token:           &c.Token,
			Triggers:        nt,
			EmailUsers:      c.EmailUsers,
			EmailAddresses:  c.EmailAddresses,
		})
		if err != nil {
			w.log.Error(err, "Reconcile Notifications", "msg", fmt.Sprintf("failed to create a new notification %q", c.ID))
			return err
		}
	}

	return nil
}

func (r *WorkspaceReconciler) updateNotifications(ctx context.Context, w *workspaceInstance, update []tfc.NotificationConfiguration) error {
	for _, u := range update {
		w.log.Info("Reconcile Notifications", "msg", fmt.Sprintf("updating notificaion %q", u.ID))
		nt := make([]tfc.NotificationTriggerType, len(u.Triggers))
		for i, t := range u.Triggers {
			nt[i] = tfc.NotificationTriggerType(t)
		}
		_, err := w.tfClient.Client.NotificationConfigurations.Update(ctx, u.ID, tfc.NotificationConfigurationUpdateOptions{
			Name:           &u.Name,
			Enabled:        &u.Enabled,
			Token:          &u.Token,
			Triggers:       nt,
			URL:            &u.URL,
			EmailAddresses: u.EmailAddresses,
			EmailUsers:     u.EmailUsers,
		})
		if err != nil {
			w.log.Error(err, "Reconcile Notifications", "msg", fmt.Sprintf("failed to update notificaion %q", u.ID))
			return err
		}
	}

	return nil
}

func (r *WorkspaceReconciler) deleteNotifications(ctx context.Context, w *workspaceInstance, delete []tfc.NotificationConfiguration) error {
	for _, d := range delete {
		w.log.Info("Reconcile Notifications", "msg", fmt.Sprintf("deleting notificaion %q", d.ID))
		err := w.tfClient.Client.NotificationConfigurations.Delete(ctx, d.ID)
		if err != nil {
			w.log.Error(err, "Reconcile Notifications", "msg", fmt.Sprintf("failed to delete notificaions %q", d.ID))
			return err
		}
	}

	return nil
}

func (r *WorkspaceReconciler) reconcileNotifications(ctx context.Context, w *workspaceInstance) error {
	w.log.Info("Reconcile Notifications", "msg", "new reconciliation event")

	spec, err := r.getInstanceNotifications(ctx, w)
	if err != nil {
		w.log.Error(err, "Reconcile Notifications", "msg", "failed to get instance notificaions")
		return err
	}

	ws, err := r.getWorkspaceNotifications(ctx, w)
	if err != nil {
		w.log.Error(err, "Reconcile Notifications", "msg", "failed to get workspace notifications")
		return err
	}

	delete := getNotificationsToDelete(spec, ws)
	if len(delete) > 0 {
		w.log.Info("Reconcile Notifications", "msg", fmt.Sprintf("deleting %d notifications", len(delete)))
		err := r.deleteNotifications(ctx, w, delete)
		if err != nil {
			w.log.Error(err, "Reconcile Notifications", "msg", "failed to delete a notificaion")
			return err
		}
	}

	create := getNotificationsToCreate(spec, ws)
	if len(create) > 0 {
		w.log.Info("Reconcile Notifications", "msg", fmt.Sprintf("creating %d notifications", len(create)))
		err := r.createNotifications(ctx, w, create)
		if err != nil {
			w.log.Error(err, "Reconcile Notifications", "msg", "failed to create a notificaion")
			return err
		}
	}

	ws, err = r.getWorkspaceNotifications(ctx, w)
	if err != nil {
		w.log.Error(err, "Reconcile Notifications", "msg", "failed to get workspace notifications")
		return err
	}

	update := getNotificationsToUpdate(spec, ws)
	if len(update) > 0 {
		w.log.Info("Reconcile Notifications", "msg", fmt.Sprintf("updating %d notifications", len(update)))
		err := r.updateNotifications(ctx, w, update)
		if err != nil {
			w.log.Error(err, "Reconcile Notifications", "msg", "failed to update a notificaion")
			return err
		}
	}

	return nil
}
