// Package marmot is the Go SDK for the Marmot data catalog.
//
// Usage:
//
//	c, err := marmot.NewClient(marmot.ClientOptions{
//		Host:   "https://marmot.example.com",
//		APIKey: os.Getenv("MARMOT_API_KEY"),
//	})
//
//	assets, err := c.Assets.Search(ctx, marmot.AssetSearchOptions{Query: "orders"})
//
// Credentials resolve from ClientOptions, then MARMOT_API_KEY / MARMOT_TOKEN,
// then the cached `marmot login` token, then a Kubernetes service-account
// token. API errors are typed: *AuthError, *NotFoundError, *ValidationError,
// *RateLimitError, *ServerError, all embedding *APIError.
package marmot
