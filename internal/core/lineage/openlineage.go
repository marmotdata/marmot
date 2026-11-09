package lineage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

const (
	EventTypeStart    = "START"
	EventTypeRunning  = "RUNNING"
	EventTypeComplete = "COMPLETE"
	EventTypeFail     = "FAIL"
	EventTypeAbort    = "ABORT"
	EventTypeOther    = "OTHER"
)

const (
	AssetTypeQuery   = "Query"
	AssetTypeCommand = "Command"
	AssetTypeDAG     = "Dag"
	AssetTypeTask    = "Task"
	AssetTypeJob     = "Job"
	AssetTypeModel   = "Model"
	AssetTypeTable   = "Table"
	AssetTypeTopic   = "Topic"
	AssetTypeFile    = "File"
	AssetTypeBucket  = "Bucket"
	AssetTypeDataset = "Dataset"
	AssetTypeProject = "Project"
)

const (
	ProviderDBT         = "DBT"
	ProviderAirflow     = "Airflow"
	ProviderSpark       = "Spark"
	ProviderBigQuery    = "BigQuery"
	ProviderPostgreSQL  = "PostgreSQL"
	ProviderMySQL       = "MySQL"
	ProviderSQLServer   = "SQLServer"
	ProviderKafka       = "Kafka"
	ProviderS3          = "S3"
	ProviderGCS         = "GCS"
	ProviderAzure       = "Azure"
	ProviderOpenLineage = "OpenLineage"
)

type RunEvent struct {
	EventType string    `json:"eventType"`
	EventTime time.Time `json:"eventTime"`
	Run       Run       `json:"run"`
	Job       Job       `json:"job"`
	Inputs    []Dataset `json:"inputs,omitempty"`
	Outputs   []Dataset `json:"outputs,omitempty"`
	Producer  string    `json:"producer,omitempty"`
	SchemaURL string    `json:"schemaURL,omitempty"`
} // @name RunEvent

type Run struct {
	RunID  string                 `json:"runId"`
	Facets map[string]interface{} `json:"facets,omitempty"`
} // @name LineageRun

type Job struct {
	Namespace string                 `json:"namespace"`
	Name      string                 `json:"name"`
	Facets    map[string]interface{} `json:"facets,omitempty"`
} // @name Job

type Dataset struct {
	Namespace    string                 `json:"namespace"`
	Name         string                 `json:"name"`
	Facets       map[string]interface{} `json:"facets,omitempty"`
	InputFacets  map[string]interface{} `json:"inputFacets,omitempty"`
	OutputFacets map[string]interface{} `json:"outputFacets,omitempty"`
} // @name Dataset

