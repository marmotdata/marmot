package lineage

import (
	"context"
	"encoding/json"
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
	AssetTypeTest    = "Test"
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

// RunEvent is one OpenLineage event. Despite the name it carries all three event types the spec defines: a RunEvent has a run and a job, a JobEvent a job and no run, and a DatasetEvent only a dataset. Kind says which.
type RunEvent struct {
	EventType string    `json:"eventType,omitempty"`
	EventTime time.Time `json:"eventTime"`
	Run       Run       `json:"run"`
	Job       Job       `json:"job"`
	Inputs    []Dataset `json:"inputs,omitempty"`
	Outputs   []Dataset `json:"outputs,omitempty"`
	// Dataset is the dataset a DatasetEvent describes.
	Dataset   *Dataset `json:"dataset,omitempty"`
	Producer  string   `json:"producer,omitempty"`
	SchemaURL string   `json:"schemaURL,omitempty"`

	kind         EventKind
	rawEventTime string
} // @name RunEvent

// UnmarshalJSON reads eventTime leniently, accepting a timestamp without an offset as UTC, and keeps the text so ParseEvent can say what was wrong with one it cannot read.
func (e *RunEvent) UnmarshalJSON(b []byte) error {
	type plain RunEvent
	aux := struct {
		*plain
		EventTime *string `json:"eventTime"`
	}{plain: (*plain)(e)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	if aux.EventTime != nil {
		e.rawEventTime = *aux.EventTime
		if t, err := parseEventTime(*aux.EventTime); err == nil {
			e.EventTime = t
		}
	}
	return nil
}

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

// ProcessOpenLineageEvent files one OpenLineage event: the job and the datasets it read and wrote become assets, linked by lineage, and a run event is recorded in the job's run history. A JobEvent files the job and its lineage with no run, and a DatasetEvent files only the dataset.
func (s *service) ProcessOpenLineageEvent(ctx context.Context, event *RunEvent, createdBy string) error {
	switch event.Kind() {
	case KindDataset:
		if _, err := s.processDatasetOutput(ctx, event.Dataset, event.Producer, createdBy, true); err != nil {
			return fmt.Errorf("failed to process dataset: %w", err)
		}
		return nil
	case KindJob:
		job, err := s.upsertJob(ctx, event, createdBy)
		if err != nil {
			return fmt.Errorf("failed to process job asset: %w", err)
		}
		return s.processLineage(ctx, event, job, createdBy, false)
	}

	job, err := s.upsertJob(ctx, event, createdBy)
	if err != nil {
		return fmt.Errorf("failed to process job asset: %w", err)
	}
	// A dataset is only renamed or dropped once the statement that does it has finished.
	if err := s.processLineage(ctx, event, job, createdBy, event.EventType == EventTypeComplete); err != nil {
		return err
	}
	if err := s.createProjectModelLineage(ctx, event, job); err != nil {
		return fmt.Errorf("failed to create project lineage: %w", err)
	}
	if err := s.createDAGTaskLineage(ctx, event, job, createdBy); err != nil {
		return fmt.Errorf("failed to create DAG-task lineage: %w", err)
	}
	if err := s.storeRunHistory(ctx, event, job.assetID); err != nil {
		return fmt.Errorf("failed to store run history: %w", err)
	}
	s.linkParentRuns(ctx, event, job)
	return nil
}

// linkParentRuns shows which job started which: a run's parent facet names the run that started it, such as a Spark application starting its jobs, a dbt invocation its models and tests, or an Airflow task a Spark application. Marmot knows which job recorded that run, so the parent job contains the child job, whatever produced either. Parents and children can arrive in either order, so a run also links the jobs whose runs named it as their parent.
func (s *service) linkParentRuns(ctx context.Context, event *RunEvent, job filedJob) {
	link := func(parent, child string) {
		if parent == "" || child == "" || parent == child {
			return
		}
		if _, err := s.CreateDirectLineage(ctx, parent, child, "CONTAINS", ""); err != nil {
			log.Warn().Err(err).Str("parent", parent).Str("child", child).Msg("Failed to link a job to the job that started it")
		}
	}
	if runID := parentRunID(event); runID != "" && runID != event.Run.RunID {
		parent, err := s.repo.RunJob(ctx, runID)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to find the parent run's job")
		}
		link(parent, job.mrn)
	}
	children, err := s.repo.ChildJobs(ctx, event.Run.RunID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to find the runs this run started")
	}
	for _, child := range children {
		link(job.mrn, child)
	}
}

// parentRunID reads the run id a run's parent facet names.
func parentRunID(event *RunEvent) string {
	parent := facetMap(event.Run.Facets, "parent")
	if parent == nil {
		parent = facetMap(event.Run.Facets, "parentRun")
	}
	run, _ := parent["run"].(map[string]interface{})
	id, _ := run["runId"].(string)
	return id
}

// filedJob is a job asset an event was filed against.
type filedJob struct {
	jobIdentity
	assetID string
	mrn     string
}

// filedDataset is a dataset an event was filed against: the event's own description of it and the MRN of the asset it is.
type filedDataset struct {
	dataset *Dataset
	mrn     string
}

// processLineage files the event's datasets and links them through the job: each input feeds the job and the job produces each output. Outputs with column lineage also get a direct edge from each input dataset carrying the column mappings.
func (s *service) processLineage(ctx context.Context, event *RunEvent, job filedJob, createdBy string, final bool) error {
	var inputs, outputs []filedDataset
	for i := range event.Inputs {
		in := &event.Inputs[i]
		m, err := s.upsertDataset(ctx, in, event.Producer, createdBy)
		if err != nil {
			log.Error().Err(err).Str("dataset", in.Namespace+"/"+in.Name).Msg("Failed to process input dataset")
			continue
		}
		inputs = append(inputs, filedDataset{dataset: in, mrn: m})
	}
	for i := range event.Outputs {
		out := &event.Outputs[i]
		m, err := s.processDatasetOutput(ctx, out, event.Producer, createdBy, final)
		if err != nil {
			log.Error().Err(err).Str("dataset", out.Namespace+"/"+out.Name).Msg("Failed to process output dataset")
			continue
		}
		if m != "" {
			outputs = append(outputs, filedDataset{dataset: out, mrn: m})
		}
	}

	for _, in := range inputs {
		if _, err := s.CreateDirectLineage(ctx, in.mrn, job.mrn, "CONSUMES", ""); err != nil {
			log.Warn().Err(err).Str("input_mrn", in.mrn).Str("job_mrn", job.mrn).Msg("Failed to create input lineage")
		}
	}
	for _, out := range outputs {
		if _, err := s.CreateDirectLineage(ctx, job.mrn, out.mrn, "PRODUCES", ""); err != nil {
			log.Warn().Err(err).Str("job_mrn", job.mrn).Str("output_mrn", out.mrn).Msg("Failed to create output lineage")
		}
	}
	return s.createColumnLineage(ctx, event, job, inputs, outputs)
}

// upsertJob files the job an event describes, creating it the first time and adding what this event says to it after that.
func (s *service) upsertJob(ctx context.Context, event *RunEvent, createdBy string) (filedJob, error) {
	id := identifyJob(event)
	target, err := s.assetSvc.ResolveMRN(ctx, id.MRN)
	if err != nil || target == "" {
		target = id.MRN
	}
	s.adoptLegacy(ctx, id.Legacy, target)

	fields := jobFields(event, id)
	source := asset.AssetSource{Name: "OpenLineage", LastSyncAt: event.EventTime, Properties: runSourceProperties(event), Priority: 1}
	existing, err := s.findAsset(ctx, target)
	if err != nil {
		return filedJob{}, err
	}
	if existing == nil {
		created, err := s.assetSvc.Create(ctx, asset.CreateInput{
			Name:          fields.Name,
			MRN:           &target,
			Type:          fields.Type,
			Providers:     fields.Providers,
			Description:   fields.Description,
			Metadata:      fields.Metadata,
			Tags:          fields.Tags,
			Query:         fields.Query,
			QueryLanguage: fields.QueryLanguage,
			CreatedBy:     createdBy,
			Writer:        openLineageWriter,
			Role:          asset.RoleNative,
			Sources:       []asset.AssetSource{source},
		})
		var exists *asset.ExistsError
		switch {
		case err == nil:
			return filedJob{jobIdentity: id, assetID: created.ID, mrn: *created.MRN}, nil
		case errors.As(err, &exists) && exists.Asset != nil:
			existing = exists.Asset
		default:
			return filedJob{}, fmt.Errorf("failed to create job asset: %w", err)
		}
	}
	s.contribute(ctx, existing, asset.RoleNative, fields, source)
	return filedJob{jobIdentity: id, assetID: existing.ID, mrn: *existing.MRN}, nil
}

// upsertDataset files a dataset an event mentions as a stub the first time, so lineage is complete without filling the catalog with undocumented assets, and adds what the event says to it after that. A dataset the system's own plugin already catalogued is that asset, and what OpenLineage says ranks below what the plugin says.
func (s *service) upsertDataset(ctx context.Context, ds *Dataset, producer, createdBy string) (string, error) {
	id := identifyDataset(ds, producer)
	target, err := s.assetSvc.ResolveMRN(ctx, id.MRN)
	if err != nil || target == "" {
		target = id.MRN
	}
	s.adoptLegacy(ctx, id.Legacy, target)
	target, aliases := s.adoptSymlinks(ctx, ds, producer, target)

	fields := datasetFields(ds, id)
	source := datasetSource(ds, id)
	existing, err := s.findAsset(ctx, target)
	if err != nil {
		return "", err
	}
	if existing == nil {
		created, err := s.assetSvc.Create(ctx, asset.CreateInput{
			Name:          fields.Name,
			MRN:           &target,
			Type:          fields.Type,
			Providers:     fields.Providers,
			Description:   fields.Description,
			Metadata:      fields.Metadata,
			Schema:        fields.Schema,
			Tags:          fields.Tags,
			Query:         fields.Query,
			QueryLanguage: fields.QueryLanguage,
			CreatedBy:     createdBy,
			IsStub:        true,
			Writer:        openLineageWriter,
			Role:          asset.RoleReference,
			Sources:       []asset.AssetSource{source},
		})
		var exists *asset.ExistsError
		switch {
		case err == nil:
			s.addAliases(ctx, created.ID, aliases)
			return *created.MRN, nil
		case errors.As(err, &exists) && exists.Asset != nil:
			existing = exists.Asset
		default:
			return "", fmt.Errorf("failed to create dataset asset: %w", err)
		}
	}
	s.contribute(ctx, existing, asset.RoleReference, fields, source)
	s.addAliases(ctx, existing.ID, aliases)
	return *existing.MRN, nil
}

// datasetSource is the OpenLineage source recorded on a dataset asset.
func datasetSource(ds *Dataset, id datasetIdentity) asset.AssetSource {
	return asset.AssetSource{
		Name:       "OpenLineage",
		LastSyncAt: time.Now(),
		Properties: map[string]interface{}{"namespace": ds.Namespace, "name": ds.Name, "provider": id.Provider, "asset_type": id.Type},
		Priority:   1,
	}
}

// findAsset looks an asset up by its current MRN, stubs included, returning nil when there is none.
func (s *service) findAsset(ctx context.Context, m string) (*asset.Asset, error) {
	c, err := asset.CanonicalMRN(m)
	if err != nil {
		return nil, nil
	}
	found, err := s.assetSvc.GetByExactMRNs(ctx, []string{c})
	if err != nil {
		return nil, fmt.Errorf("looking up %s: %w", m, err)
	}
	return found[c], nil
}

// contribute adds what an event says to an asset, on top of what earlier events said, and records OpenLineage as one of its sources. It logs rather than fails, so one asset that cannot be updated does not lose the rest of the event.
func (s *service) contribute(ctx context.Context, a *asset.Asset, role string, fields asset.ContributionFields, source asset.AssetSource) {
	var prev *asset.ContributionFields
	if contribs, err := s.assetSvc.Contributions(ctx, a.ID); err == nil {
		for i := range contribs {
			if contribs[i].Writer == openLineageWriter {
				prev = &contribs[i].Fields
				break
			}
		}
	}
	if _, err := s.assetSvc.Contribute(ctx, a.ID, openLineageWriter, role, mergeContribution(prev, fields), true); err != nil {
		log.Warn().Err(err).Str("asset_id", a.ID).Msg("Failed to update asset from OpenLineage event")
	}
	if _, err := s.assetSvc.Update(ctx, a.ID, asset.UpdateInput{Sources: []asset.AssetSource{source}, SkipNotification: true}); err != nil {
		log.Warn().Err(err).Str("asset_id", a.ID).Msg("Failed to record OpenLineage as a source of the asset")
	}
}

// ownedByOpenLineage reports whether OpenLineage is the only thing that has said anything about an asset, so Marmot may move, merge or delete it without losing anyone else's work.
func (s *service) ownedByOpenLineage(ctx context.Context, a *asset.Asset) bool {
	contribs, err := s.assetSvc.Contributions(ctx, a.ID)
	if err != nil || len(contribs) == 0 {
		return false
	}
	for _, c := range contribs {
		if c.Writer != openLineageWriter {
			return false
		}
	}
	return true
}

// adoptLegacy brings an asset an earlier release filed under an older MRN onto the MRN it has now, keeping its id and everything attached to it. It only touches assets OpenLineage alone made: one with nothing at the new MRN moves there, and one that turns out to duplicate the asset already there is merged into it.
func (s *service) adoptLegacy(ctx context.Context, legacy []string, current string) {
	for _, old := range legacy {
		oldMRN, err := asset.CanonicalMRN(old)
		if err != nil || oldMRN == current {
			continue
		}
		found, err := s.assetSvc.GetByExactMRNs(ctx, []string{oldMRN, current})
		if err != nil {
			return
		}
		was, ok := found[oldMRN]
		if !ok || !s.ownedByOpenLineage(ctx, was) {
			continue
		}
		if now, ok := found[current]; ok && now.ID != was.ID {
			if _, err := s.assetSvc.Merge(ctx, was.ID, now.ID); err != nil {
				log.Warn().Err(err).Str("from", oldMRN).Str("into", current).Msg("Failed to merge an asset from an earlier OpenLineage naming")
			}
			continue
		}
		if err := s.assetSvc.MoveMRN(ctx, was.ID, current, "openlineage-naming"); err != nil {
			log.Warn().Err(err).Str("from", oldMRN).Str("to", current).Msg("Failed to move an asset from an earlier OpenLineage naming")
		}
	}
}

// adoptSymlinks reads a dataset's symlinks facet, which lists other names the same dataset goes by, such as the Hive table over a set of files. When an asset already exists under one of those names and none under the dataset's own, the event is filed against that asset. When the dataset's own asset is an OpenLineage stub that duplicates one, the stub is merged in. It returns the MRN to file against and the symlink MRNs to record as aliases of it.
func (s *service) adoptSymlinks(ctx context.Context, ds *Dataset, producer, primary string) (string, []string) {
	f := facetMap(ds.Facets, "symlinks")
	if f == nil {
		return primary, nil
	}
	list, _ := f["identifiers"].([]interface{})
	var links []string
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		ns, _ := m["namespace"].(string)
		name, _ := m["name"].(string)
		if ns == "" || name == "" {
			continue
		}
		id := identifyDataset(&Dataset{Namespace: ns, Name: name}, producer)
		resolved, err := s.assetSvc.ResolveMRN(ctx, id.MRN)
		if err != nil || resolved == "" {
			resolved = id.MRN
		}
		if resolved != primary && !contains(links, resolved) {
			links = append(links, resolved)
		}
	}
	if len(links) == 0 {
		return primary, nil
	}
	found, err := s.assetSvc.GetByExactMRNs(ctx, append([]string{primary}, links...))
	if err != nil {
		return primary, links
	}
	own := found[primary]
	for _, l := range links {
		other, ok := found[l]
		if !ok || (own != nil && other.ID == own.ID) {
			continue
		}
		if own == nil {
			return *other.MRN, []string{primary}
		}
		if own.IsStub && s.ownedByOpenLineage(ctx, own) {
			if _, err := s.assetSvc.Merge(ctx, own.ID, other.ID); err != nil {
				log.Warn().Err(err).Str("stub", primary).Str("into", l).Msg("Failed to merge an OpenLineage stub into the asset its symlink names")
				return primary, nil
			}
			return *other.MRN, nil
		}
		log.Info().Str("dataset", primary).Str("symlink", l).Msg("Dataset and its symlink are separate assets; leaving both")
		return primary, nil
	}
	return primary, links
}

