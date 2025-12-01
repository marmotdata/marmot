//go:build !production
// +build !production

package websocket

func isProduction() bool {
	return false
}
