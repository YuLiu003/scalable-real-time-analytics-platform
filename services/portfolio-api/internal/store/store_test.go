package store

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type fakeS3 struct {
	get    func(*s3.GetObjectInput) (*s3.GetObjectOutput, error)
	list   func(*s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
	delete func(*s3.DeleteObjectsInput) (*s3.DeleteObjectsOutput, error)
}

func (f fakeS3) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return f.get(input)
}

func (f fakeS3) ListObjectsV2(_ context.Context, input *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return f.list(input)
}

func (f fakeS3) DeleteObjects(_ context.Context, input *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	return f.delete(input)
}

type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (failingReadCloser) Close() error             { return nil }

func validSettings() Settings {
	return Settings{
		Endpoint:  "http://127.0.0.1:3900",
		Region:    "garage",
		Bucket:    "analytics",
		AccessKey: "access",
		SecretKey: "secret",
	}
}

func TestFromEnvironmentSupportsLocalAndWorkloadIdentitySettings(t *testing.T) {
	for _, name := range []string{"S3_ENDPOINT", "AWS_REGION", "S3_BUCKET", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"} {
		t.Setenv(name, "")
	}
	if _, err := FromEnvironment(); err == nil {
		t.Fatal("FromEnvironment() accepted missing settings")
	}
	t.Setenv("S3_ENDPOINT", "http://garage")
	t.Setenv("AWS_REGION", "garage")
	t.Setenv("S3_BUCKET", "analytics")
	t.Setenv("AWS_ACCESS_KEY_ID", "access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	settings, err := FromEnvironment()
	if err != nil || settings.Bucket != "analytics" {
		t.Fatalf("FromEnvironment() = %+v, %v", settings, err)
	}

	t.Setenv("S3_ENDPOINT", "")
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	settings, err = FromEnvironment()
	if err != nil || settings.Endpoint != "" || settings.AccessKey != "" {
		t.Fatalf("FromEnvironment(workload identity) = %+v, %v", settings, err)
	}

	t.Setenv("AWS_ACCESS_KEY_ID", "partial")
	if _, err := FromEnvironment(); err == nil {
		t.Fatal("FromEnvironment() accepted partial static credentials")
	}
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("S3_ENDPOINT", "http://garage")
	if _, err := FromEnvironment(); err == nil {
		t.Fatal("FromEnvironment() accepted a custom endpoint without credentials")
	}
}

func TestNewBuildsClientAndPropagatesConfigurationError(t *testing.T) {
	store, err := New(context.Background(), validSettings())
	if err != nil || store.client == nil || store.bucket != "analytics" {
		t.Fatalf("New() = %+v, %v", store, err)
	}
	want := errors.New("configuration failed")
	_, err = newWithConfigLoader(context.Background(), validSettings(), func(context.Context, ...func(*config.LoadOptions) error) (aws.Config, error) {
		return aws.Config{}, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("newWithConfigLoader() error = %v", err)
	}

	cloud := Settings{Region: "us-west-2", Bucket: "analytics"}
	store, err = newWithConfigLoader(context.Background(), cloud, func(_ context.Context, options ...func(*config.LoadOptions) error) (aws.Config, error) {
		if len(options) != 1 {
			t.Fatalf("workload identity config options = %d, want 1", len(options))
		}
		return aws.Config{Region: "us-west-2"}, nil
	})
	if err != nil || store.client == nil {
		t.Fatalf("newWithConfigLoader(workload identity) = %+v, %v", store, err)
	}
	if _, err := newWithConfigLoader(context.Background(), Settings{}, func(context.Context, ...func(*config.LoadOptions) error) (aws.Config, error) {
		return aws.Config{}, nil
	}); err == nil {
		t.Fatal("newWithConfigLoader() accepted invalid settings")
	}
}

func TestGetAndLatest(t *testing.T) {
	client := fakeS3{get: func(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
		if !strings.HasSuffix(aws.ToString(input.Key), "/latest.json") {
			t.Fatalf("key = %q", aws.ToString(input.Key))
		}
		return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader("result"))}, nil
	}}
	store := &Store{client: client, bucket: "analytics"}
	data, err := store.Latest(context.Background(), "demo")
	if err != nil || string(data) != "result" {
		t.Fatalf("Latest() = %q, %v", data, err)
	}
}