// addAliases records MRNs an asset also answers to, skipping any another asset already holds.
func (s *service) addAliases(ctx context.Context, assetID string, aliases []string) {
	for _, a := range aliases {
		if err := s.assetSvc.AddAlias(ctx, assetID, a, "openlineage-symlink"); err != nil && !errors.Is(err, asset.ErrAlreadyExists) {
			log.Warn().Err(err).Str("asset_id", assetID).Str("alias", a).Msg("Failed to record an OpenLineage symlink")
		}
	}
}

// createDAGTaskLineage links an Airflow task to the DAG its parent run facet names.
func (s *service) createDAGTaskLineage(ctx context.Context, event *RunEvent, job filedJob, createdBy string) error {
	if job.Provider != ProviderAirflow || job.Type != AssetTypeTask {
		return nil
	}
	parentNamespace, parentJobName, ok := parentJob(event)
	if !ok {
		return nil
	}
	dagMRN := openLineageMRN(AssetTypeDAG, job.Provider, parentNamespace, parentJobName)
	if err := s.ensureDAGAssetExists(ctx, dagMRN, parentJobName, parentNamespace, job.Provider, createdBy); err != nil {
		return err
	}
	if _, err := s.CreateDirectLineage(ctx, dagMRN, job.mrn, "CONTAINS", ""); err != nil {
		log.Warn().Err(err).Str("dag_mrn", dagMRN).Str("task_mrn", job.mrn).Msg("Failed to create DAG-task lineage")
	}
	return nil
}

