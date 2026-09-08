package gluepipeline

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSTS struct {
	account string
	err     error
}

func (f fakeSTS) GetCallerIdentity(ctx context.Context, in *sts.GetCallerIdentityInput, opts ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &sts.GetCallerIdentityOutput{Account: aws.String(f.account)}, nil
}

func TestCallerAccount_ReturnsTheAccountID(t *testing.T) {
	assert.Equal(t, "123456789012", callerAccount(context.Background(), fakeSTS{account: "123456789012"}))
}

func TestCallerAccount_IsEmptyWhenTheCallFails(t *testing.T) {
	assert.Empty(t, callerAccount(context.Background(), fakeSTS{err: errUnavailable}))
}

func TestGlueARN_BuildsAWorkflowARN(t *testing.T) {
	assert.Equal(t, "arn:aws:glue:us-east-1:123456789012:workflow/daily-etl",
		glueARN("us-east-1", "123456789012", "workflow", "daily-etl"))
}

func TestGlueARN_BuildsAJobARN(t *testing.T) {
	assert.Equal(t, "arn:aws:glue:eu-west-1:123456789012:job/load-orders",
		glueARN("eu-west-1", "123456789012", "job", "load-orders"))
}

func TestGlueARN_IsEmptyWithoutAnAccount(t *testing.T) {
	assert.Empty(t, glueARN("us-east-1", "", "workflow", "daily-etl"))
}

func TestGlueARN_IsEmptyWithoutARegion(t *testing.T) {
	assert.Empty(t, glueARN("", "123456789012", "workflow", "daily-etl"))
}

func TestWorkflowURL_PointsAtTheConsole(t *testing.T) {
	assert.Equal(t,
		"https://us-east-1.console.aws.amazon.com/glue/home?region=us-east-1#/v2/etl-configuration/workflows/view/daily-etl",
		workflowURL("us-east-1", "daily-etl"))
}

func TestWorkflowURL_IsEmptyWithoutARegion(t *testing.T) {
	assert.Empty(t, workflowURL("", "daily-etl"))
}

func TestBucketFromS3Path_TakesTheBucket(t *testing.T) {
	assert.Equal(t, "marmot-lake", bucketFromS3Path("s3://marmot-lake/raw/orders/"))
}

func TestBucketFromS3Path_HandlesABareBucket(t *testing.T) {
	assert.Equal(t, "marmot-lake", bucketFromS3Path("s3://marmot-lake"))
}

func TestBucketFromS3Path_IgnoresOtherSchemes(t *testing.T) {
	assert.Empty(t, bucketFromS3Path("jdbc:mysql://host/db"))
}

func TestChunk_SplitsAtTheBatchSize(t *testing.T) {
	batches := chunk([]string{"a", "b", "c"}, 2)

	require.Len(t, batches, 2)
	assert.Equal(t, []string{"a", "b"}, batches[0])
	assert.Equal(t, []string{"c"}, batches[1])
}

func TestChunk_IsEmptyForNoNames(t *testing.T) {
	assert.Empty(t, chunk(nil, 25))
}

func TestTruncate_KeepsTheLimit(t *testing.T) {
	assert.Equal(t, []int{1, 2}, truncate([]int{1, 2, 3}, 2))
}

func TestTruncate_LeavesShorterListsAlone(t *testing.T) {
	assert.Equal(t, []int{1}, truncate([]int{1}, 20))
}

func TestFormatParameters_SortsKeys(t *testing.T) {
	assert.Equal(t, "a=1, b=2", formatParameters(map[string]string{"b": "2", "a": "1"}))
}

func TestFormatPredicate_DescribesACrawlerCondition(t *testing.T) {
	predicate := &types.Predicate{
		Logical:    types.LogicalAny,
		Conditions: []types.Condition{{CrawlerName: aws.String("orders-crawler"), CrawlState: types.CrawlStateSucceeded}},
	}

	assert.Equal(t, "ANY: crawler orders-crawler SUCCEEDED", formatPredicate(predicate))
}

func TestFormatPredicate_DescribesAJobCondition(t *testing.T) {
	predicate := &types.Predicate{
		Conditions: []types.Condition{{JobName: aws.String("load-orders"), State: types.JobRunStateSucceeded}},
	}

	assert.Equal(t, "AND: job load-orders SUCCEEDED", formatPredicate(predicate))
}

func TestFormatPredicate_IsEmptyWithoutConditions(t *testing.T) {
	assert.Empty(t, formatPredicate(nil))
}

func TestFormatActions_ListsJobsAndCrawlers(t *testing.T) {
	actions := []types.Action{
		{JobName: aws.String("load-orders")},
		{CrawlerName: aws.String("orders-crawler")},
	}

	assert.Equal(t, "job load-orders, crawler orders-crawler", formatActions(actions))
}

func TestNodeType_IsLowercase(t *testing.T) {
	assert.Equal(t, "job", nodeType(types.NodeTypeJob))
	assert.Equal(t, "crawler", nodeType(types.NodeTypeCrawler))
	assert.Equal(t, "trigger", nodeType(types.NodeTypeTrigger))
}
