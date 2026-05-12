package auth

import (
	"os"
	"strings"
)

const K8sSATokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token" //nolint:gosec

// WorkloadCredential returns an in-cluster Kubernetes service-account
// credential, or nil if none is available.
func WorkloadCredential() Credential {
	if c := kubernetesSA(); c != nil {
		return c
	}
	return nil
}

func kubernetesSA() Credential {
	data, err := os.ReadFile(K8sSATokenPath)
	if err != nil {
		return nil
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return nil
	}
	return &bearerCred{token: token, source: "k8s-sa"}
}
