package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type Settings struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

func FromEnvironment() (Settings, error) {
	settings := Settings{
		Endpoint:  os.Getenv("S3_ENDPOINT"),
		Region:    os.Getenv("AWS_REGION"),
		Bucket:    os.Getenv("S3_BUCKET"),
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
	if err := settings.validate(); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func (s Settings) validate() error {
	if s.Region == "" || s.Bucket == "" {
		return errors.New("AWS region and S3 bucket are required")
	}
	if (s.AccessKey == "") != (s.SecretKey == "") {
		return errors.New("S3 access key and secret key must be configured together")
	}
	if s.Endpoint != "" && s.AccessKey == "" {
		return errors.New("a custom S3 endpoint requires static access credentials")
	}
	return nil
}

type Store struct {
	client s3API
	bucket string
}

func New(ctx context.Context, settings Settings) (*Store, error) {
	return newWithConfigLoader(ctx, settings, config.LoadDefaultConfig)
}

type configLoader func(context.Context, ...func(*config.LoadOptions) error) (aws.Config, error)

type s3API interface {
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	DeleteObjects(context.Context, *s3.DeleteObjectsInput, ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
}

func newWithConfigLoader(ctx context.Context, settings Settings, load configLoader) (*Store, error) {
	if err := settings.validate(); err != nil {
		return nil, err
	}
	loadOptions := []func(*config.LoadOptions) error{config.WithRegion(settings.Region)}
	if settings.AccessKey != "" {
		loadOptions = append(loadOptions, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(settings.AccessKey, settings.SecretKey, "")))
	}
	awsConfig, err := load(ctx, loadOptions...)
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

func (s *Store) Latest(ctx context.Context, portfolioID string) ([]byte, error) {
	key := fmt.Sprintf("gold/portfolio_allocations/v2/portfolio=%s/latest.json", portfolioID)
	return s.Get(ctx, key)
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	const maximumResultBytes = 2 << 20

	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, fmt.Errorf("get portfolio result: %w", err)
	}
	defer output.Body.Close()
	data, err := io.ReadAll(io.LimitReader(output.Body, maximumResultBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read portfolio result: %w", err)
	}
	if len(data) > maximumResultBytes {
		return nil, errors.New("portfolio result exceeds 2 MiB limit")
	}
	return data, nil
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
			return nil, fmt.Errorf("list objects for prefix %s: %w", prefix, err)
		}
		for _, object := range output.Contents {
			keys = append(keys, aws.ToString(object.Key))
		}
		if !aws.ToBool(output.IsTruncated) {
			return keys, nil
		}
		continuation = output.NextContinuationToken
	}
}

func (s *Store) DeletePrefix(ctx context.Context, prefix string) (int, error) {
	keys, err := s.ListKeys(ctx, prefix)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for start := 0; start < len(keys); start += 1000 {
		end := start + 1000
		if end > len(keys) {
			end = len(keys)
		}
		objects := make([]s3types.ObjectIdentifier, 0, end-start)
		for _, key := range keys[start:end] {
			objects = append(objects, s3types.ObjectIdentifier{Key: aws.String(key)})
		}
		output, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &s3types.Delete{Objects: objects, Quiet: aws.Bool(true)},
		})
		if err != nil {
			return deleted, fmt.Errorf("delete objects for prefix %s: %w", prefix, err)
		}
		if len(output.Errors) != 0 {
			return deleted, fmt.Errorf("delete objects for prefix %s returned %d errors", prefix, len(output.Errors))
		}
		deleted += len(objects)
	}
	return deleted, nil
}
