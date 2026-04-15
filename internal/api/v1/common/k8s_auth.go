package common

import (
	"context"
	"fmt"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// K8sTokenValidator validates Kubernetes ServiceAccount tokens via the TokenReview API.
type K8sTokenValidator struct {
	clientset *kubernetes.Clientset
}

// NewK8sTokenValidator creates a validator using in-cluster credentials.
func NewK8sTokenValidator() (*K8sTokenValidator, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("getting in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes clientset: %w", err)
	}

	return &K8sTokenValidator{clientset: clientset}, nil
}

// Validate checks a token via the TokenReview API and returns the namespace and service account name.
func (v *K8sTokenValidator) Validate(ctx context.Context, token string) (namespace, serviceAccount string, err error) {
	review := &authenticationv1.TokenReview{
		Spec: authenticationv1.TokenReviewSpec{
			Token: token,
		},
	}

	result, err := v.clientset.AuthenticationV1().TokenReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("token review request failed: %w", err)
	}

	if !result.Status.Authenticated {
		return "", "", fmt.Errorf("token not authenticated")
	}

	// Username format: system:serviceaccount:<namespace>:<name>
	username := result.Status.User.Username
	const prefix = "system:serviceaccount:"
	if len(username) <= len(prefix) {
		return "", "", fmt.Errorf("unexpected username format: %s", username)
	}

	parts := username[len(prefix):]
	for i := 0; i < len(parts); i++ {
		if parts[i] == ':' {
			return parts[:i], parts[i+1:], nil
		}
	}

	return "", "", fmt.Errorf("unexpected username format: %s", username)
}
