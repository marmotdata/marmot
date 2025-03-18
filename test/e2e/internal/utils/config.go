package utils

type TestConfig struct {
	PostgresPort          string
	PostgresContainerName string
	NetworkName           string
	PostgresPassword      string
	ApplicationPort       string
}

func NewDefaultConfig() TestConfig {
	return TestConfig{
		PostgresPort:     "5432",
		NetworkName:      "marmot-test-network",
		PostgresPassword: "testpassword",
		ApplicationPort:  "8080",
	}
}
