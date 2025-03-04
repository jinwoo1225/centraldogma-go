// Copyright 2018 LINE Corporation
//
// LINE Corporation licenses this file to you under the Apache License,
// version 2.0 (the "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at:
//
//   https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package centraldogma

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type contentService service

// Query specifies a query on a file.
type Query struct {
	Path string
	// QueryType can be "identity" or "json_path". "identity" is used to retrieve the content as it is.
	// "json_path" applies a series of JSON path to the content.
	// See https://github.com/json-path/JsonPath/blob/master/README.md
	Type        QueryType
	Expressions []string
}

type QueryType int

const (
	Identity QueryType = iota + 1
	JSONPath
)

// Entry represents an entry in the repository.
type Entry struct {
	Path       string       `json:"path"`
	Type       EntryType    `json:"type"` // can be JSON, TEXT or DIRECTORY
	Content    EntryContent `json:"content,omitempty"`
	Revision   int64        `json:"revision,omitempty"`
	URL        string       `json:"url,omitempty"`
	ModifiedAt string       `json:"modifiedAt,omitempty"`
}

func (c *Entry) MarshalJSON() ([]byte, error) {
	type Alias Entry
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  c.Type.String(),
		Alias: (*Alias)(c),
	})
}

func (c *Entry) UnmarshalJSON(b []byte) error {
	type Alias Entry
	auxiliary := &struct {
		Type string `json:"type"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(b, &auxiliary); err != nil {
		return err
	}
	c.Type = entryTypeMap[auxiliary.Type]
	return nil
}

// EntryContent represents the content of an entry
type EntryContent []byte

func (e *EntryContent) UnmarshalJSON(b []byte) error {
	if n := len(b); n >= 2 && b[0] == 34 && b[n-1] == 34 { // string
		var dst string
		if err := json.Unmarshal(b, &dst); err != nil {
			return err
		}
		*e = []byte(dst)
	} else {
		*e = b
	}
	return nil
}

// PushResult represents a result of push in the repository.
type PushResult struct {
	Revision int64  `json:"revision"`
	PushedAt string `json:"pushedAt"`
}

// Commit represents a commit in the repository.
type Commit struct {
	Revision      int64         `json:"revision"`
	Author        Author        `json:"author,omitempty"`
	CommitMessage CommitMessage `json:"commitMessage,omitempty"`
	PushedAt      string        `json:"pushedAt,omitempty"`
}

// CommitMessages represents a commit message in the repository.
type CommitMessage struct {
	Summary string `json:"summary"`
	Detail  string `json:"detail,omitempty"`
	Markup  string `json:"markup,omitempty"`
}

// Change represents a change to commit in the repository.
type Change struct {
	Path    string      `json:"path"`
	Type    ChangeType  `json:"type"`
	Content interface{} `json:"content,omitempty"`
}

func (c *Change) MarshalJSON() ([]byte, error) {
	type Alias Change
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  c.Type.String(),
		Alias: (*Alias)(c),
	})
}

func (c *Change) UnmarshalJSON(b []byte) error {
	type Alias Change
	auxiliary := &struct {
		Type string `json:"type"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(b, &auxiliary); err != nil {
		return err
	}
	c.Type = changeTypeMap[auxiliary.Type]
	return nil
}

func (con *contentService) listFiles(ctx context.Context,
	projectName, repoName, revision, pathPattern string) ([]*Entry, int, error) {
	if len(pathPattern) != 0 && !strings.HasPrefix(pathPattern, "/") {
		// Normalize the pathPattern when it does not start with "/" so that the pathPattern fits into the url.
		pathPattern = "/**/" + pathPattern
	}

	// build relative url
	u, err := url.Parse(path.Join(
		defaultPathPrefix,
		projects, projectName,
		repos, repoName,
		actionList, pathPattern,
	))
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	// build query params
	q := u.Query()
	setRevision(&q, revision)
	u.RawQuery = q.Encode()

	req, err := con.client.newRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	var entries []*Entry
	httpStatusCode, err := con.client.do(ctx, req, &entries, false)
	if err != nil {
		return nil, httpStatusCode, err
	}
	return entries, httpStatusCode, nil
}

func (con *contentService) getFile(
	ctx context.Context, projectName, repoName, revision string, query *Query) (*Entry, int, error) {
	if query == nil {
		return nil, UnknownHttpStatusCode, errors.New("query should not be nil")
	}

	// build relative url
	u, err := url.Parse(path.Join(
		defaultPathPrefix,
		projects, projectName,
		repos, repoName,
		contents, query.Path,
	))
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	// build query params
	q := u.Query()
	if err := getFileURLValues(&q, revision, query); err != nil {
		return nil, UnknownHttpStatusCode, err
	}
	u.RawQuery = q.Encode()

	req, err := con.client.newRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	entry := new(Entry)
	httpStatusCode, err := con.client.do(ctx, req, entry, false)
	if err != nil {
		return nil, httpStatusCode, err
	}

	return entry, httpStatusCode, nil
}

func (con *contentService) getFiles(ctx context.Context,
	projectName, repoName, revision, pathPattern string) ([]*Entry, int, error) {
	if len(pathPattern) != 0 && !strings.HasPrefix(pathPattern, "/") {
		// Normalize the pathPattern when it does not start with "/" so that the pathPattern fits into the url.
		pathPattern = "/**/" + pathPattern
	}

	// build relative url
	u, err := url.Parse(path.Join(
		defaultPathPrefix,
		projects, projectName,
		repos, repoName,
		contents, pathPattern,
	))
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	// build query params
	q := u.Query()
	setRevision(&q, revision)
	u.RawQuery = q.Encode()

	req, err := con.client.newRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	var entries []*Entry
	httpStatusCode, err := con.client.do(ctx, req, &entries, false)
	if err != nil {
		return nil, httpStatusCode, err
	}
	return entries, httpStatusCode, nil
}

func (con *contentService) getHistory(ctx context.Context,
	projectName, repoName, from, to, pathPattern string, maxCommits int) ([]*Commit, int, error) {

	// build relative url
	u, err := url.Parse(path.Join(
		defaultPathPrefix,
		projects, projectName,
		repos, repoName,
		commits, from,
	))
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	// build query params
	q := u.Query()
	setPath(&q, pathPattern)
	setFromTo(&q, "", to)
	setMaxCommits(&q, maxCommits)
	u.RawQuery = q.Encode()

	req, err := con.client.newRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	var commits []*Commit
	httpStatusCode, err := con.client.do(ctx, req, &commits, false)
	if err != nil {
		return nil, httpStatusCode, err
	}
	return commits, httpStatusCode, nil
}

func (con *contentService) getDiff(ctx context.Context,
	projectName, repoName, from, to string, query *Query) (*Change, int, error) {

	// validate query
	if query == nil {
		return nil, UnknownHttpStatusCode, errors.New("query should not be nil")
	}
	if len(query.Path) == 0 {
		return nil, UnknownHttpStatusCode, errors.New("the path should not be empty")
	}

	// build relative url
	u, err := url.Parse(path.Join(
		defaultPathPrefix,
		projects, projectName,
		repos, repoName,
		actionCompare,
	))
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	// build query params
	q := u.Query()
	setPath(&q, path.Join("/", query.Path))
	if err := setJSONPaths(&q, query); err != nil {
		return nil, UnknownHttpStatusCode, err
	}
	setFromTo(&q, from, to)
	u.RawQuery = q.Encode()

	req, err := con.client.newRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	change := new(Change)
	httpStatusCode, err := con.client.do(ctx, req, change, false)
	if err != nil {
		return nil, httpStatusCode, err
	}

	return change, httpStatusCode, nil
}

func (con *contentService) getDiffs(ctx context.Context,
	projectName, repoName, from, to, pathPattern string) ([]*Change, int, error) {

	// validate path pattern
	if len(pathPattern) == 0 {
		pathPattern = "/**"
	}

	// build relative url
	u, err := url.Parse(path.Join(
		defaultPathPrefix,
		projects, projectName,
		repos, repoName,
		actionCompare,
	))
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	// build query params
	q := u.Query()
	setPathPattern(&q, pathPattern)
	setFromTo(&q, from, to)
	u.RawQuery = q.Encode()

	req, err := con.client.newRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	var changes []*Change
	httpStatusCode, err := con.client.do(ctx, req, &changes, false)
	if err != nil {
		return nil, httpStatusCode, err
	}
	return changes, httpStatusCode, nil
}

type push struct {
	CommitMessage *CommitMessage `json:"commitMessage"`
	Changes       []*Change      `json:"changes"`
}

func (con *contentService) push(ctx context.Context, projectName, repoName, baseRevision string,
	commitMessage *CommitMessage, changes []*Change) (*PushResult, int, error) {
	if len(commitMessage.Summary) == 0 {
		return nil, UnknownHttpStatusCode, fmt.Errorf(
			"summary of commitMessage cannot be empty. commitMessage: %+v", commitMessage)
	}

	if len(changes) == 0 {
		return nil, UnknownHttpStatusCode, errors.New("no changes to commit")
	}

	// build relative url
	u, err := url.Parse(path.Join(
		defaultPathPrefix,
		projects, projectName,
		repos, repoName,
		contents,
	))
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	// build query params
	q := u.Query()
	setRevision(&q, baseRevision)
	u.RawQuery = q.Encode()

	body := push{CommitMessage: commitMessage, Changes: changes}

	req, err := con.client.newRequest(http.MethodPost, u, body)
	if err != nil {
		return nil, UnknownHttpStatusCode, err
	}

	pushResult := new(PushResult)
	httpStatusCode, err := con.client.do(ctx, req, pushResult, false)
	if err != nil {
		return nil, httpStatusCode, err
	}
	return pushResult, httpStatusCode, nil
}
