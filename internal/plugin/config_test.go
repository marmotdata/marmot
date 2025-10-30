package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestConfig struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password" sensitive:"true"`
	APIKey   string `json:"api_key" sensitive:"true"`
	Port     int    `json:"port"`
}

type NestedCredentials struct {
	Username string `json:"username"`
	Secret   string `json:"secret" sensitive:"true"`
}

type TestConfigNested struct {
	Host        string            `json:"host"`
	Credentials NestedCredentials `json:"credentials"`
}

func TestMaskSensitiveFields(t *testing.T) {
	tests := []struct {
		name     string
		config   RawPluginConfig
		expected RawPluginConfig
		cfgType  interface{}
	}{
		{
			name: "masks password field",
			config: RawPluginConfig{
				"host":     "localhost",
				"user":     "admin",
				"password": "secret123",
				"port":     5432,
			},
			expected: RawPluginConfig{
				"host":     "localhost",
				"user":     "admin",
				"password": SensitiveMask,
				"port":     5432,
			},
			cfgType: &TestConfig{},
		},
		{
			name: "masks multiple sensitive fields",
			config: RawPluginConfig{
				"host":     "localhost",
				"user":     "admin",
				"password": "secret123",
				"api_key":  "key-abc-123",
				"port":     5432,
			},
			expected: RawPluginConfig{
				"host":     "localhost",
				"user":     "admin",
				"password": SensitiveMask,
				"api_key":  SensitiveMask,
				"port":     5432,
			},
			cfgType: &TestConfig{},
		},
		{
			name: "handles empty sensitive fields",
			config: RawPluginConfig{
				"host":     "localhost",
				"user":     "admin",
				"password": "",
				"port":     5432,
			},
			expected: RawPluginConfig{
				"host":     "localhost",
				"user":     "admin",
				"password": "",
				"port":     5432,
			},
			cfgType: &TestConfig{},
		},
		{
			name: "masks nested sensitive fields",
			config: RawPluginConfig{
				"host": "localhost",
				"credentials": map[string]interface{}{
					"username": "admin",
					"secret":   "supersecret",
				},
			},
			expected: RawPluginConfig{
				"host": "localhost",
				"credentials": map[string]interface{}{
					"username": "admin",
					"secret":   SensitiveMask,
				},
			},
			cfgType: &TestConfigNested{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.MaskSensitiveFields(tt.cfgType)
			assert.Equal(t, tt.expected, result)
		})
	}
}
