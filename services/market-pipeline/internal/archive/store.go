package archive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type Result string

const (
	Created   Result = "created"
	Duplicate Result = "duplicate"
)

var ErrEventIDCollision = errors.New("event_id already exists with different content")

type Settings struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

type Store struct {
	client *s3.Client
	bucket string
}

func New(ctx context.Context, settings Settings) (*Store, error) {
	if settings.Region == "" || settings.Bucket == "" {
		return nil, errors.New("AWS region and S3 bucket are required")
	}
	if (settings.AccessKey == "") != (settings.SecretKey == "") {
		return nil, errors.New("S3 access key and secret key must be configured together")
	}
	if settings.Endpoint != "" && settings.AccessKey == "" {
		return nil, errors.New("a custom S3 endpoint requires static access credentials")
	}
	loadOptions := []func(*config.LoadOptions) error{config.WithRegion(settings.Region)}
	if settings.AccessKey != "" {
		loadOptions = append(loadOptions, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(settings.AccessKey, settings.SecretKey, "")))
	}
	awsConfig, err := config.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		if settings.Endpoint != "" {
			options.BaseEndpoint = aws.String(settings.Endpoint)
			options.UsePathStyle = true
		}
	})
	return &Store{client: client, bucket: settings.Bucket}, nil
}

func (s *Store) PutEvent(ctx context.Context, envelope event.Envelope, raw []byte) (Result, string, error) {
	key, err := envelope.ArchiveKey()
	if err != nil {
		return "", "", err
	}
	hash := contentHash(raw)
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err == nil {
		if strings.EqualFold(head.Metadata["sha256"], hash) {
			return Duplicate, key, nil
		}
		return "", key, ErrEventIDCollision
	}
	if !isNotFound(err) {
		return "", key, fmt.Errorf("head archive object: %w", err)
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(raw),
		ContentType: aws.String("application/json"),
		Metadata: map[string]string{
			"event-id":       envelope.EventID,
			"event-type":     envelope.EventType,
			"schema-version": fmt.Sprintf("%d", envelope.SchemaVersion),
			"sha256":         hash,
		},
	})
	if err != nil {
		return "", key, fmt.Errorf("put archive object: %w", err)
	}
	return Created, key, nil
}

func (s *Store) ListKeys(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	var continuation *string
	for {
		output, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuation,
		})
		if err != nil {
			return nil, fmt.Errorf("list archive objects: %w", err)
		}
		for _, object := range output.Contents {
			keys = append(keys, aws.ToString(object.Key))
		}
		if !aws.ToBool(output.IsTruncated) {
			break
		}
		continuation = output.NextContinuationToken
	}
	sort.Strings(keys)
	return keys, nil
}

func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func isNotFound(err error) bool {
	var apiError smithy.APIError
	if !errors.As(err, &apiError) {
		return false
	}
	return apiError.ErrorCode() == "NotFound" || apiError.ErrorCode() == "NoSuchKey"
}
