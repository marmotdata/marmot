package bigtable

import (
	"context"
	"errors"
	"fmt"
	"sort"

	// bt is Google's Bigtable client. It is aliased because this package is
	// also called bigtable.
	bt "cloud.google.com/go/bigtable"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// instanceDetail is everything discovery knows about one instance. Only
// ID is guaranteed: against an emulator there is no instance admin API to
// ask, so the rest stays empty.
type instanceDetail struct {
	ID          string
	DisplayName string
	State       string
	Type        string
	Labels      map[string]string
	Clusters    []clusterDetail
}

// clusterDetail is one cluster of an instance, as it appears in the
// instance asset's metadata.
type clusterDetail struct {
	Name        string `json:"name"`
	Zone        string `json:"zone"`
	Nodes       int    `json:"nodes"`
	StorageType string `json:"storage_type"`
}

// rowReader reads rows from one table. The Bigtable client's Table
// satisfies it; tests supply rows without a server.
type rowReader interface {
	ReadRows(ctx context.Context, arg bt.RowSet, f func(bt.Row) bool, opts ...bt.ReadOption) error
}

// clientOptions builds the options every Bigtable client in this plugin
// is created with. An emulator speaks plain gRPC and knows nothing about
// Google accounts, so pointing at one means switching off both TLS and
// authentication. All of it comes from the config rather than the
// BIGTABLE_EMULATOR_HOST environment variable, so one pipeline cannot
// change how another one connects.
func (s *Source) clientOptions() []option.ClientOption {
	if s.config.EmulatorHost != "" {
		return []option.ClientOption{
			option.WithEndpoint(s.config.EmulatorHost),
			option.WithoutAuthentication(),
			option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		}
	}

	switch {
	case s.config.Credentials.CredentialsJSON != "":
		return []option.ClientOption{option.WithCredentialsJSON([]byte(s.config.Credentials.CredentialsJSON))}
	case s.config.Credentials.CredentialsFile != "":
		return []option.ClientOption{option.WithCredentialsFile(s.config.Credentials.CredentialsFile)}
	}

	// Neither key given: fall back to the credentials the environment
	// already provides (Workload Identity, a service account attached to
	// the machine, or GOOGLE_APPLICATION_CREDENTIALS).
	return nil
}

// listInstances resolves the instances to discover, filling in as much
// detail as the project allows.
func (s *Source) listInstances(ctx context.Context) ([]instanceDetail, error) {
	// The emulator implements neither ListInstances nor ListClusters, so
	// the configured ids are all there is to go on.
	if s.config.EmulatorHost != "" {
		details := make([]instanceDetail, 0, len(s.config.Instances))
		for _, id := range s.config.Instances {
			details = append(details, instanceDetail{ID: id})
		}
		return details, nil
	}

	client, err := bt.NewInstanceAdminClient(ctx, s.config.ProjectID, s.clientOptions()...)
	if err != nil {
		return nil, fmt.Errorf("creating instance admin client: %w", err)
	}
	defer client.Close()

	details, err := s.instanceDetails(ctx, client)
	if err != nil {
		return nil, err
	}

	for i := range details {
		clusters, err := clusterDetails(ctx, client, details[i].ID)
		if err != nil {
			log.Warn().Err(err).Str("instance", details[i].ID).Msg("Failed to list clusters")
			continue
		}
		details[i].Clusters = clusters
	}

	return details, nil
}

// instanceDetails returns the configured instances, or every instance in
// the project when none were configured.
func (s *Source) instanceDetails(ctx context.Context, client *bt.InstanceAdminClient) ([]instanceDetail, error) {
	if len(s.config.Instances) > 0 {
		details := make([]instanceDetail, 0, len(s.config.Instances))
		for _, id := range s.config.Instances {
			info, err := client.InstanceInfo(ctx, id)
			if err != nil {
				// The id is configured, so the instance is still worth an
				// asset even when its details are out of reach.
				log.Warn().Err(err).Str("instance", id).Msg("Failed to read instance details")
				details = append(details, instanceDetail{ID: id})
				continue
			}
			details = append(details, newInstanceDetail(info))
		}
		return details, nil
	}

	infos, err := client.Instances(ctx)
	if err != nil {
		// A project spread over several regions reports the regions it
		// could not reach and still returns the rest.
		var partial bt.ErrPartiallyUnavailable
		if !errors.As(err, &partial) || len(infos) == 0 {
			return nil, fmt.Errorf("listing instances: %w", err)
		}
		log.Warn().Err(err).Msg("Some locations were unavailable, discovering the instances that were returned")
	}

	details := make([]instanceDetail, 0, len(infos))
	for _, info := range infos {
		details = append(details, newInstanceDetail(info))
	}
	return details, nil
}

func newInstanceDetail(info *bt.InstanceInfo) instanceDetail {
	return instanceDetail{
		ID:          info.Name,
		DisplayName: info.DisplayName,
		State:       instanceStateText(info.InstanceState),
		Type:        instanceTypeText(info.InstanceType),
		Labels:      info.Labels,
	}
}

func clusterDetails(ctx context.Context, client *bt.InstanceAdminClient, instanceID string) ([]clusterDetail, error) {
	infos, err := client.Clusters(ctx, instanceID)
	if err != nil {
		var partial bt.ErrPartiallyUnavailable
		if !errors.As(err, &partial) || len(infos) == 0 {
			return nil, fmt.Errorf("listing clusters: %w", err)
		}
		log.Warn().Err(err).Str("instance", instanceID).Msg("Some locations were unavailable, using the clusters that were returned")
	}

	clusters := make([]clusterDetail, 0, len(infos))
	for _, info := range infos {
		clusters = append(clusters, clusterDetail{
			Name:        info.Name,
			Zone:        info.Zone,
			Nodes:       info.ServeNodes,
			StorageType: storageTypeText(info.StorageType),
		})
	}
	return clusters, nil
}

// backupCount totals the backups held by an instance's clusters. Backups
// live on a cluster rather than on a table, so an instance with no
// clusters (an emulator) reports none.
func backupCount(ctx context.Context, client *bt.AdminClient, clusters []clusterDetail) int {
	total := 0
	for _, cluster := range clusters {
		it := client.Backups(ctx, cluster.Name)
		for {
			_, err := it.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				log.Warn().Err(err).Str("cluster", cluster.Name).Msg("Failed to list backups")
				break
			}
			total++
		}
	}
	return total
}

