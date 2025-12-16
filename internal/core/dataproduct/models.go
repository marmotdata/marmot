package dataproduct

import (
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("data product not found")
	ErrConflict     = errors.New("data product with this name already exists")
	ErrInvalidInput = errors.New("invalid input")
	ErrRuleNotFound = errors.New("rule not found")
)

type DataProduct struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   *string                `json:"description,omitempty"`
	Documentation *string                `json:"documentation,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Owners        []Owner                `json:"owners"`
	Rules         []Rule                 `json:"rules,omitempty"`
	CreatedBy     *string                `json:"created_by,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`

	AssetCount       int `json:"asset_count,omitempty"`
	ManualAssetCount int `json:"manual_asset_count,omitempty"`
	RuleAssetCount   int `json:"rule_asset_count,omitempty"`
}

type Owner struct {
	ID             string  `json:"id"`
	Username       *string `json:"username,omitempty"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	Email          *string `json:"email,omitempty"`
	ProfilePicture *string `json:"profile_picture,omitempty"`
}

type OwnerInput struct {
	ID   string `json:"id" validate:"required"`
	Type string `json:"type" validate:"required,oneof=user team"`
}

type RuleType string

const (
	RuleTypeQuery         RuleType = "query"
	RuleTypeMetadataMatch RuleType = "metadata_match"
)

const (
	PatternTypeExact    = "exact"
	PatternTypeWildcard = "wildcard"
	PatternTypeRegex    = "regex"
	PatternTypePrefix   = "prefix"
)

// Membership source types
const (
	SourceManual = "manual"
	SourceRule   = "rule"
)

// Rule target types for candidate lookup
const (
	TargetTypeAssetType   = "asset_type"
	TargetTypeProvider    = "provider"
	TargetTypeTag         = "tag"
	TargetTypeMetadataKey = "metadata_key"
	TargetTypeQuery       = "query"
)

type Rule struct {
	ID              string    `json:"id"`
	DataProductID   string    `json:"data_product_id"`
	Name            string    `json:"name"`
	Description     *string   `json:"description,omitempty"`
	RuleType        RuleType  `json:"rule_type"`
	QueryExpression *string   `json:"query_expression,omitempty"`
	MetadataField   *string   `json:"metadata_field,omitempty"`
	PatternType     *string   `json:"pattern_type,omitempty"`
	PatternValue    *string   `json:"pattern_value,omitempty"`
	Priority        int       `json:"priority"`
	IsEnabled       bool      `json:"is_enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	MatchedAssetCount int `json:"matched_asset_count,omitempty"`
}

type CreateInput struct {
	Name          string                 `json:"name" validate:"required,min=1,max=255"`
	Description   *string                `json:"description,omitempty"`
	Documentation *string                `json:"documentation,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Owners        []OwnerInput           `json:"owners" validate:"required,min=1,dive"`
	Rules         []RuleInput            `json:"rules,omitempty" validate:"omitempty,dive"`
}

type UpdateInput struct {
	Name          *string                `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description   *string                `json:"description,omitempty"`
	Documentation *string                `json:"documentation,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Owners        []OwnerInput           `json:"owners,omitempty" validate:"omitempty,min=1,dive"`
}

type RuleInput struct {
	ID              *string  `json:"id,omitempty"`
	Name            string   `json:"name" validate:"required,min=1,max=255"`
	Description     *string  `json:"description,omitempty"`
	RuleType        RuleType `json:"rule_type" validate:"required,oneof=query metadata_match"`
	QueryExpression *string  `json:"query_expression,omitempty"`
	MetadataField   *string  `json:"metadata_field,omitempty"`
	PatternType     *string  `json:"pattern_type,omitempty" validate:"omitempty,oneof=exact wildcard regex prefix"`
	PatternValue    *string  `json:"pattern_value,omitempty"`
	Priority        int      `json:"priority"`
	IsEnabled       bool     `json:"is_enabled"`
}

type SearchFilter struct {
	Query    string   `json:"query,omitempty"`
	OwnerIDs []string `json:"owner_ids,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Limit    int      `json:"limit,omitempty" validate:"omitempty,gte=0,lte=100"`
	Offset   int      `json:"offset,omitempty" validate:"omitempty,gte=0"`
}

type ListResult struct {
	DataProducts []*DataProduct `json:"data_products"`
	Total        int            `json:"total"`
}

type ResolvedAssets struct {
	ManualAssets  []string `json:"manual_assets"`
	DynamicAssets []string `json:"dynamic_assets"`
	AllAssets     []string `json:"all_assets"`
	Total         int      `json:"total"`
}

type RulePreview struct {
	AssetIDs   []string `json:"asset_ids"`
	AssetCount int      `json:"asset_count"`
	Errors     []string `json:"errors,omitempty"`
}

type AssetsResult struct {
	AssetIDs []string `json:"asset_ids"`
	Total    int      `json:"total"`
}
