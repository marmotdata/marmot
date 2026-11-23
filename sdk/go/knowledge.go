package marmot

import (
	"context"
	"fmt"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/assets"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/docs"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/knowledge"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// KnowledgeKind is a kind of knowledge entry, such as a policy or runbook, with its fields and statuses.
type KnowledgeKind = models.KnowledgeKind

// KnowledgeKinds is every kind and the relations an entry can have to an asset.
type KnowledgeKinds = models.KnowledgeKindsResponse

// KnowledgeEntry is an entry's typed record: kind, status, fields, links and review dates. Its name and summary are on the asset with the same ID.
type KnowledgeEntry = models.KnowledgeEntry

// KnowledgeSummary is one entry in a listing.
type KnowledgeSummary = models.KnowledgeSummary

// KnowledgeList is a page of entries.
type KnowledgeList = models.KnowledgeEntriesResponse

// KnowledgeField is one typed field of a kind.
type KnowledgeField = models.KnowledgeField

// KnowledgeOwner is a team or user that owns an entry, as listings show it.
type KnowledgeOwner = models.KnowledgeOwner

// KnowledgeAbout is a page of the entries that apply to an asset.
type KnowledgeAbout = models.KnowledgeAboutPage

// KnowledgeLinkInput links an entry to an asset: relation is applies_to, documents or related_to.
type KnowledgeLinkInput = models.KnowledgeLinkInput

// KnowledgeRuleInput applies an entry to every asset with a tag, type or provider.
type KnowledgeRuleInput = models.KnowledgeRuleInput

// DocsPage is one documentation page, the body of a knowledge entry.
type DocsPage = models.DocsPage

// KnowledgeProvider is the provider every entry written in Marmot carries.
const KnowledgeProvider = "Marmot"

// KnowledgeListOptions filters KnowledgeService.List.
type KnowledgeListOptions struct {
	Kind      string
	Status    string
	Query     string
	Team      string
	ReviewDue bool
	Mine      bool
	Sort      string
	Limit     int64
	Offset    int64
}

// KnowledgeAboutOptions filters KnowledgeService.About.
type KnowledgeAboutOptions struct {
	Kind     string
	State    string
	Relation string
	Source   string
	Limit    int64
	Cursor   string
}

// KnowledgeSaveInput sets an entry's typed record. Nil Links or Rules leave them as they are; an empty slice clears them.
type KnowledgeSaveInput struct {
	Status        string
	Fields        map[string]any
	EffectiveFrom string
	ReviewBy      string
	Supersedes    string
	Links         []*KnowledgeLinkInput
	Rules         []*KnowledgeRuleInput
}

// EntryOwner is a team or user that owns an entry.
type EntryOwner struct {
	ID   string
	Type string
}

// CreateKnowledgeInput is the input for KnowledgeService.Create. Type is the kind's asset type, such as "Policy"; Writeup is the Markdown body.
type CreateKnowledgeInput struct {
	Type    string
	Title   string
	Summary string
	Owners  []EntryOwner
	Tags    []string
	Writeup string
	Entry   KnowledgeSaveInput
}

// KnowledgeService reads and writes knowledge entries.
type KnowledgeService struct {
	gen *apiclient.Marmot
}

// Kinds returns every kind of entry with its fields and statuses.
func (s *KnowledgeService) Kinds(ctx context.Context) (*KnowledgeKinds, error) {
	resp, err := s.gen.Knowledge.GetKnowledgeKinds(knowledge.NewGetKnowledgeKindsParams().WithContext(ctx), nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// List returns entries matching opts.
func (s *KnowledgeService) List(ctx context.Context, opts KnowledgeListOptions) (*KnowledgeList, error) {
	p := knowledge.NewGetKnowledgeEntriesParams().WithContext(ctx)
	if opts.Kind != "" {
		p = p.WithKind(&opts.Kind)
	}
	if opts.Status != "" {
		p = p.WithStatus(&opts.Status)
	}
	if opts.Query != "" {
		p = p.WithQ(&opts.Query)
	}
	if opts.Team != "" {
		p = p.WithTeam(&opts.Team)
	}
	if opts.ReviewDue {
		p = p.WithReviewDue(&opts.ReviewDue)
	}
	if opts.Mine {
		p = p.WithMine(&opts.Mine)
	}
	if opts.Sort != "" {
		p = p.WithSort(&opts.Sort)
	}
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Knowledge.GetKnowledgeEntries(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Get returns an entry's typed record by its asset ID.
func (s *KnowledgeService) Get(ctx context.Context, assetID string) (*KnowledgeEntry, error) {
	resp, err := s.gen.Knowledge.GetKnowledgeEntry(knowledge.NewGetKnowledgeEntryParams().WithContext(ctx).WithAssetID(assetID), nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Save sets an entry's typed record, creating it for an asset of a knowledge type that has none yet.
func (s *KnowledgeService) Save(ctx context.Context, assetID string, in KnowledgeSaveInput) (*KnowledgeEntry, error) {
	body := &models.KnowledgeSaveInput{
		Status:        in.Status,
		Fields:        in.Fields,
		EffectiveFrom: in.EffectiveFrom,
		ReviewBy:      in.ReviewBy,
		Supersedes:    in.Supersedes,
		Links:         in.Links,
		Rules:         in.Rules,
	}
	p := knowledge.NewPutKnowledgeEntryParams().WithContext(ctx).WithAssetID(assetID).WithEntry(body)
	resp, err := s.gen.Knowledge.PutKnowledgeEntry(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// MarkReviewed records that an owner checked the entry is still true, and sets the next review date (YYYY-MM-DD, or empty for none).
func (s *KnowledgeService) MarkReviewed(ctx context.Context, assetID, nextReview string) (*KnowledgeEntry, error) {
	p := knowledge.NewPostKnowledgeEntryReviewedParams().WithContext(ctx).WithAssetID(assetID).
		WithRequest(&models.KnowledgeReviewedRequest{ReviewBy: nextReview})
	resp, err := s.gen.Knowledge.PostKnowledgeEntryReviewed(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// About returns the entries that apply to an asset: linked to it, matched by a rule, or applying to something that contains it.
func (s *KnowledgeService) About(ctx context.Context, assetID string, opts KnowledgeAboutOptions) (*KnowledgeAbout, error) {
	p := knowledge.NewGetKnowledgeAboutParams().WithContext(ctx).WithAssetID(assetID)
	if opts.Kind != "" {
		p = p.WithKind(&opts.Kind)
	}
	if opts.State != "" {
		p = p.WithState(&opts.State)
	}
	if opts.Relation != "" {
		p = p.WithRelation(&opts.Relation)
	}
	if opts.Source != "" {
		p = p.WithSource(&opts.Source)
	}
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Cursor != "" {
		p = p.WithCursor(&opts.Cursor)
	}
	resp, err := s.gen.Knowledge.GetKnowledgeAbout(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Create writes a new entry in one call: the asset, its typed record and its write-up as the first documentation page. A failure after the asset exists names it, so a retry can finish the job with Save and SetWriteup.
func (s *KnowledgeService) Create(ctx context.Context, in CreateKnowledgeInput) (*Asset, *KnowledgeEntry, error) {
	owners := make([]*models.CreateAssetOwner, len(in.Owners))
	for i, o := range in.Owners {
		owners[i] = &models.CreateAssetOwner{ID: o.ID, Type: o.Type}
	}
	name, typ := in.Title, in.Type
	body := &models.CreateAssetRequest{
		Name:            &name,
		Type:            &typ,
		Providers:       []string{KnowledgeProvider},
		UserDescription: in.Summary,
		Owners:          owners,
		Tags:            in.Tags,
	}
	resp, err := s.gen.Assets.PostAssets(assets.NewPostAssetsParams().WithContext(ctx).WithAsset(body), nil)
	if err != nil {
		return nil, nil, mapErr(err)
	}
	a := resp.Payload
	entry, err := s.Save(ctx, a.ID, in.Entry)
	if err != nil {
		return a, nil, fmt.Errorf("asset %s was created, but saving its entry failed: %w", a.ID, err)
	}
	if in.Writeup != "" {
		if _, err := s.SetWriteup(ctx, a.Mrn, in.Title, in.Writeup); err != nil {
			return a, entry, fmt.Errorf("entry %s was created, but saving its write-up failed: %w", a.ID, err)
		}
	}
	return a, entry, nil
}

// Writeup returns an entry's documentation pages, in order. mrn is the entry's asset MRN.
func (s *KnowledgeService) Writeup(ctx context.Context, mrn string) ([]*DocsPage, error) {
	p := docs.NewGetDocsEntityPagesParams().WithContext(ctx).WithEntityType("asset").WithEntityID(mrn)
	resp, err := s.gen.Docs.GetDocsEntityPages(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	var out []*DocsPage
	var walk func([]*DocsPage)
	walk = func(ps []*DocsPage) {
		for _, pg := range ps {
			out = append(out, pg)
			walk(pg.Children)
		}
	}
	if resp.Payload != nil {
		walk(resp.Payload.Pages)
	}
	return out, nil
}

// SetWriteup replaces the content of an entry's first documentation page, or creates it when there is none.
func (s *KnowledgeService) SetWriteup(ctx context.Context, mrn, title, content string) (*DocsPage, error) {
	pages, err := s.Writeup(ctx, mrn)
	if err != nil {
		return nil, err
	}
	if len(pages) > 0 {
		p := docs.NewPutDocsPageParams().WithContext(ctx).WithPageID(pages[0].ID).
			WithRequest(&models.DocsUpdatePageRequest{Content: content})
		resp, err := s.gen.Docs.PutDocsPage(p, nil)
		if err != nil {
			return nil, mapErr(err)
		}
		return resp.Payload, nil
	}
	p := docs.NewPostDocsEntityPagesParams().WithContext(ctx).WithEntityType("asset").WithEntityID(mrn).
		WithRequest(&models.DocsCreatePageRequest{Title: title, Content: content})
	resp, err := s.gen.Docs.PostDocsEntityPages(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}