func TestGetRejectsClientReadAndSizeFailures(t *testing.T) {
	tests := []struct {
		name   string
		client fakeS3
	}{
		{
			name: "client",
			client: fakeS3{get: func(*s3.GetObjectInput) (*s3.GetObjectOutput, error) {
				return nil, errors.New("get failed")
			}},
		},
		{
			name: "read",
			client: fakeS3{get: func(*s3.GetObjectInput) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{Body: failingReadCloser{}}, nil
			}},
		},
		{
			name: "size",
			client: fakeS3{get: func(*s3.GetObjectInput) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), (2<<20)+1)))}, nil
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &Store{client: tt.client, bucket: "analytics"}
			if _, err := store.Get(context.Background(), "key"); err == nil {
				t.Fatal("Get() unexpectedly succeeded")
			}
		})
	}
}

func TestListKeysPaginatesAndReportsFailure(t *testing.T) {
	calls := 0
	client := fakeS3{list: func(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
		calls++
		if calls == 1 {
			return &s3.ListObjectsV2Output{
				Contents:              []s3types.Object{{Key: aws.String("a")}},
				IsTruncated:           aws.Bool(true),
				NextContinuationToken: aws.String("next"),
			}, nil
		}
		if aws.ToString(input.ContinuationToken) != "next" {
			t.Fatalf("continuation = %q", aws.ToString(input.ContinuationToken))
		}
		return &s3.ListObjectsV2Output{Contents: []s3types.Object{{Key: aws.String("b")}}}, nil
	}}
	store := &Store{client: client, bucket: "analytics"}
	keys, err := store.ListKeys(context.Background(), "gold/")
	if err != nil || strings.Join(keys, ",") != "a,b" {
		t.Fatalf("ListKeys() = %v, %v", keys, err)
	}

	store.client = fakeS3{list: func(*s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
		return nil, errors.New("list failed")
	}}
	if _, err := store.ListKeys(context.Background(), "gold/"); err == nil {
		t.Fatal("ListKeys() unexpectedly succeeded")
	}
}

func TestDeletePrefixHandlesEmptyBatchesAndFailures(t *testing.T) {
	list := func(keys []string) func(*s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
		return func(*s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
			objects := make([]s3types.Object, 0, len(keys))
			for _, key := range keys {
				objects = append(objects, s3types.Object{Key: aws.String(key)})
			}
			return &s3.ListObjectsV2Output{Contents: objects}, nil
		}
	}

	empty := &Store{client: fakeS3{list: list(nil)}, bucket: "analytics"}
	if deleted, err := empty.DeletePrefix(context.Background(), "gold/"); err != nil || deleted != 0 {
		t.Fatalf("DeletePrefix(empty) = %d, %v", deleted, err)
	}

	keys := make([]string, 1001)
	for index := range keys {
		keys[index] = "key"
	}
	deleteCalls := 0
	batched := &Store{client: fakeS3{
		list: list(keys),
		delete: func(input *s3.DeleteObjectsInput) (*s3.DeleteObjectsOutput, error) {
			deleteCalls++
			if count := len(input.Delete.Objects); count != 1000 && count != 1 {
				t.Fatalf("batch size = %d", count)
			}
			return &s3.DeleteObjectsOutput{}, nil
		},
	}, bucket: "analytics"}
	if deleted, err := batched.DeletePrefix(context.Background(), "gold/"); err != nil || deleted != 1001 || deleteCalls != 2 {
		t.Fatalf("DeletePrefix(batched) = %d, %v, calls=%d", deleted, err, deleteCalls)
	}

	listFailure := &Store{client: fakeS3{list: func(*s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
		return nil, errors.New("list failed")
	}}, bucket: "analytics"}
	if _, err := listFailure.DeletePrefix(context.Background(), "gold/"); err == nil {
		t.Fatal("DeletePrefix() accepted a list failure")
	}

	for _, tt := range []struct {
		name   string
		result *s3.DeleteObjectsOutput
		err    error
	}{
		{name: "request failure", result: nil, err: errors.New("delete failed")},
		{name: "object errors", result: &s3.DeleteObjectsOutput{Errors: []s3types.Error{{Code: aws.String("Denied")}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := &Store{client: fakeS3{
				list: list([]string{"key"}),
				delete: func(*s3.DeleteObjectsInput) (*s3.DeleteObjectsOutput, error) {
					return tt.result, tt.err
				},
			}, bucket: "analytics"}
			if _, err := store.DeletePrefix(context.Background(), "gold/"); err == nil {
				t.Fatal("DeletePrefix() unexpectedly succeeded")
			}
		})
	}
}
