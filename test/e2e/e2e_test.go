package e2e

import (
	"log"
	"os"
	"testing"
)

var env *TestEnvironment

func TestMain(m *testing.M) {
	var err error
	env, err = SetupTestEnvironment(&testing.T{})
	if err != nil {
		log.Fatalf("Failed to set up test environment: %v", err)
	}

	code := m.Run()

	if env != nil {
		env.Cleanup()
	}
	os.Exit(code)
}
