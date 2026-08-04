package archive

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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

func TestCountKeysStreamsPages(t *testing.T) {
	client := &fakeObjectClient{pages: []*s3.ListObjectsV2Output{
		{
			Contents:              []types.Object{{Key: aws.String("one")}, {Key: aws.String("two")}},
			IsTruncated:           aws.Bool(true),
			NextContinuationToken: aws.String("next"),
		},
		{Contents: []types.Object{{Key: aws.String("three")}}},
	}}
	store := &Store{client: client, bucket: "archive"}
	count, err := store.CountKeys(context.Background(), "bronze/")
	if err != nil || count != 3 || client.calls != 2 || client.secondToken != "next" {
		t.Fatalf("CountKeys() = %d, %v; client = %+v", count, err, client)
	}

	client = &fakeObjectClient{listErr: errors.New("unavailable")}
	store.client = client
	if _, err := store.CountKeys(context.Background(), "bronze/"); err == nil || !strings.Contains(err.Error(), "list archive objects") {
		t.Fatalf("CountKeys() error = %v", err)
	}
}

type fakeObjectClient struct {
	pages       []*s3.ListObjectsV2Output
	listErr     error
	calls       int
	secondToken string
}

func (*fakeObjectClient) HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return nil, errors.New("not implemented")
}

func (*fakeObjectClient) PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeObjectClient) ListObjectsV2(_ context.Context, input *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	f.calls++
	if f.calls == 2 {
		f.secondToken = aws.ToString(input.ContinuationToken)
	}
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.pages[f.calls-1], nil
}

type assertionError string

func (e assertionError) Error() string { return string(e) }
