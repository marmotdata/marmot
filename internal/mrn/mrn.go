package mrn

import (
	"fmt"
	"strings"
)

type Format struct {
	Type    string
	Service string
	Name    string
}

func New(assetType, service, name string) string {
	sanitized := strings.Map(func(r rune) rune {
		if r == '/' || r == ' ' {
			return '-'
		}
		return r
	}, name)

	return fmt.Sprintf("mrn://%s/%s/%s",
		strings.ToLower(assetType),
		strings.ToLower(service),
		strings.ToLower(sanitized))
}

func Parse(mrn string) (*Format, error) {
	parts := strings.Split(strings.TrimPrefix(mrn, "mrn://"), "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid MRN format: expected mrn://<type>/<service>/<name>, got %s", mrn)
	}

	return &Format{
		Type:    parts[0],
		Service: parts[1],
		Name:    parts[2],
	}, nil
}
