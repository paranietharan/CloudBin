package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cloudbin-object-api/internal/object/model"

	"golang.org/x/sync/errgroup"
)

func (s *Service) writeReplicas(ctx context.Context, nodes []string, key, contentType string, data []byte) error {
	var writes errgroup.Group
	for _, node := range nodes {
		writes.Go(func() error { return s.putToNode(ctx, node, key, contentType, data) })
	}
	if err := writes.Wait(); err != nil {
		return ErrReplicationWriteFail
	}
	return nil
}

func (s *Service) openObject(ctx context.Context, rec model.ObjectRecord) (model.ObjectRecord, io.ReadCloser, error) {
	key := rec.BlobKey()
	stream, err := s.getFromNodeStream(ctx, rec.PrimaryNode, key)
	if err != nil && rec.ReplicaNode != "" {
		stream, err = s.getFromNodeStream(ctx, rec.ReplicaNode, key)
	}
	if err != nil {
		return model.ObjectRecord{}, nil, err
	}
	return rec, stream, nil
}

func nodeObjectURL(node, key string) string {
	return strings.TrimRight(node, "/") + "/objects/" + key
}

func (s *Service) putToNode(ctx context.Context, node, objectKey, contentType string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, nodeObjectURL(node, objectKey), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)

	res, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("storage node put failed: %d", res.StatusCode)
}

func (s *Service) getFromNodeStream(ctx context.Context, node, objectKey string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nodeObjectURL(node, objectKey), nil)
	if err != nil {
		return nil, err
	}
	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return nil, fmt.Errorf("storage node get failed: %d", res.StatusCode)
	}
	return res.Body, nil
}

func (s *Service) DeleteReplica(ctx context.Context, node, objectKey string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, nodeObjectURL(node, objectKey), nil)
	if err != nil {
		return err
	}
	res, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound || (res.StatusCode >= 200 && res.StatusCode < 300) {
		return nil
	}
	return fmt.Errorf("storage node delete failed: %d", res.StatusCode)
}