type RunHistoryEntry struct {
	ID           string                 `json:"id"`
	AssetID      string                 `json:"asset_id"`
	RunID        string                 `json:"run_id"`
	JobNamespace string                 `json:"job_namespace"`
	JobName      string                 `json:"job_name"`
	EventType    string                 `json:"event_type"`
	EventTime    time.Time              `json:"event_time"`
	Producer     string                 `json:"producer,omitempty"`
	RunFacets    map[string]interface{} `json:"run_facets,omitempty"`
	JobFacets    map[string]interface{} `json:"job_facets,omitempty"`
	Inputs       []Dataset              `json:"inputs,omitempty"`
	Outputs      []Dataset              `json:"outputs,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

func (s *service) ProcessOpenLineageEvent(ctx context.Context, event *RunEvent, createdBy string) error {
	jobAssetMRN, err := s.processJobAsset(ctx, event, createdBy)
	if err != nil {
		return fmt.Errorf("failed to process job asset: %w", err)
	}

	provider := inferProvider(event.Producer)
	jobAssetType := inferJobType(event, provider)

	if err := s.processDatasets(ctx, event, jobAssetMRN, createdBy); err != nil {
		return fmt.Errorf("failed to process datasets: %w", err)
	}

	if err := s.createProjectModelLineage(ctx, event, jobAssetMRN, jobAssetType); err != nil {
		return fmt.Errorf("failed to create project lineage: %w", err)
	}

	if err := s.createDAGTaskLineage(ctx, event, jobAssetMRN, jobAssetType, provider, createdBy); err != nil {
		return fmt.Errorf("failed to create DAG-task lineage: %w", err)
	}

	jobAsset, err := s.assetSvc.GetByMRN(ctx, jobAssetMRN)
	if err != nil {
		return fmt.Errorf("failed to get job asset for run history: %w", err)
	}

	if err := s.storeRunHistory(ctx, event, jobAsset.ID); err != nil {
		return fmt.Errorf("failed to store run history: %w", err)
	}

	return nil
}

func (s *service) createDAGTaskLineage(ctx context.Context, event *RunEvent, jobAssetMRN string, jobAssetType string, provider string, createdBy string) error {
	if provider != ProviderAirflow {
		return nil
	}

	if jobAssetType == AssetTypeTask && event.Run.Facets != nil && event.Run.Facets["parent"] != nil {
		parentFacet, ok := event.Run.Facets["parent"].(map[string]interface{})
		if !ok {
			return nil
		}

		jobInfo, ok := parentFacet["job"].(map[string]interface{})
		if !ok {
			return nil
		}

		parentJobName, ok := jobInfo["name"].(string)
		if !ok {
			return nil
		}

		parentNamespace, ok := jobInfo["namespace"].(string)
		if !ok {
			return nil
		}

		dagMRN := openLineageMRN(AssetTypeDAG, provider, parentNamespace, parentJobName)

		err := s.ensureDAGAssetExists(ctx, dagMRN, parentJobName, parentNamespace, provider, createdBy)
		if err != nil {
			return err
		}

		if _, err := s.CreateDirectLineage(ctx, dagMRN, jobAssetMRN, "CONTAINS", ""); err != nil {
			log.Warn().Err(err).
				Str("dag_mrn", dagMRN).
				Str("task_mrn", jobAssetMRN).
				Msg("Failed to create DAG-task lineage")
		}
	}

	return nil
}

func (s *service) ensureDAGAssetExists(ctx context.Context, dagMRN, dagName, namespace, provider, createdBy string) error {
	_, err := s.assetSvc.GetByMRN(ctx, dagMRN)
	if err == nil {
		return nil
	}

	desc := fmt.Sprintf("Airflow DAG from %s namespace", namespace)
	metadata := make(map[string]interface{})
	metadata["openlineage_producer"] = "airflow"
	metadata["job_type"] = AssetTypeDAG
	metadata["namespace"] = namespace

	createInput := asset.CreateInput{
		Name:        &dagName,
		MRN:         &dagMRN,
		Type:        AssetTypeDAG,
		Providers:   []string{provider},
		Description: &desc,
		Metadata:    metadata,
		Tags:        []string{"openlineage", strings.ToLower(AssetTypeDAG), strings.ToLower(provider)},
		CreatedBy:   createdBy,
		Writer:      openLineageWriter,
		Role:        asset.RoleNative,
		Sources: []asset.AssetSource{{
			Name:       "OpenLineage",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"dag_name":  dagName,
				"namespace": namespace,
				"provider":  provider,
			},
			Priority: 1,
		}},
	}

	_, err = s.assetSvc.Create(ctx, createInput)
	return err
}

func (s *service) createProjectModelLineage(ctx context.Context, event *RunEvent, jobAssetMRN string, jobAssetType string) error {
	provider := inferProvider(event.Producer)
	if provider != ProviderDBT || jobAssetType != AssetTypeModel {
		return nil
	}

	if event.Run.Facets == nil || event.Run.Facets["parent"] == nil {
		return nil
	}

	parentFacet, ok := event.Run.Facets["parent"].(map[string]interface{})
	if !ok {
		return nil
	}

	jobInfo, ok := parentFacet["job"].(map[string]interface{})
	if !ok {
		return nil
	}

	parentJobName, ok := jobInfo["name"].(string)
	if !ok {
		return nil
	}

	parentNamespace, ok := jobInfo["namespace"].(string)
	if !ok {
		return nil
	}

	projectJobName := extractSimpleJobName(parentJobName)
	projectMRN := openLineageMRN(AssetTypeProject, provider, parentNamespace, projectJobName)

	if _, err := s.CreateDirectLineage(ctx, projectMRN, jobAssetMRN, "CONTAINS", ""); err != nil {
		log.Warn().Err(err).
			Str("project_mrn", projectMRN).
			Str("model_mrn", jobAssetMRN).
			Msg("Failed to create project-model lineage")
	}

	return nil
}

func (s *service) processJobAsset(ctx context.Context, event *RunEvent, createdBy string) (string, error) {
	provider := inferProvider(event.Producer)
	jobName := extractSimpleJobName(event.Job.Name)
	assetType := inferJobType(event, provider)

	jobMRN := openLineageMRN(assetType, provider, event.Job.Namespace, jobName)

	desc := fmt.Sprintf("%s from %s namespace", assetType, event.Job.Namespace)
	if event.Job.Facets != nil {
		if documentation, ok := event.Job.Facets["documentation"]; ok {
			if doc, ok := documentation.(map[string]interface{}); ok {
				if description, ok := doc["description"].(string); ok {
					desc = description
				}
			}
		}
	}

	metadata := make(map[string]interface{})
	metadata["openlineage_producer"] = event.Producer
	metadata["job_type"] = assetType
	metadata["namespace"] = event.Job.Namespace

	var query string
	var queryLanguage string
	if event.Job.Facets != nil {
		extractJobMetadata(metadata, event.Job.Facets, &query, &queryLanguage)
	}

	runMetadata := s.extractRunMetadata(event)

	var queryPtr *string
	var queryLanguagePtr *string
	if query != "" {
		queryPtr = &query
	}
	if queryLanguage != "" {
		queryLanguagePtr = &queryLanguage
	}

	createInput := asset.CreateInput{
		Name:          &jobName,
		MRN:           &jobMRN,
		Type:          assetType,
		Providers:     []string{provider},
		Description:   &desc,
		Metadata:      metadata,
		Tags:          []string{"openlineage", strings.ToLower(assetType), strings.ToLower(provider)},
		CreatedBy:     createdBy,
		Writer:        openLineageWriter,
		Role:          asset.RoleNative,
		Query:         queryPtr,
		QueryLanguage: queryLanguagePtr,
		Sources: []asset.AssetSource{{
			Name:       "OpenLineage",
			LastSyncAt: event.EventTime,
			Properties: runMetadata,
			Priority:   1,
		}},
	}

	_, err := s.assetSvc.Create(ctx, createInput)
	if err != nil {
		if errors.Is(err, asset.ErrAlreadyExists) {
			existingAsset, getErr := s.assetSvc.GetByMRN(ctx, jobMRN)
			if getErr != nil {
				return "", fmt.Errorf("failed to get existing asset: %w", getErr)
			}

			fields := asset.ContributionFields{Description: &desc, Metadata: metadata}
			if query != "" {
				fields.Query = &query
				fields.QueryLanguage = &queryLanguage
			}
			if _, updateErr := s.assetSvc.Contribute(ctx, existingAsset.ID, openLineageWriter, asset.RoleNative, fields, false); updateErr != nil {
				log.Warn().Err(updateErr).Str("asset_id", existingAsset.ID).Msg("Failed to update existing job asset")
			}
			if _, updateErr := s.assetSvc.Update(ctx, existingAsset.ID, asset.UpdateInput{Sources: []asset.AssetSource{{
				Name:       "OpenLineage",
				LastSyncAt: event.EventTime,
				Properties: runMetadata,
				Priority:   1,
			}}, SkipNotification: true}); updateErr != nil {
				log.Warn().Err(updateErr).Str("asset_id", existingAsset.ID).Msg("Failed to record OpenLineage as a source of the job asset")
			}

			return jobMRN, nil
		}
		return "", fmt.Errorf("failed to create job asset: %w", err)
	}

	return jobMRN, nil
}

func (s *service) processDatasets(ctx context.Context, event *RunEvent, jobAssetMRN string, createdBy string) error {
	var inputMRNs []string
	for _, input := range event.Inputs {
		mrn, err := s.processDatasetAsset(ctx, &input, "input", createdBy)
		if err != nil {
			log.Error().Err(err).
				Str("dataset", input.Namespace+"."+input.Name).
				Msg("Failed to process input dataset")
			continue
		}
		inputMRNs = append(inputMRNs, mrn)
	}

	var outputMRNs []string
	for _, output := range event.Outputs {
		mrn, err := s.processDatasetAsset(ctx, &output, "output", createdBy)
		if err != nil {
			log.Error().Err(err).
				Str("dataset", output.Namespace+"."+output.Name).
				Msg("Failed to process output dataset")
			continue
		}
		outputMRNs = append(outputMRNs, mrn)
	}

	for _, inputMRN := range inputMRNs {
		if _, err := s.CreateDirectLineage(ctx, inputMRN, jobAssetMRN, "CONSUMES", ""); err != nil {
			log.Warn().Err(err).
				Str("input_mrn", inputMRN).
				Str("job_mrn", jobAssetMRN).
				Msg("Failed to create input lineage")
		}
	}

	for _, outputMRN := range outputMRNs {
		if _, err := s.CreateDirectLineage(ctx, jobAssetMRN, outputMRN, "PRODUCES", ""); err != nil {
			log.Warn().Err(err).
				Str("job_mrn", jobAssetMRN).
				Str("output_mrn", outputMRN).
				Msg("Failed to create output lineage")
		}
	}

	return nil
}

func (s *service) processDatasetAsset(ctx context.Context, dataset *Dataset, role string, createdBy string) (string, error) {
	provider := inferDatasetProvider(dataset)
	assetType := inferDatasetType(dataset)

	name := dataset.Name
	namespace := dataset.Namespace

	datasetMRN := openLineageDatasetMRN(assetType, provider, namespace, name)
	if resolved, err := s.assetSvc.ResolveMRN(ctx, datasetMRN); err == nil && resolved != "" {
		datasetMRN = resolved
	}
	s.adoptLegacyStub(ctx, openLineageMRN(assetType, provider, namespace, name), datasetMRN)

	desc := fmt.Sprintf("%s from %s namespace (%s)", assetType, namespace, role)

	metadata := make(map[string]interface{})
	metadata["openlineage_role"] = role
	metadata["namespace"] = namespace

	var query string
	var queryLanguage string
	schema := extractDatasetMetadata(metadata, dataset.Facets, dataset.InputFacets, dataset.OutputFacets, role, &query, &queryLanguage)

	datasetRunMetadata := map[string]interface{}{
		"dataset_name": name,
		"namespace":    namespace,
		"role":         role,
		"provider":     provider,
		"asset_type":   assetType,
	}

	runMetadata := datasetRunMetadata

	existingAsset, err := s.assetSvc.GetByMRN(ctx, datasetMRN)
	if err == nil {
		// OpenLineage only mentions a dataset, so what it says ranks below the system that owns it and never overwrites that system's metadata or schema.
		fields := asset.ContributionFields{Metadata: metadata, Schema: schema}
		if query != "" {
			fields.Query = &query
			fields.QueryLanguage = &queryLanguage
		}
		if _, updateErr := s.assetSvc.Contribute(ctx, existingAsset.ID, openLineageWriter, asset.RoleReference, fields, false); updateErr != nil {
			log.Warn().Err(updateErr).Str("asset_id", existingAsset.ID).Msg("Failed to update existing dataset asset")
		}
		if _, updateErr := s.assetSvc.Update(ctx, existingAsset.ID, asset.UpdateInput{Sources: []asset.AssetSource{{
			Name:       "OpenLineage",
			LastSyncAt: time.Now(),
			Properties: runMetadata,
			Priority:   1,
		}}, SkipNotification: true}); updateErr != nil {
			log.Warn().Err(updateErr).Str("asset_id", existingAsset.ID).Msg("Failed to record OpenLineage as a source of the dataset asset")
		}

		return *existingAsset.MRN, nil
	}

	createInput := asset.CreateInput{
		Name:        &name,
		MRN:         &datasetMRN,
		Type:        assetType,
		Providers:   []string{provider},
		Description: &desc,
		Metadata:    metadata,
		Schema:      schema,
		Tags:        []string{"openlineage", strings.ToLower(assetType), role, strings.ToLower(provider)},
		CreatedBy:   createdBy,
		IsStub:      true,
		Writer:      openLineageWriter,
		Role:        asset.RoleReference,
		Sources: []asset.AssetSource{{
			Name:       "OpenLineage",
			LastSyncAt: time.Now(),
			Properties: runMetadata,
			Priority:   1,
		}},
	}

	if query != "" {
		createInput.Query = &query
		createInput.QueryLanguage = &queryLanguage
	}

	_, err = s.assetSvc.Create(ctx, createInput)
	if err != nil {
		// A stub from an earlier event is hidden from GetByMRN, so seeing it again is not a failure: the dataset is already there.
		var exists *asset.ExistsError
		if errors.As(err, &exists) && exists.Asset != nil && exists.Asset.MRN != nil {
			return *exists.Asset.MRN, nil
		}
		return "", fmt.Errorf("failed to create stub asset: %w", err)
	}

	return datasetMRN, nil
}

func (s *service) storeRunHistory(ctx context.Context, event *RunEvent, jobAssetID string) error {
	historyID := uuid.New().String()

	entry := &RunHistoryEntry{
		ID:           historyID,
		AssetID:      jobAssetID,
		RunID:        event.Run.RunID,
		JobNamespace: event.Job.Namespace,
		JobName:      event.Job.Name,
		EventType:    event.EventType,
		EventTime:    event.EventTime,
		Producer:     event.Producer,
		RunFacets:    event.Run.Facets,
		JobFacets:    event.Job.Facets,
		Inputs:       event.Inputs,
		Outputs:      event.Outputs,
		CreatedAt:    time.Now(),
	}

	return s.repo.StoreRunHistory(ctx, entry)
}

func (s *service) extractRunMetadata(event *RunEvent) map[string]interface{} {
	runMetadata := map[string]interface{}{
		"event_time": event.EventTime.Format(time.RFC3339),
		"run_id":     event.Run.RunID,
		"job_name":   event.Job.Name,
		"namespace":  event.Job.Namespace,
		"producer":   event.Producer,
	}

	return runMetadata
}

func extractJobMetadata(metadata map[string]interface{}, facets map[string]interface{}, query *string, queryLanguage *string) {
	if facets == nil {
		return
	}

	if sql, ok := facets["sql"]; ok {
		if sqlData, ok := sql.(map[string]interface{}); ok {
			if q, ok := sqlData["query"].(string); ok {
				*query = q
				*queryLanguage = "sql"
			}
		}
	}

	if sourceCode, ok := facets["sourceCode"]; ok {
		if sourceCodeData, ok := sourceCode.(map[string]interface{}); ok {
			if code, ok := sourceCodeData["sourceCode"].(string); ok {
				*query = code
				*queryLanguage = "python"
			}
		}
	}

	if sourceCodeLocation, ok := facets["sourceCodeLocation"]; ok {
		if scl, ok := sourceCodeLocation.(map[string]interface{}); ok {
			if url, ok := scl["url"].(string); ok {
				metadata["source_code_url"] = url
			}
		}
	}

	if ownership, ok := facets["ownership"]; ok {
		if own, ok := ownership.(map[string]interface{}); ok {
			if owners, ok := own["owners"].([]interface{}); ok && len(owners) > 0 {
				if owner, ok := owners[0].(map[string]interface{}); ok {
					if name, ok := owner["name"].(string); ok {
						metadata["owner"] = name
					}
				}
			}
		}
	}

	if airflowFacet, ok := facets["airflow"]; ok {
		if airflowData, ok := airflowFacet.(map[string]interface{}); ok {
			if tasks, ok := airflowData["tasks"].(map[string]interface{}); ok {
				var taskNames []string
				for taskName := range tasks {
					taskNames = append(taskNames, taskName)
				}
				if len(taskNames) > 0 {
					metadata["tasks"] = taskNames
				}
			}
		}
	}
}

func extractDatasetMetadata(metadata map[string]interface{}, facets, inputFacets, outputFacets map[string]interface{}, role string, query *string, queryLanguage *string) map[string]string {
	var schema map[string]string

	if facets != nil {
		if sql, ok := facets["sql"]; ok {
			if sqlData, ok := sql.(map[string]interface{}); ok {
				if q, ok := sqlData["query"].(string); ok {
					*query = q
					*queryLanguage = "sql"
				}
			}
		}

		if schemaFacet, ok := facets["schema"]; ok {
			if schemaData, ok := schemaFacet.(map[string]interface{}); ok {
				schema = convertSchemaToJSONSchema(schemaData)
			}
		}

		if dataSource, ok := facets["dataSource"]; ok {
			if ds, ok := dataSource.(map[string]interface{}); ok {
				if name, ok := ds["name"].(string); ok {
					metadata["data_source"] = name
				}
			}
		}

		if version, ok := facets["version"]; ok {
			if v, ok := version.(map[string]interface{}); ok {
				if datasetVersion, ok := v["datasetVersion"].(string); ok {
					metadata["version"] = datasetVersion
				}
			}
		}

		if ownership, ok := facets["ownership"]; ok {
			if own, ok := ownership.(map[string]interface{}); ok {
				if owners, ok := own["owners"].([]interface{}); ok && len(owners) > 0 {
					if owner, ok := owners[0].(map[string]interface{}); ok {
						if name, ok := owner["name"].(string); ok {
							metadata["owner"] = name
						}
					}
				}
			}
		}
	}

	if role == "input" && inputFacets != nil {
		if inputStats, ok := inputFacets["inputStatistics"]; ok {
			if stats, ok := inputStats.(map[string]interface{}); ok {
				if rowCount, ok := stats["rowCount"].(float64); ok {
					metadata["input_row_count"] = int64(rowCount)
				}
			}
		}
	}

	if role == "output" && outputFacets != nil {
		if outputStats, ok := outputFacets["outputStatistics"]; ok {
			if stats, ok := outputStats.(map[string]interface{}); ok {
				if rowCount, ok := stats["rowCount"].(float64); ok {
					metadata["output_row_count"] = int64(rowCount)
				}
			}
		}
	}

	return schema
}

// openLineageMRN builds the MRN for an asset an OpenLineage event describes.
// A namespace is whatever the producer chose to send, commonly a connection
// URI or a scheduler name, so it goes through mrn.New rather than into the
// string directly: mrn.New is what the rest of Marmot uses to turn a type,
// a provider and a name into an identity, and it is the only thing that
// guarantees no space or stray slash reaches the MRN.
func openLineageMRN(assetType, provider, namespace, name string) string {
	return mrn.New(assetType, provider, namespace+"."+name)
}

// openLineageWriter is the contribution OpenLineage events make to the assets they describe.
const openLineageWriter = "openlineage"

// defaultPorts fills in the port of a namespace that leaves it out, so the host hint matches the host:port the system's own plugin reported.
var defaultPorts = map[string]string{
	"postgres": "5432", "postgresql": "5432", "mysql": "3306", "mariadb": "3306", "sqlserver": "1433", "mssql": "1433",
	"kafka": "9092", "trino": "8080", "presto": "8080", "clickhouse": "8123", "oracle": "1521", "redshift": "5439",
}

// singlePartSchemes are namespaces whose dataset name is one opaque name, such as a Kafka topic or an object key, rather than dotted levels.
var singlePartSchemes = map[string]bool{"kafka": true, "s3": true, "s3a": true, "gs": true, "gcs": true, "abfss": true, "wasbs": true, "hdfs": true, "file": true, "dbfs": true}

// openLineageDatasetMRN names a dataset from an OpenLineage event the way the system's own plugin names it. OpenLineage identifies a dataset by a namespace, the data source's address such as postgres://pg-eu:5432, and a name, the dotted path inside it, so the address becomes a host hint Marmot resolves to a zone and the name becomes the path. A namespace that is not an address, such as a scheduler's name, keeps the namespace in the name as before.
func openLineageDatasetMRN(assetType, provider, namespace, name string) string {
	u, err := url.Parse(namespace)
	if err != nil || u.Scheme == "" {
		if strings.EqualFold(namespace, "bigquery") {
			return mrn.Asset(assetType, provider, strings.Split(name, ".")...).String()
		}
		return openLineageMRN(assetType, provider, namespace, name)
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Host)
	if host != "" && u.Port() == "" {
		if port, ok := defaultPorts[scheme]; ok {
			host += ":" + port
		}
	}

	var m mrn.MRN
	switch {
	case scheme == "s3" || scheme == "s3a" || scheme == "gs" || scheme == "gcs":
		bucket := u.Host
		key := strings.TrimPrefix(name, "/")
		if key == "" {
			m = mrn.Asset(assetType, provider, bucket)
		} else {
			m = mrn.Asset(assetType, provider, bucket, key)
		}
		return m.String()
	case singlePartSchemes[scheme]:
		m = mrn.Asset(assetType, provider, name)
	default:
		m = mrn.Asset(assetType, provider, strings.Split(name, ".")...)
	}
	if host != "" {
		m = m.WithHost(host)
	}
	return m.String()
}

// adoptLegacyStub moves a dataset stub an older Marmot named by its whole namespace onto the MRN the dataset has now, or folds it into the asset already there, so its lineage and anything people added to it carry over.
func (s *service) adoptLegacyStub(ctx context.Context, legacyMRN, currentMRN string) {
	if legacyMRN == currentMRN {
		return
	}
	found, err := s.assetSvc.GetByMRNs(ctx, []string{legacyMRN, currentMRN})
	if err != nil {
		return
	}
	stub, ok := found[legacyMRN]
	if !ok || stub.MRN == nil || *stub.MRN != legacyMRN {
		return
	}
	if target, ok := found[currentMRN]; ok && target.ID != stub.ID {
		if !stub.IsStub {
			return
		}
		if _, err := s.assetSvc.Merge(ctx, stub.ID, target.ID); err != nil {
			log.Warn().Err(err).Str("stub", legacyMRN).Str("into", currentMRN).Msg("Failed to merge legacy OpenLineage stub")
		}
		return
	}
	if err := s.assetSvc.MoveMRN(ctx, stub.ID, currentMRN, "openlineage-naming"); err != nil {
		log.Warn().Err(err).Str("stub", legacyMRN).Str("to", currentMRN).Msg("Failed to move legacy OpenLineage stub")
	}
}

func inferJobType(event *RunEvent, provider string) string {
	if event.Job.Facets != nil {
		if jobFacet, ok := event.Job.Facets["jobType"]; ok {
			if jobData, ok := jobFacet.(map[string]interface{}); ok {
				if jobType, ok := jobData["jobType"].(string); ok {
					switch strings.ToUpper(jobType) {
					case "QUERY":
						return AssetTypeQuery
					case "COMMAND":
						return AssetTypeCommand
					case "DAG":
						return AssetTypeDAG
					case "TASK":
						return AssetTypeTask
					case "MODEL":
						return AssetTypeModel
					case "PROJECT":
						return AssetTypeProject
					case "JOB":
						if provider == ProviderDBT &&
							strings.HasPrefix(event.Job.Name, "dbt-run-") &&
							(event.Run.Facets == nil || event.Run.Facets["parent"] == nil) {
							return AssetTypeProject
						}
						return AssetTypeJob
					}
				}
			}
		}
	}

	switch provider {
	case ProviderDBT:
		return AssetTypeModel
	case ProviderAirflow:
		return AssetTypeDAG
	case ProviderSpark:
		return AssetTypeJob
	default:
		return AssetTypeJob
	}
}

func inferProvider(producer string) string {
	producer = strings.ToLower(producer)
	if strings.Contains(producer, "airflow") {
		return ProviderAirflow
	}
	if strings.Contains(producer, "spark") {
		return ProviderSpark
	}
	if strings.Contains(producer, "dbt") {
		return ProviderDBT
	}
	return ProviderOpenLineage
}

func inferDatasetProvider(dataset *Dataset) string {
	namespace := strings.ToLower(dataset.Namespace)

	if strings.Contains(namespace, "bigquery") || strings.Contains(namespace, "bq") {
		return ProviderBigQuery
	}
	if strings.Contains(namespace, "postgres") {
		return ProviderPostgreSQL
	}
	if strings.Contains(namespace, "mysql") {
		return ProviderMySQL
	}
	if strings.Contains(namespace, "sqlserver") {
		return ProviderSQLServer
	}
	if strings.Contains(namespace, "kafka") {
		return ProviderKafka
	}
	if strings.Contains(namespace, "s3") {
		return ProviderS3
	}
	if strings.Contains(namespace, "gcs") {
		return ProviderGCS
	}
	if strings.Contains(namespace, "azure") {
		return ProviderAzure
	}
	return ProviderOpenLineage
}

func inferDatasetType(dataset *Dataset) string {
	namespace := strings.ToLower(dataset.Namespace)
	name := strings.ToLower(dataset.Name)

	if strings.Contains(namespace, "postgres") || strings.Contains(namespace, "mysql") ||
		strings.Contains(namespace, "sqlserver") || strings.Contains(namespace, "bigquery") {
		return AssetTypeTable
	}
	if strings.Contains(namespace, "kafka") {
		return AssetTypeTopic
	}
	if strings.Contains(namespace, "s3") || strings.Contains(namespace, "gcs") || strings.Contains(namespace, "azure") {
		if strings.Contains(name, ".parquet") || strings.Contains(name, ".csv") {
			return AssetTypeFile
		}
		return AssetTypeBucket
	}
	if strings.Contains(name, ".parquet") || strings.Contains(name, ".csv") {
		return AssetTypeFile
	}

	return AssetTypeDataset
}

func convertSchemaToJSONSchema(schemaData map[string]interface{}) map[string]string {
	result := make(map[string]string)
	if fields, ok := schemaData["fields"].([]interface{}); ok {
		for _, field := range fields {
			if fieldMap, ok := field.(map[string]interface{}); ok {
				if name, ok := fieldMap["name"].(string); ok {
					if fieldType, ok := fieldMap["type"].(string); ok {
						jsonSchemaType := convertToJSONSchemaType(fieldType)
						result[name] = jsonSchemaType
					}
				}
			}
		}
	}
	return result
}

func convertToJSONSchemaType(olType string) string {
	switch strings.ToLower(olType) {
	case "string", "varchar", "char", "text":
		return "string"
	case "integer", "int", "bigint", "smallint":
		return "integer"
	case "double", "float", "decimal", "numeric":
		return "number"
	case "boolean", "bool":
		return "boolean"
	case "timestamp", "datetime", "date", "time":
		return "string"
	case "array":
		return "array"
	case "object", "struct", "record":
		return "object"
	default:
		return "string"
	}
}

func extractSimpleJobName(fullJobName string) string {
	if strings.Contains(fullJobName, ".") {
		parts := strings.Split(fullJobName, ".")
		return parts[len(parts)-1]
	}

	if strings.HasPrefix(fullJobName, "dbt-run-") {
		return strings.TrimPrefix(fullJobName, "dbt-run-")
	}

	return fullJobName
}