// parentJob reads the job a run's parent facet names. Older Airflow integrations send the facet as parentRun.
func parentJob(event *RunEvent) (namespace, name string, ok bool) {
	parent := facetMap(event.Run.Facets, "parent")
	if parent == nil {
		parent = facetMap(event.Run.Facets, "parentRun")
	}
	job, _ := parent["job"].(map[string]interface{})
	namespace, _ = job["namespace"].(string)
	name, _ = job["name"].(string)
	return namespace, name, namespace != "" && name != ""
}

func (s *service) ensureDAGAssetExists(ctx context.Context, dagMRN, dagName, namespace, provider, createdBy string) error {
	if existing, err := s.findAsset(ctx, dagMRN); err == nil && existing != nil {
		return nil
	}
	desc := fmt.Sprintf("Airflow DAG from %s namespace", namespace)
	_, err := s.assetSvc.Create(ctx, asset.CreateInput{
		Name:        &dagName,
		MRN:         &dagMRN,
		Type:        AssetTypeDAG,
		Providers:   []string{provider},
		Description: &desc,
		Metadata:    map[string]interface{}{"openlineage_producer": "airflow", "job_type": AssetTypeDAG, "namespace": namespace},
		Tags:        []string{"openlineage", strings.ToLower(AssetTypeDAG), strings.ToLower(provider)},
		CreatedBy:   createdBy,
		Writer:      openLineageWriter,
		Role:        asset.RoleNative,
		Sources: []asset.AssetSource{{
			Name:       "OpenLineage",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{"dag_name": dagName, "namespace": namespace, "provider": provider},
			Priority:   1,
		}},
	})
	if errors.Is(err, asset.ErrAlreadyExists) {
		return nil
	}
	return err
}

