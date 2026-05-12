package cmd

import (
	"encoding/json"
	"fmt"

	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/marmotdata/marmot/sdk/go/auth"
)

// newClient builds an SDK client configured from the active CLI context.
func newClient() (*marmot.Client, error) {
	token, isBearer := getAuthToken()
	opts := marmot.ClientOptions{Host: getHost()}
	if token != "" {
		if isBearer {
			opts.Credential = auth.Bearer(token)
		} else {
			opts.Credential = auth.APIKey(token)
		}
	}
	c, err := marmot.NewClient(opts)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return c, nil
}

// marshalPayload marshals any SDK payload to JSON bytes for raw output.
func marshalPayload(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshalling response: %w", err)
	}
	return data, nil
}
