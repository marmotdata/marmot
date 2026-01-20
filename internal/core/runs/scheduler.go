package runs

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/crypto"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

const (
	DefaultSchedulerInterval = 1 * time.Minute
	DefaultMaxWorkers        = 10
	DefaultLeaseExpiry       = 5 * time.Minute
	DefaultClaimExpiry       = 30 * time.Second
)

type Scheduler struct {
	service     *ScheduleService
	runsService Service
	encryptor   *crypto.Encryptor
	registry    *plugin.Registry

	maxWorkers        int
	schedulerInterval time.Duration
	leaseExpiry       time.Duration
	claimExpiry       time.Duration

	jobQueue      chan *JobRun
	semaphore     chan struct{}
	activeWorkers atomic.Int32

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type SchedulerConfig struct {
	MaxWorkers        int
	SchedulerInterval time.Duration
	LeaseExpiry       time.Duration
	ClaimExpiry       time.Duration
}

func NewScheduler(service *ScheduleService, runsService Service, encryptor *crypto.Encryptor, registry *plugin.Registry, config *SchedulerConfig) *Scheduler {
	if config == nil {
		config = &SchedulerConfig{}
	}

	maxWorkers := config.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = DefaultMaxWorkers
	}

	schedulerInterval := config.SchedulerInterval
	if schedulerInterval <= 0 {
		schedulerInterval = DefaultSchedulerInterval
	}

	leaseExpiry := config.LeaseExpiry
	if leaseExpiry <= 0 {
		leaseExpiry = DefaultLeaseExpiry
	}

	claimExpiry := config.ClaimExpiry
	if claimExpiry <= 0 {
		claimExpiry = DefaultClaimExpiry
	}

	return &Scheduler{
		service:           service,
		runsService:       runsService,
		encryptor:         encryptor,
		registry:          registry,
		maxWorkers:        maxWorkers,
		schedulerInterval: schedulerInterval,
		leaseExpiry:       leaseExpiry,
		claimExpiry:       claimExpiry,
		jobQueue:          make(chan *JobRun, 100),
		semaphore:         make(chan struct{}, maxWorkers),
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.jobDispatcher()
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.schedulerLoop()
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.leaseCleanupLoop()
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.pendingJobsPoller()
	}()

	log.Info().
		Int("max_workers", s.maxWorkers).
		Dur("interval", s.schedulerInterval).
		Msg("Ingestion scheduler started")

	return nil
}

func (s *Scheduler) Stop() {
	log.Info().Msg("Stopping ingestion scheduler...")

	if s.cancel != nil {
		s.cancel()
	}

	close(s.jobQueue)
	s.wg.Wait()

	log.Info().Msg("Ingestion scheduler stopped")
}

func (s *Scheduler) jobDispatcher() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case job, ok := <-s.jobQueue:
			if !ok {
				return
			}

			s.semaphore <- struct{}{}
			s.activeWorkers.Add(1)

			go func(j *JobRun) {
				defer func() {
					if r := recover(); r != nil {
						log.Error().
							Interface("panic", r).
							Str("run_id", j.ID).
							Msg("Worker panic recovered")
						errorMsg := fmt.Sprintf("Worker panic: %v", r)
						_ = s.service.CompleteJobRun(context.Background(), j.ID, false, &errorMsg, 0, 0, 0, 0, 0)
					}
					<-s.semaphore
					s.activeWorkers.Add(-1)
				}()

				worker := newWorker(s.service, s.runsService, s.encryptor, s.registry)
				if err := worker.executeJob(s.ctx, j); err != nil {
					log.Error().
						Err(err).
						Str("run_id", j.ID).
						Msg("Worker failed to execute job")
				}
			}(job)
		}
	}
}

func (s *Scheduler) schedulerLoop() {
	ticker := time.NewTicker(s.schedulerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.processSchedules(); err != nil {
				log.Error().Err(err).Msg("Error processing schedules")
			}
		}
	}
}

func (s *Scheduler) processSchedules() error {
	ctx := context.Background()

	schedules, err := s.service.GetSchedulesDueForRun(ctx, 100)
	if err != nil {
		return fmt.Errorf("getting due schedules: %w", err)
	}

	for _, schedule := range schedules {
		run, err := s.service.CreateJobRun(ctx, &schedule.ID, "scheduler")
		if err != nil {
			log.Error().
				Err(err).
				Str("schedule_id", schedule.ID).
				Str("schedule_name", schedule.Name).
				Msg("Failed to create job run")
			continue
		}

		log.Info().
			Str("schedule_id", schedule.ID).
			Str("schedule_name", schedule.Name).
			Str("run_id", run.ID).
			Msg("Created job run for schedule")

		nextRun, err := s.service.CalculateNextRun(schedule.CronExpression, time.Now())
		if err != nil {
			log.Error().
				Err(err).
				Str("schedule_id", schedule.ID).
				Msg("Failed to calculate next run time")
			continue
		}

		if err := s.service.UpdateScheduleNextRun(ctx, schedule.ID, nextRun); err != nil {
			log.Error().
				Err(err).
				Str("schedule_id", schedule.ID).
				Msg("Failed to update next run time")
		}
	}

	return nil
}

