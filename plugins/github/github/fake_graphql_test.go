package github

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// serveGraphQL answers the one query discovery sends, from the same data
// the fake's REST API serves, so both paths can be held to one result.
func (f *fakeGitHub) serveGraphQL(w http.ResponseWriter, body map[string]any) {
	if f.noGraphQL {
		fail(w, http.StatusNotFound, "Not Found")
		return
	}
	if f.graphqlError {
		writeJSON(w, 200, map[string]any{"errors": []map[string]any{{"message": "Something went wrong"}}})
		return
	}
	query, _ := body["query"].(string)
	vars, _ := body["variables"].(map[string]any)
	ids, _ := vars["ids"].([]any)

	nodes := []any{}
	for _, id := range ids {
		name := strings.TrimPrefix(fmt.Sprint(id), "R_")
		r, ok := f.repos[name]
		if !ok {
			nodes = append(nodes, nil)
			continue
		}
		node := map[string]any{"id": id, "name": name}
		tree := f.trees[f.commits[r.branches["main"]].tree]
		blob := func(path string) any {
			file, ok := tree[path]
			if !ok {
				return nil
			}
			text := f.blobs[file.sha]
			return map[string]any{"text": text, "isBinary": strings.ContainsRune(text, 0)}
		}
		if strings.Contains(query, "readme0:") {
			for i, p := range readmeCandidates {
				node[fmt.Sprintf("readme%d", i)] = blob(p)
			}
		}
		if strings.Contains(query, "codeowners0:") {
			for i, p := range codeownersPaths {
				node[fmt.Sprintf("codeowners%d", i)] = blob(p)
			}
		}
		if strings.Contains(query, "refs(") {
			names := make([]string, 0, len(r.branches))
			for b := range r.branches {
				names = append(names, b)
			}
			slices.Sort(names)
			var refs []any
			for _, b := range names[:min(len(names), graphqlFirst)] {
				refs = append(refs, map[string]any{"name": b, "target": map[string]any{"oid": r.branches[b]}})
			}
			node["refs"] = map[string]any{"pageInfo": map[string]any{"hasNextPage": len(names) > graphqlFirst}, "nodes": refs}
		}
		if strings.Contains(query, "pullRequests(") {
			var prs []any
			for n := len(r.pulls); n >= 1; n-- {
				pr := r.pulls[n]
				if pr == nil || pr.state != "open" {
					continue
				}
				prs = append(prs, map[string]any{
					"number": pr.number, "title": pr.title, "body": pr.body, "isDraft": false,
					"url":       fmt.Sprintf("https://github.com/%s/%s/pull/%d", f.owner, name, pr.number),
					"createdAt": pr.created, "updatedAt": pr.updated, "author": map[string]any{"login": pr.user},
					"headRefName": pr.head, "baseRefName": pr.base,
					"labels": map[string]any{"nodes": []any{}}, "assignees": map[string]any{"nodes": []any{}},
					"reviewRequests": map[string]any{"nodes": []any{}},
				})
			}
			more := len(prs) > graphqlFirst
			node["pullRequests"] = map[string]any{"pageInfo": map[string]any{"hasNextPage": more}, "nodes": prs[:min(len(prs), graphqlFirst)]}
		}
		if strings.Contains(query, "issues(") {
			var issues []any
			for _, is := range r.issues {
				if is["pull_request"] != nil {
					continue // the GraphQL issues list holds no pull requests
				}
				issues = append(issues, map[string]any{
					"number": is["number"], "title": is["title"], "body": is["body"], "url": is["html_url"],
					"createdAt": is["created_at"], "updatedAt": is["updated_at"], "author": is["user"],
					"labels": map[string]any{"nodes": orEmpty(is["labels"])}, "assignees": map[string]any{"nodes": orEmpty(is["assignees"])},
					"comments": map[string]any{"totalCount": is["comments"]},
				})
			}
			// A repository with issues turned off has an empty list.
			node["issues"] = map[string]any{"pageInfo": map[string]any{"hasNextPage": len(issues) > graphqlFirst}, "nodes": issues[:min(len(issues), graphqlFirst)]}
		}
		nodes = append(nodes, node)
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"nodes": nodes}})
}

func orEmpty(v any) any {
	if v == nil {
		return []any{}
	}
	return v
}
