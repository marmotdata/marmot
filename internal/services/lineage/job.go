package lineage

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/services/asset"
)

type Job struct {
	Namespace string                     `json:"namespace"`
	Name      string                     `json:"name"`
	Facets    map[string]json.RawMessage `json:"facets,omitempty"`
}

type JobType struct {
	Name        string
	Service     string
	MinVersion  string
	Facets      []string
	DataSources []string
}

var SupportedJobs = map[string]JobType{
	"AIRFLOW": {
		Name:       "Airflow",
		Service:    "airflow",
		MinVersion: "1.10",
		Facets: []string{
			"airflow_version",
			"airflow_runArgs",
		},
		DataSources: []string{
			"postgresql", "mysql", "snowflake", "athena", "redshift",
			"sagemaker", "bigquery", "gcs", "greatexpectations",
		},
	},
	"SPARK": {
		Name:       "Spark",
		Service:    "spark",
		MinVersion: "2.4",
		Facets: []string{
			"spark.logicalPlan",
			"spark.metrics",
		},
		DataSources: []string{
			"jdbc", "hdfs", "gcs", "bigquery", "s3",
			"azure_blob", "azure_datalake", "azure_synapse",
		},
	},
	"DBT": {
		Name:       "dbt",
		Service:    "dbt",
		MinVersion: "0.20",
		Facets: []string{
			"dbt_version",
			"sql",
		},
		DataSources: []string{
			"snowflake", "bigquery",
		},
	},
	"GREAT_EXPECTATIONS": {
		Name:       "Great Expectations",
		Service:    "great_expectations",
		MinVersion: "0.13",
		Facets: []string{
			"dataQualityMetrics",
			"dataQualityAssertions",
		},
		DataSources: []string{},
	},
	"ASYNCAPI": {
		Name:       "AsyncAPI",
		Service:    "asyncapi",
		MinVersion: "2.0.0",
		Facets: []string{
			"asyncapi",
		},
		DataSources: []string{
			"kafka", "aws", "rabbitmq", "mqtt",
		},
	},
}

type JobDetector struct {
	facets map[string]json.RawMessage
	job    *Job
}

func NewJobDetector(facets map[string]json.RawMessage, job *Job) *JobDetector {
	return &JobDetector{
		facets: facets,
		job:    job,
	}
}

func (d *JobDetector) DetectJob() (*JobType, error) {
	// First check if job type is explicitly known from jobType facet
	if jobType, ok := d.facets["jobType"]; ok {
		var jobTypeFacet struct {
			Integration string `json:"integration"`
		}
		if err := json.Unmarshal(jobType, &jobTypeFacet); err == nil {
			integrationUpper := strings.ToUpper(jobTypeFacet.Integration)
			if pipeline, ok := SupportedJobs[integrationUpper]; ok {
				pipelineCopy := pipeline // Create a copy we can take the address of
				return &pipelineCopy, nil
			}
		}
	}

	// Check for technology-specific facets
	for jobType, pipeline := range SupportedJobs {
		for _, facet := range pipeline.Facets {
			if _, ok := d.facets[facet]; ok {
				pipelineCopy := SupportedJobs[jobType] // Create a copy we can take the address of
				return &pipelineCopy, nil
			}
		}
	}

	// Check namespace hints
	if d.job != nil && d.job.Namespace != "" {
		namespace := strings.ToLower(d.job.Namespace)
		for jobType, pipeline := range SupportedJobs {
			if strings.Contains(namespace, strings.ToLower(pipeline.Service)) {
				pipelineCopy := SupportedJobs[jobType] // Create a copy we can take the address of
				return &pipelineCopy, nil
			}
		}
	}

	return nil, fmt.Errorf("unknown pipeline type")
}

func (d *JobDetector) ExtractMetadata() map[string]interface{} {
	metadata := make(map[string]interface{})

	// Extract SQL if present
	if sqlFacet, ok := d.facets["sql"]; ok {
		var sql struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(sqlFacet, &sql); err == nil {
			metadata["sql"] = sql.Query
		}
	}

	// Extract version information
	if versionFacet, ok := d.facets["dbt_version"]; ok {
		var version struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(versionFacet, &version); err == nil {
			metadata["version"] = version.Version
		}
	}

	// Extract Airflow metadata
	if airflowVersion, ok := d.facets["airflow_version"]; ok {
		var version struct {
			AirflowVersion string `json:"airflowVersion"`
		}
		if err := json.Unmarshal(airflowVersion, &version); err == nil {
			metadata["version"] = version.AirflowVersion
		}
	}

	return metadata
}

func (d *JobDetector) DetectAsset(pipeline *JobType, job *Job) *asset.CreateInput {
	name := strings.Split(job.Name, ".")[len(strings.Split(job.Name, "."))-1]
	mrn := fmt.Sprintf("mrn://job/%s/%s", pipeline.Service, name)

	return &asset.CreateInput{
		Name:      &name,
		Type:      "JOB",
		Providers: []string{pipeline.Service},
		MRN:       &mrn,
		CreatedBy: "system",
		Tags:      []string{strings.ToLower(pipeline.Service)},
		Metadata:  d.ExtractMetadata(),
	}
}