func (s *Scheduler) pendingJobsPoller() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkPendingJobs()
		}
	}
}

func (s *Scheduler) checkPendingJobs() {
	ctx := context.Background()
	status := JobStatusPending

	runs, _, err := s.service.ListJobRuns(ctx, nil, &status, 50, 0)
	if err != nil {
		log.Error().Err(err).Msg("Error listing pending jobs")
		return
	}

	for _, run := range runs {
		select {
		case s.jobQueue <- run:
		default:
			log.Warn().Str("run_id", run.ID).Msg("Job queue full, skipping")
		}
	}
}

func (s *Scheduler) leaseCleanupLoop() {
	ticker := time.NewTicker(s.claimExpiry)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			ctx := context.Background()
			released, err := s.service.ReleaseExpiredClaims(ctx, s.leaseExpiry)
			if err != nil {
				log.Error().Err(err).Msg("Error releasing expired claims")
			} else if released > 0 {
				log.Warn().Int("count", released).Msg("Released expired job claims")
			}
		}
	}
}

type worker struct {
	service     *ScheduleService
	runsService Service
	encryptor   *crypto.Encryptor
	registry    *plugin.Registry
}

func newWorker(service *ScheduleService, runsService Service, encryptor *crypto.Encryptor, registry *plugin.Registry) *worker {
	return &worker{
		service:     service,
		runsService: runsService,
		encryptor:   encryptor,
		registry:    registry,
	}
}

