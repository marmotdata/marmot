package nifi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUniqueNames_KeepsDistinctNamesAsTheyAre(t *testing.T) {
	names := uniqueNames([]namedID{{"a", "Generate"}, {"b", "Log"}})

	assert.Equal(t, "Generate", names["a"])
	assert.Equal(t, "Log", names["b"])
}

func TestUniqueNames_AppendsTheIDToSiblingsSharingAName(t *testing.T) {
	names := uniqueNames([]namedID{{"a", "LogAttribute"}, {"b", "LogAttribute"}, {"c", "Other"}})

	assert.Equal(t, "LogAttribute (a)", names["a"])
	assert.Equal(t, "LogAttribute (b)", names["b"])
	assert.Equal(t, "Other", names["c"])
}

func TestBreadcrumbPath_JoinsTheChainFromTheRoot(t *testing.T) {
	leaf := breadcrumbEntity{
		Breadcrumb: breadcrumb{ID: "c", Name: "Parse"},
		ParentBreadcrumb: &breadcrumbEntity{
			Breadcrumb: breadcrumb{ID: "b", Name: "Ingest"},
			ParentBreadcrumb: &breadcrumbEntity{
				Breadcrumb: breadcrumb{ID: "a", Name: "NiFi Flow"},
			},
		},
	}

	assert.Equal(t, "NiFi Flow/Ingest/Parse", breadcrumbPath(leaf))
}

func TestBreadcrumbPath_RootIsJustItsOwnName(t *testing.T) {
	assert.Equal(t, "NiFi Flow", breadcrumbPath(breadcrumbEntity{Breadcrumb: breadcrumb{ID: "a", Name: "NiFi Flow"}}))
}

func TestSimpleTypeName_StripsThePackage(t *testing.T) {
	assert.Equal(t, "PutS3Object", simpleTypeName("org.apache.nifi.processors.aws.s3.PutS3Object"))
}

func TestSimpleTypeName_LeavesABareNameAlone(t *testing.T) {
	assert.Equal(t, "PutS3Object", simpleTypeName("PutS3Object"))
}

func TestProcessorState_ReportsAStoppedInvalidProcessorAsInvalid(t *testing.T) {
	assert.Equal(t, "INVALID", processorState(processorComponent{State: "STOPPED", ValidationStatus: "INVALID"}))
}

func TestProcessorState_KeepsARunningProcessorRunning(t *testing.T) {
	assert.Equal(t, "RUNNING", processorState(processorComponent{State: "RUNNING", ValidationStatus: "VALID"}))
}

func TestProcessorState_KeepsAStoppedValidProcessorStopped(t *testing.T) {
	assert.Equal(t, "STOPPED", processorState(processorComponent{State: "STOPPED", ValidationStatus: "VALID"}))
}

func str(s string) *string { return &s }

func TestPublicProperties_MasksSensitiveValues(t *testing.T) {
	config := processorConfig{
		Properties:  map[string]*string{"Request Password": str("********"), "HTTP URL": str("https://hooks.example.com")},
		Descriptors: map[string]propertyDescriptor{"Request Password": {Sensitive: true}, "HTTP URL": {}},
	}

	props := publicProperties(config)

	assert.Equal(t, maskedValue, props["Request Password"])
	assert.Equal(t, "https://hooks.example.com", props["HTTP URL"])
}

func TestPublicProperties_DropsUnsetValues(t *testing.T) {
	config := processorConfig{
		Properties: map[string]*string{"Validation Query": nil, "Batch Size": str("")},
	}

	assert.Empty(t, publicProperties(config))
}

func TestPropertyValue_TriesTheNamesInOrder(t *testing.T) {
	config := processorConfig{Properties: map[string]*string{"topic": str("legacy"), "Topic Name": str("orders")}}

	assert.Equal(t, "orders", propertyValue(config, "Topic Name", "topic"))
	assert.Equal(t, "legacy", propertyValue(config, "topic", "Topic Name"))
}

func TestPropertyValue_SkipsExpressionLanguage(t *testing.T) {
	config := processorConfig{Properties: map[string]*string{"Bucket": str("${s3.bucket}")}}

	assert.Empty(t, propertyValue(config, "Bucket"))
}

func TestPropertyValue_SkipsParameterReferences(t *testing.T) {
	config := processorConfig{Properties: map[string]*string{"Bucket": str("#{landing_bucket}")}}

	assert.Empty(t, propertyValue(config, "Bucket"))
}

func TestPropertyValue_SkipsSensitiveValues(t *testing.T) {
	config := processorConfig{
		Properties:  map[string]*string{"Bucket": str("********")},
		Descriptors: map[string]propertyDescriptor{"Bucket": {Sensitive: true}},
	}

	assert.Empty(t, propertyValue(config, "Bucket"))
}

func TestPropertyValue_TrimsWhitespace(t *testing.T) {
	config := processorConfig{Properties: map[string]*string{"Bucket": str("  landing  ")}}

	assert.Equal(t, "landing", propertyValue(config, "Bucket"))
}

func TestRelationshipNames_AreSorted(t *testing.T) {
	assert.Equal(t, []string{"failure", "retry", "success"},
		relationshipNames([]relationship{{Name: "success"}, {Name: "failure"}, {Name: "retry"}}))
}
