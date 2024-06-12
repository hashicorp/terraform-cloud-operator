// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"time"
)

// SHARED CONSTANTS
const (
	annotationTrue  = "true"
	annotationFalse = "false"
	maxPageSize     = 100
	requeueInterval = 15 * time.Second
	runMessage      = "Triggered by HCP Terraform Operator"
)

// AGENT POOL CONTROLLER'S CONSTANTS
const (
	agentPoolFinalizer = "agentpool.app.terraform.io/finalizer"
)

// MODULE CONTROLLER'S CONSTANTS
const (
	requeueConfigurationUploadInterval = 10 * time.Second
	requeueNewRunInterval              = 10 * time.Second
	requeueRunStatusInterval           = 30 * time.Second
	moduleFinalizer                    = "module.app.terraform.io/finalizer"

	moduleTemplate = `
{{- $moduleName  := .Name -}}
{{- if .Variables }}
  {{ range $v := .Variables }}
variable "{{ $v.Name }}" {}
  {{- end}}
{{- end }}

module "{{ $moduleName }}" {
  source  = "{{ .Module.Source }}"
{{- if .Module.Version }}
  version = "{{ .Module.Version }}"
{{- end }}

{{- if .Variables }}
  {{ range $v := .Variables }}
    {{ $v.Name }} = var.{{ $v.Name }}
  {{- end}}
{{- end }}
}

{{- if .Outputs }}
  {{ range $o := .Outputs }}
output "{{ $o.Name }}" {
  value     = module.{{ $moduleName }}.{{ $o.Name }}
  sensitive = {{ $o.Sensitive }}
}
  {{- end}}
{{- end }}
`
)

// PROJECT CONTROLLER'S CONSTANTS
const (
	projectFinalizer = "project.app.terraform.io/finalizer"
)

// WORKSPACE CONTROLLER'S CONSTANTS
const (
	workspaceFinalizerAlpha1 = "finalizer.workspace.app.terraform.io"
	workspaceFinalizer       = "workspace.app.terraform.io/finalizer"

	workspaceAnnotationRunNew              = "workspace.app.terraform.io/run-new"
	workspaceAnnotationRunType             = "workspace.app.terraform.io/run-type"
	workspaceAnnotationRunTerraformVersion = "workspace.app.terraform.io/run-terraform-version"

	runTypePlan    = "plan"
	runTypeApply   = "apply"
	runTypeRefresh = "refresh"
	runTypeDefault = runTypePlan
)
