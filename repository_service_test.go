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
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestCreateRepository(t *testing.T) {
	c, mux, teardown := setup()
	defer teardown()

	input := &Repository{Name: "bar"}

	mux.HandleFunc("/api/v1/projects/foo/repos", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, http.MethodPost)
		testAuthorization(t, r)

		repo := new(Repository)
		_ = json.NewDecoder(r.Body).Decode(repo)
		if !reflect.DeepEqual(repo, input) {
			t.Errorf("Request body = %+v, want %+v", repo, input)
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Location", "/api/v1/projects/foo/repos/bar")
		fmt.Fprint(w, `{"name":"bar", "creator":{"name":"minux", "email":"minux@m.x"}, "headRevision": 2}`)
	})

	repo, httpStatusCode, _ := c.CreateRepository(context.Background(), "foo", "bar")
	testStatusCode(t, httpStatusCode, 201)

	want := &Repository{Name: "bar", Creator: Author{Name: "minux", Email: "minux@m.x"}, HeadRevision: 2}
	if !reflect.DeepEqual(repo, want) {
		t.Errorf("CreateRepository returned %+v, want %+v", repo, want)
	}
}

func TestRemoveRepository(t *testing.T) {
	c, mux, teardown := setup()
	defer teardown()

	mux.HandleFunc("/api/v1/projects/foo/repos/bar", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, http.MethodDelete)
		w.WriteHeader(http.StatusNoContent)
	})

	httpStatusCode, _ := c.RemoveRepository(context.Background(), "foo", "bar")
	testStatusCode(t, httpStatusCode, 204)
}

func TestPurgeRepository(t *testing.T) {
	c, mux, teardown := setup()
	defer teardown()

	mux.HandleFunc("/api/v1/projects/foo/repos/bar/removed", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, http.MethodDelete)
		w.WriteHeader(http.StatusNoContent)
	})

	httpStatusCode, _ := c.PurgeRepository(context.Background(), "foo", "bar")
	testStatusCode(t, httpStatusCode, 204)
}

func TestUnremoveRepository(t *testing.T) {
	c, mux, teardown := setup()
	defer teardown()

	mux.HandleFunc("/api/v1/projects/foo/repos/bar", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, http.MethodPatch)
		testHeader(t, r, "Content-Type", "application/json-patch+json")
		testBody(t, r, `[{"op":"replace", "path":"/status", "value":"active"}]`)
		fmt.Fprint(w,
			`{"name":"bar",
"creator":{"name":"minux", "email":"minux@m.x"},
"url":"/api/v1/projects/foo/repos/bar",
"headRevision":2}`)
	})

	repo, _, _ := c.UnremoveRepository(context.Background(), "foo", "bar")
	want :=
		&Repository{Name: "bar", Creator: Author{Name: "minux", Email: "minux@m.x"},
			URL: "/api/v1/projects/foo/repos/bar", HeadRevision: 2}
	if !reflect.DeepEqual(repo, want) {
		t.Errorf("UnremoveRepository returned %+v, want %+v", repo, want)
	}
}

func TestListRepositories(t *testing.T) {
	c, mux, teardown := setup()
	defer teardown()

	mux.HandleFunc("/api/v1/projects/foo/repos", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, http.MethodGet)
		fmt.Fprint(w,
			`[{
"name":"bar",
"creator":{"name":"minux", "email":"minux@m.x"},
"url":"/api/v1/projects/foo/repos/bar",
"headRevision":2},{
"name":"baz",
"creator":{"name":"minux", "email":"minux@m.x"},
"url":"/api/v1/projects/foo/repos/baz",
"headRevision":3}]`)
	})

	repos, _, _ := c.ListRepositories(context.Background(), "foo")
	want := []*Repository{
		{Name: "bar", Creator: Author{Name: "minux", Email: "minux@m.x"},
			URL: "/api/v1/projects/foo/repos/bar", HeadRevision: 2},
		{Name: "baz", Creator: Author{Name: "minux", Email: "minux@m.x"},
			URL: "/api/v1/projects/foo/repos/baz", HeadRevision: 3}}
	if !reflect.DeepEqual(repos, want) {
		t.Errorf("ListRepositories returned %+v, want %+v", repos, want)
	}
}

func TestListRemovedRepository(t *testing.T) {
	c, mux, teardown := setup()
	defer teardown()

	mux.HandleFunc("/api/v1/projects/foo/repos", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, http.MethodGet)
		testURLQuery(t, r, "status", "removed")
		fmt.Fprint(w, `[{"name":"bar"}, {"name":"baz"}]`)
	})

	repos, _, _ := c.ListRemovedRepositories(context.Background(), "foo")
	want := []*Repository{{Name: "bar"}, {Name: "baz"}}
	if !reflect.DeepEqual(repos, want) {
		t.Errorf("ListRemovedRepositories returned %+v, want %+v", repos, want)
	}
}

func TestNormalizeRevision(t *testing.T) {
	c, mux, teardown := setup()
	defer teardown()

	mux.HandleFunc("/api/v1/projects/foo/repos/bar/revision/-2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, http.MethodGet)
		fmt.Fprint(w, `{"revision":3}`)
	})

	normalizedRevision, _, _ := c.NormalizeRevision(context.Background(), "foo", "bar", "-2")
	var want int64 = 3
	if normalizedRevision != want {
		t.Errorf("NormalizeRevision returned %v, want %v", normalizedRevision, want)
	}
}
