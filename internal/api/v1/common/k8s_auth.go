package common

import (
	"context"
	"fmt"
	"slices"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// K8sTokenAudience is the audience ingestion pods request for the projected ServiceAccount token they send to Marmot. Binding the token to Marmot means it is useless against the Kubernetes API, and a token minted for the API server is refused here.
const K8sTokenAudience = "marmot"

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
			Token:     token,
			Audiences: []string{K8sTokenAudience},
		},
	}

	result, err := v.clientset.AuthenticationV1().TokenReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("token review request failed: %w", err)
	}

	if !result.Status.Authenticated {
		return "", "", fmt.Errorf("token not authenticated")
	}
	// The TokenReview API asks clients that set spec.audiences to confirm the authenticator honoured them; an empty status.audiences means the token was only checked against the API server's own audience.
	if !slices.Contains(result.Status.Audiences, K8sTokenAudience) {
		return "", "", fmt.Errorf("token is not for audience %q", K8sTokenAudience)
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
