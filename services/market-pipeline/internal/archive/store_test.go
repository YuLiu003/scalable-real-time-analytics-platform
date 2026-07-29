package archive

import (
	"context"
	"testing"
)

func TestNewSupportsStaticAndWorkloadIdentityCredentials(t *testing.T) {
	for _, settings := range []Settings{
		{Endpoint: "http://garage", Region: "garage", Bucket: "analytics", AccessKey: "access", SecretKey: "secret"},
		{Region: "us-west-2", Bucket: "analytics"},
	} {
		store, err := New(context.Background(), settings)
		if err != nil || store == nil {
			t.Fatalf("New(%+v) = %+v, %v", settings, store, err)
		}
	}
	for _, settings := range []Settings{
		{},
		{Region: "us-west-2", Bucket: "analytics", AccessKey: "partial"},
		{Endpoint: "http://garage", Region: "garage", Bucket: "analytics"},
	} {
		if _, err := New(context.Background(), settings); err == nil {
			t.Fatalf("New(%+v) unexpectedly succeeded", settings)
		}
	}
}

func TestContentHashIsStable(t *testing.T) {
	first := contentHash([]byte(`{"event_id":"one"}`))
	second := contentHash([]byte(`{"event_id":"one"}`))
	different := contentHash([]byte(`{"event_id":"two"}`))
	if first != second {
		t.Fatal("identical content produced different hashes")
	}
	if first == different {
		t.Fatal("different content produced an identical hash")
	}
}

func TestIsNotFoundRejectsOrdinaryErrors(t *testing.T) {
	if isNotFound(assertionError("network unavailable")) {
		t.Fatal("ordinary errors must not be treated as an absent object")
	}
}

type assertionError string

func (e assertionError) Error() string { return string(e) }
