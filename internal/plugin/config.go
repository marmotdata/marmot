package plugin

import (
	"encoding/json"
	"strings"
)

// MaskSensitiveInfo masks sensitive values in a plugin configuration
func (c RawPluginConfig) MaskSensitiveInfo(sensitiveValues ...string) RawPluginConfig {
	jsonBytes, err := json.Marshal(c)
	if err != nil {
		return c
	}

	jsonStr := string(jsonBytes)

	for _, value := range sensitiveValues {
		if value != "" {
			jsonStr = strings.Replace(jsonStr, value, "******", -1)
		}
	}

	var result RawPluginConfig
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return c
	}

	return result
}
