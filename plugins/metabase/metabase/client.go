package metabase

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// client talks to the Metabase REST API. Every path is relative to
// {host}/api. Metabase authenticates machine clients with an API key
// (0.49 and newer) sent as X-API-KEY, or with a session id obtained by
// logging in with a username and password.
type client struct {
	baseURL string
	apiKey  string
	session string
	http    *http.Client
}

func newClient(host, apiKey string, timeout time.Duration, verifySSL bool) *client {
	transport := http.DefaultTransport
	if !verifySSL {
		// Clone rather than replace, so proxy settings and connection
		// pooling behave the same with verification turned off.
		insecure := http.DefaultTransport.(*http.Transport).Clone()
		insecure.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in via verify_ssl: false
		transport = insecure
	}

	return &client{
		baseURL: strings.TrimSuffix(host, "/") + "/api",
		apiKey:  apiKey,
		http: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

// login exchanges a username and password for a session id, used on
// every later request when no API key is configured.
func (c *client) login(ctx context.Context, username, password string) error {
	var resp struct {
		ID string `json:"id"`
	}
	body := map[string]string{"username": username, "password": password}
	if err := c.do(ctx, http.MethodPost, "/session", nil, body, &resp); err != nil {
		return err
	}
	if resp.ID == "" {
		return fmt.Errorf("logging in: response carried no session id")
	}
	c.session = resp.ID
	return nil
}

// do performs one request against a path below the API root and
// decodes the JSON response into out. A nil out discards the body.
func (c *client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request for %s: %w", path, err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("building request for %s: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-KEY", c.apiKey)
	} else if c.session != "" {
		req.Header.Set("X-Metabase-Session", c.session)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &apiError{Path: path, Status: resp.StatusCode, Body: strings.TrimSpace(string(snippet))}
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding %s: %w", path, err)
	}
	return nil
}

// apiError is a non-2xx response from Metabase.
type apiError struct {
	Path   string
	Status int
	Body   string
}

func (e *apiError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s: %s", e.Path, http.StatusText(e.Status))
	}
	return fmt.Sprintf("%s: %s: %s", e.Path, http.StatusText(e.Status), e.Body)
}

// listOf decodes a Metabase list response. Some list endpoints answer
// with a bare array and others (for example /database) wrap it in
// {"data": [...]}, and the same endpoint has changed shape between
// releases, so both are accepted everywhere.
type listOf[T any] []T

