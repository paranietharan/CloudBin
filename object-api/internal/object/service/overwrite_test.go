package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"cloudbin-object-api/internal/object/model"
	"cloudbin-object-api/internal/object/repository"
)

type versionRepo struct {
	mockRepo
	current model.ObjectRecord
	reject  bool
}

func (r *versionRepo) FindByOwnerAndKey(context.Context, string, string) (model.ObjectRecord, error) {
	if r.current.StorageKey == "" {
		return model.ObjectRecord{}, repository.ErrNotFound
	}
	return r.current, nil
}

func (r *versionRepo) CommitVersion(_ context.Context, next model.ObjectRecord, _ *model.ObjectRecord) error {
	if r.reject {
		return errors.New("metadata write failed")
	}
	r.current = next
	return nil
}

type memoryNodes struct {
	mu       sync.Mutex
	objects  map[string]string
	failHost string
}

func (n *memoryNodes) RoundTrip(req *http.Request) (*http.Response, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	key := req.URL.String()
	if req.Method == http.MethodPut {
		if req.URL.Host == n.failHost {
			return nil, errors.New("replica unavailable")
		}
		data, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		n.objects[key] = string(data)
	}
	value, ok := n.objects[key]
	status := http.StatusOK
	if !ok {
		status = http.StatusNotFound
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(value))}, nil
}

func TestFailedOverwritePreservesPublishedBytesAndETag(t *testing.T) {
	for _, failure := range []string{"replica", "metadata"} {
		t.Run(failure, func(t *testing.T) {
			ctx := context.Background()
			repo := &versionRepo{}
			nodes := &memoryNodes{objects: make(map[string]string)}
			svc, err := New(repo, []string{"http://primary", "http://replica"}, 2, nil)
			if err != nil {
				t.Fatal(err)
			}
			svc.httpClient.Transport = nodes
			if err := svc.Upload(ctx, "owner", "key", "text/plain", []byte("A")); err != nil {
				t.Fatal(err)
			}
			etag := repo.current.ETag
			if failure == "replica" {
				nodes.failHost = "replica"
			} else {
				repo.reject = true
			}
			if err := svc.Upload(ctx, "owner", "key", "text/plain", []byte("B")); err == nil {
				t.Fatal("expected overwrite failure")
			}
			rec, body, err := svc.Download(ctx, "owner", "key")
			if err != nil {
				t.Fatal(err)
			}
			defer body.Close()
			data, err := io.ReadAll(body)
			if err != nil || string(data) != "A" || rec.ETag != etag {
				t.Fatalf("lost committed object: %q %s %v", data, rec.ETag, err)
			}
		})
	}
}