// createProjectModelLineage links a dbt model to the dbt project run its parent run facet names. The project's MRN is built from its full job name, the same way the project's own events build it.
func (s *service) createProjectModelLineage(ctx context.Context, event *RunEvent, job filedJob) error {
	if job.Provider != ProviderDBT || job.Type != AssetTypeModel {
		return nil
	}
	parentNamespace, parentJobName, ok := parentJob(event)
	if !ok {
		return nil
	}
	projectMRN := openLineageMRN(AssetTypeProject, job.Provider, parentNamespace, parentJobName)
	if existing, err := s.findAsset(ctx, projectMRN); err != nil || existing == nil {
		return nil
	}
	if _, err := s.CreateDirectLineage(ctx, projectMRN, job.mrn, "CONTAINS", ""); err != nil {
		log.Warn().Err(err).Str("project_mrn", projectMRN).Str("model_mrn", job.mrn).Msg("Failed to create project-model lineage")
	}
	return nil
}

func (s *service) storeRunHistory(ctx context.Context, event *RunEvent, jobAssetID string) error {
	entry := &RunHistoryEntry{
		ID:           uuid.New().String(),
		AssetID:      jobAssetID,
		RunID:        event.Run.RunID,
		JobNamespace: event.Job.Namespace,
		JobName:      event.Job.Name,
		EventType:    normaliseEventType(event.EventType),
		EventTime:    event.EventTime,
		Producer:     event.Producer,
		RunFacets:    orEmpty(event.Run.Facets),
		JobFacets:    orEmpty(event.Job.Facets),
		Inputs:       event.Inputs,
		Outputs:      event.Outputs,
		CreatedAt:    time.Now(),
	}
	if entry.Inputs == nil {
		entry.Inputs = []Dataset{}
	}
	if entry.Outputs == nil {
		entry.Outputs = []Dataset{}
	}
	return s.repo.StoreRunHistory(ctx, entry)
}

// runSourceProperties is what the job's OpenLineage source records about the latest run.
func runSourceProperties(event *RunEvent) map[string]interface{} {
	return map[string]interface{}{
		"event_time": event.EventTime.Format(time.RFC3339),
		"run_id":     event.Run.RunID,
		"job_name":   event.Job.Name,
		"namespace":  event.Job.Namespace,
		"producer":   event.Producer,
	}
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
					case "TEST":
						return AssetTypeTest
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

// orEmpty stores an absent facet map as an empty object rather than JSON null.
func orEmpty(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	return m
}