func (l *listOf[T]) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var items []T
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return err
		}
		*l = items
		return nil
	}

	var envelope struct {
		Data []T `json:"data"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return err
	}
	*l = envelope.Data
	return nil
}

// collectionID is a collection's id, which Metabase gives as an integer
// for real collections and as the string "root" for the root.
type collectionID string

const rootCollectionID collectionID = "root"

func (id *collectionID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*id = rootCollectionID
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return err
		}
		*id = collectionID(s)
		return nil
	}
	var n int
	if err := json.Unmarshal(trimmed, &n); err != nil {
		return fmt.Errorf("collection id %s: %w", string(trimmed), err)
	}
	*id = collectionID(strconv.Itoa(n))
	return nil
}

// collection is one entry of /collection or one node of /collection/tree.
type collection struct {
	ID              collectionID `json:"id"`
	Name            string       `json:"name"`
	Location        string       `json:"location"`
	Archived        bool         `json:"archived"`
	PersonalOwnerID *int         `json:"personal_owner_id"`
	Children        []collection `json:"children"`
}

// listCollections returns every collection the credentials can see.
// The flat /collection endpoint is preferred because it includes the
// root; when it fails the nested /collection/tree is flattened
// instead. Metabase 0.63 answers /collection with a 500 for API key
// principals (it tries to hydrate a personal collection the key does
// not have), while the tree endpoint works.
func (c *client) listCollections(ctx context.Context) ([]collection, error) {
	var flat listOf[collection]
	err := c.do(ctx, http.MethodGet, "/collection", nil, nil, &flat)
	if err == nil {
		return flat, nil
	}
	log.Warn().Err(err).Msg("Listing collections failed, falling back to the collection tree")

	var tree listOf[collection]
	if treeErr := c.do(ctx, http.MethodGet, "/collection/tree", nil, nil, &tree); treeErr != nil {
		return nil, fmt.Errorf("listing collections: %w (tree fallback: %w)", err, treeErr)
	}

	var all []collection
	var flatten func(nodes []collection)
	flatten = func(nodes []collection) {
		for _, node := range nodes {
			children := node.Children
			node.Children = nil
			all = append(all, node)
			flatten(children)
		}
	}
	flatten(tree)
	return all, nil
}

// database is one entry of /database.
type database struct {
	ID       int            `json:"id"`
	Name     string         `json:"name"`
	Engine   string         `json:"engine"`
	Details  map[string]any `json:"details"`
	IsSample bool           `json:"is_sample"`
}

func (c *client) listDatabases(ctx context.Context) ([]database, error) {
	var dbs listOf[database]
	if err := c.do(ctx, http.MethodGet, "/database", nil, nil, &dbs); err != nil {
		return nil, err
	}
	return dbs, nil
}

// table is one entry of the tables array in /database/{id}/metadata.
type table struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Schema      string `json:"schema"`
	DisplayName string `json:"display_name"`
}

// listTables returns every table Metabase has synced for one database.
// One call per database replaces a /table/{id} call per card.
func (c *client) listTables(ctx context.Context, databaseID int) ([]table, error) {
	var resp struct {
		Tables []table `json:"tables"`
	}
	if err := c.do(ctx, http.MethodGet, "/database/"+strconv.Itoa(databaseID)+"/metadata", nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Tables, nil
}

// card is one entry of /card. A card is a question, a metric or a
// model; all three share the shape.
type card struct {
	ID           int             `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Display      string          `json:"display"`
	Type         string          `json:"type"`
	QueryType    string          `json:"query_type"`
	CollectionID collectionID    `json:"collection_id"`
	DatabaseID   int             `json:"database_id"`
	TableID      int             `json:"table_id"`
	SourceCardID int             `json:"source_card_id"`
	CreatorID    int             `json:"creator_id"`
	Archived     bool            `json:"archived"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
	DatasetQuery json.RawMessage `json:"dataset_query"`
}

// listCards returns every card, or with archived every archived card.
// Metabase filters archived cards out unless asked for them alone.
func (c *client) listCards(ctx context.Context, archived bool) ([]card, error) {
	query := url.Values{}
	if archived {
		query.Set("f", "archived")
	}
	var cards listOf[card]
	if err := c.do(ctx, http.MethodGet, "/card", query, nil, &cards); err != nil {
		return nil, err
	}
	return cards, nil
}

// dashboard is one entry of /dashboard, or the body of /dashboard/{id}
// which adds the cards placed on it.
type dashboard struct {
	ID           int          `json:"id"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	CollectionID collectionID `json:"collection_id"`
	CreatorID    int          `json:"creator_id"`
	Archived     bool         `json:"archived"`
	CreatedAt    string       `json:"created_at"`
	UpdatedAt    string       `json:"updated_at"`

	// Dashcards is the placement list on 0.48 and newer; older servers
	// call the same list ordered_cards.
	Dashcards    []dashcard `json:"dashcards"`
	OrderedCards []dashcard `json:"ordered_cards"`
}

// dashcard is one placement on a dashboard. Text and heading cards
// have no card_id.
type dashcard struct {
	CardID int `json:"card_id"`
}

// cardIDs returns the ids of the cards placed on the dashboard,
// whichever name the server used for the list.
func (d dashboard) cardIDs() []int {
	placements := d.Dashcards
	if len(placements) == 0 {
		placements = d.OrderedCards
	}
	ids := make([]int, 0, len(placements))
	for _, p := range placements {
		if p.CardID != 0 {
			ids = append(ids, p.CardID)
		}
	}
	return ids
}

// listDashboards returns every dashboard, or with archived every
// archived dashboard, without their placements.
func (c *client) listDashboards(ctx context.Context, archived bool) ([]dashboard, error) {
	query := url.Values{}
	if archived {
		query.Set("f", "archived")
	}
	var dashboards listOf[dashboard]
	if err := c.do(ctx, http.MethodGet, "/dashboard", query, nil, &dashboards); err != nil {
		return nil, err
	}
	return dashboards, nil
}

// getDashboard returns one dashboard with the cards placed on it.
func (c *client) getDashboard(ctx context.Context, id int) (*dashboard, error) {
	var d dashboard
	if err := c.do(ctx, http.MethodGet, "/dashboard/"+strconv.Itoa(id), nil, nil, &d); err != nil {
		return nil, err
	}
	return &d, nil
}
