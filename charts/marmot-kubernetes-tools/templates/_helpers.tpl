{{- define "marmot-tools.fullname" -}}
{{- if contains .Chart.Name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "marmot-tools.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "marmot-tools.serviceAccountName" -}}
{{- default (include "marmot-tools.fullname" .) .Values.serviceAccount.name -}}
{{- end -}}

{{/* The username the API server gives requests made with the ServiceAccount's token. */}}
{{- define "marmot-tools.username" -}}
{{- printf "system:serviceaccount:%s:%s" .Release.Namespace (include "marmot-tools.serviceAccountName" .) -}}
{{- end -}}

{{/* Whether any namespaced verb, the reads or the triggers are on, so a Role has something to grant. */}}
{{- define "marmot-tools.namespaced" -}}
{{- with .Values.tools -}}
{{- if or .scale .rolloutRestart .cronJobs .observe -}}true{{- end -}}
{{- end -}}
{{- if .Values.triggers.enabled -}}true{{- end -}}
{{- end -}}


{{/* What triggers watch, namespaced or cluster wide. */}}
{{- define "marmot-tools.triggerRules" -}}
- apiGroups: [""]
  resources: ["pods", "events", "persistentvolumeclaims"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["replicasets", "deployments", "statefulsets", "daemonsets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["batch"]
  resources: ["jobs", "cronjobs"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["autoscaling"]
  resources: ["horizontalpodautoscalers"]
  verbs: ["get", "list", "watch"]
{{- end -}}