// sampleColumns reads up to limit rows and reports the family:qualifier
// pairs they hold. Bigtable declares column families but not the
// qualifiers under them, so the only way to learn a table's columns is to
// look at rows. It returns the columns and the number of rows read.
func sampleColumns(ctx context.Context, reader rowReader, limit int) ([]column, int, error) {
	type seenColumn struct {
		family       string
		qualifier    string
		occurrence   int
		inferredType string
	}

	found := make(map[string]*seenColumn)
	sampled := 0

	err := reader.ReadRows(ctx, bt.InfiniteRange(""), func(row bt.Row) bool {
		sampled++
		// A row can hold several cells for the same column even under a
		// latest-version filter, so occurrence counts rows, not cells.
		inRow := make(map[string]bool)

		for _, items := range row {
			for _, item := range items {
				family, qualifier := splitColumn(item.Column)
				entry, ok := found[item.Column]
				if !ok {
					entry = &seenColumn{family: family, qualifier: qualifier}
					found[item.Column] = entry
				}
				if !inRow[item.Column] {
					inRow[item.Column] = true
					entry.occurrence++
				}
				entry.inferredType = mergeType(entry.inferredType, inferValueType(item.Value))
			}
		}
		return true
	}, bt.LimitRows(int64(limit)), bt.RowFilter(bt.LatestNFilter(1)))
	if err != nil {
		return nil, 0, fmt.Errorf("reading rows: %w", err)
	}

	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)

	columns := []column{rowKeyColumn()}
	for _, name := range names {
		entry := found[name]
		columns = append(columns, column{
			Column: pluginsdk.Column{
				Name:     name,
				DataType: "bytes",
				// Bigtable rows are sparse: a column found in one row says
				// nothing about whether the next row has it.
				Nullable: true,
			},
			ColumnFamily: entry.family,
			Qualifier:    entry.qualifier,
			Occurrence:   entry.occurrence,
			InferredType: entry.inferredType,
		})
	}

	return columns, sampled, nil
}

// countRows counts a table's rows, stopping at max. It returns the count
// and whether the whole table was counted: Bigtable keeps no row count,
// so the only way to get one is to scan, and a scan that hits the limit
// has counted a prefix rather than the table.
func countRows(ctx context.Context, reader rowReader, max int) (int64, bool, error) {
	var count int64

	// The values are not needed, only the keys, so strip them and keep one
	// cell per row to save bandwidth. Reading one row past the limit is
	// what tells a table of exactly max rows apart from a longer one.
	err := reader.ReadRows(ctx, bt.InfiniteRange(""), func(bt.Row) bool {
		count++
		return true
	},
		bt.LimitRows(int64(max)+1),
		bt.RowFilter(bt.ChainFilters(bt.CellsPerRowLimitFilter(1), bt.StripValueFilter())),
	)
	if err != nil {
		return 0, false, fmt.Errorf("counting rows: %w", err)
	}

	if count > int64(max) {
		return int64(max), false, nil
	}
	return count, true, nil
}