func (w *worker) executeJob(ctx context.Context, run *JobRun) error {
	if run.ScheduleID == nil {
		return fmt.Errorf("job run has no schedule_id")
	}

	log.Info().
		Str("run_id", run.ID).
		Str("schedule_id", *run.ScheduleID).
		Msg("Executing job run")

	if err := w.service.StartJobRun(ctx, run.ID); err != nil {
		return fmt.Errorf("starting job run: %w", err)
	}

	schedule, err := w.service.GetSchedule(ctx, *run.ScheduleID)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get schedule: %v", err)
		_ = w.service.CompleteJobRun(ctx, run.ID, false, &errorMsg, 0, 0, 0, 0, 0)
		return fmt.Errorf("getting schedule: %w", err)
	}

	if err := DecryptScheduleConfig(schedule, w.encryptor); err != nil {
		errorMsg := fmt.Sprintf("Failed to decrypt config: %v", err)
		_ = w.service.CompleteJobRun(ctx, run.ID, false, &errorMsg, 0, 0, 0, 0, 0)
		return fmt.Errorf("decrypting config: %w", err)
	}

	source, err := w.registry.GetSource(schedule.PluginID)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get plugin source: %v", err)
		_ = w.service.CompleteJobRun(ctx, run.ID, false, &errorMsg, 0, 0, 0, 0, 0)
		return fmt.Errorf("getting plugin source: %w", err)
	}

	validatedConfig, err := source.Validate(schedule.Config)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to validate plugin config: %v", err)
		_ = w.service.CompleteJobRun(ctx, run.ID, false, &errorMsg, 0, 0, 0, 0, 0)
		return fmt.Errorf("validating plugin config: %w", err)
	}

	pluginRun, err := w.runsService.StartRun(ctx, schedule.Name, schedule.PluginID, run.CreatedBy, validatedConfig)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to start run: %v", err)
		_ = w.service.CompleteJobRun(ctx, run.ID, false, &errorMsg, 0, 0, 0, 0, 0)
		return fmt.Errorf("starting run: %w", err)
	}

	// Link the plugin run to the job run for entity tracking
	if err := w.service.SetJobRunPluginRunID(ctx, run.ID, pluginRun.ID); err != nil {
		log.Warn().Err(err).Msg("Failed to set plugin run ID on job run")
	}

	result, err := source.Discover(ctx, validatedConfig)
	if err != nil {
		errorMsg := fmt.Sprintf("Plugin discovery failed: %v", err)
		_ = w.service.CompleteJobRun(ctx, run.ID, false, &errorMsg, 0, 0, 0, 0, 0)
		_ = w.runsService.CompleteRun(ctx, pluginRun.RunID, plugin.StatusFailed, nil, err.Error())
		return fmt.Errorf("executing plugin: %w", err)
	}

	assetsInput := make([]CreateAssetInput, 0, len(result.Assets))
	for _, a := range result.Assets {
		name := ""
		if a.Name != nil {
			name = *a.Name
		}

		schema := make(map[string]interface{})
		for k, v := range a.Schema {
			schema[k] = v
		}

		sources := make([]string, len(a.Sources))
		for j, source := range a.Sources {
			sources[j] = source.Name
		}

		assetsInput = append(assetsInput, CreateAssetInput{
			Name:          name,
			MRN:           a.MRN,
			Type:          a.Type,
			Providers:     a.Providers,
			Description:   a.Description,
			Metadata:      a.Metadata,
			Schema:        schema,
			Tags:          a.Tags,
			Sources:       sources,
			ExternalLinks: convertAssetExternalLinks(a.ExternalLinks),
			Query:         a.Query,
			QueryLanguage: a.QueryLanguage,
		})
	}

	lineageInput := make([]LineageInput, 0, len(result.Lineage))
	for _, l := range result.Lineage {
		lineageInput = append(lineageInput, LineageInput{
			Source: l.Source,
			Target: l.Target,
			Type:   l.Type,
		})
	}

	docsInput := make([]DocumentationInput, 0, len(result.Documentation))
	for _, d := range result.Documentation {
		docsInput = append(docsInput, DocumentationInput{
			AssetMRN: d.MRN,
			Content:  d.Content,
			Type:     d.Source,
		})
	}

	statsInput := make([]StatisticInput, 0, len(result.Statistics))
	for _, s := range result.Statistics {
		statsInput = append(statsInput, StatisticInput{
			AssetMRN:   s.AssetMRN,
			MetricName: s.MetricName,
			Value:      s.Value,
		})
	}

	response, err := w.runsService.ProcessEntities(
		ctx,
		pluginRun.RunID,
		assetsInput,
		lineageInput,
		docsInput,
		statsInput,
		schedule.Name,
		schedule.PluginID,
	)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to process entities: %v", err)
		_ = w.service.CompleteJobRun(ctx, run.ID, false, &errorMsg, 0, 0, 0, 0, 0)
		_ = w.runsService.CompleteRun(ctx, pluginRun.RunID, plugin.StatusFailed, nil, err.Error())
		return fmt.Errorf("processing entities: %w", err)
	}

	assetsCreated := 0
	assetsUpdated := 0
	for _, ar := range response.Assets {
		switch ar.Status {
		case StatusCreated:
			assetsCreated++
		case StatusUpdated:
			assetsUpdated++
		}
	}

	lineageCreated := 0
	for _, lr := range response.Lineage {
		if lr.Status == StatusCreated {
			lineageCreated++
		}
	}

	docsAdded := 0
	for _, dr := range response.Documentation {
		if dr.Status == StatusCreated {
			docsAdded++
		}
	}

	runHistoryStored := 0
	if len(result.RunHistory) > 0 {
		var runHistoryInputs []RunHistoryInput
		for _, arh := range result.RunHistory {
			for _, run := range arh.Runs {
				runHistoryInputs = append(runHistoryInputs, RunHistoryInput{
					AssetMRN:     arh.AssetMRN,
					RunID:        run.RunID,
					JobNamespace: run.JobNamespace,
					JobName:      run.JobName,
					EventType:    run.EventType,
					EventTime:    run.EventTime,
					RunFacets:    run.RunFacets,
					JobFacets:    run.JobFacets,
				})
			}
		}
		stored, err := w.runsService.ProcessRunHistory(ctx, runHistoryInputs)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to process some run history entries")
		}
		runHistoryStored = stored
		log.Debug().Int("run_history_stored", runHistoryStored).Msg("Processed run history")
	}

	summary := &plugin.RunSummary{
		AssetsCreated:      assetsCreated,
		AssetsUpdated:      assetsUpdated,
		AssetsDeleted:      len(response.StaleEntitiesRemoved),
		LineageCreated:     lineageCreated,
		DocumentationAdded: docsAdded,
		TotalEntities:      len(result.Assets) + len(result.Lineage) + len(result.Documentation),
	}
	_ = w.runsService.CompleteRun(ctx, pluginRun.RunID, plugin.StatusCompleted, summary, "")

	err = w.service.CompleteJobRun(
		ctx,
		run.ID,
		true,
		nil,
		assetsCreated,
		assetsUpdated,
		len(response.StaleEntitiesRemoved),
		lineageCreated,
		docsAdded,
	)

	if err != nil {
		return fmt.Errorf("completing job run: %w", err)
	}

	log.Info().
		Str("run_id", run.ID).
		Int("assets_created", assetsCreated).
		Int("assets_updated", assetsUpdated).
		Msg("Job run completed successfully")

	return nil
}

func convertAssetExternalLinks(links []asset.ExternalLink) []map[string]string {
	result := make([]map[string]string, 0, len(links))
	for _, link := range links {
		result = append(result, map[string]string{
			"name": link.Name,
			"url":  link.URL,
		})
	}
	return result
}
