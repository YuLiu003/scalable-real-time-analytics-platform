package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/archive"
)

func main() {
	prefix := flag.String("prefix", "bronze/", "object key prefix")
	expected := flag.Int("expected", -1, "expected object count; negative disables assertion")
	flag.Parse()
	store, err := archive.New(context.Background(), archive.Settings{
		Endpoint:  os.Getenv("S3_ENDPOINT"),
		Region:    os.Getenv("AWS_REGION"),
		Bucket:    os.Getenv("S3_BUCKET"),
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	})
	if err != nil {
		fatal(err)
	}
	keys, err := store.ListKeys(context.Background(), *prefix)
	if err != nil {
		fatal(err)
	}
	result := struct {
		Prefix string `json:"prefix"`
		Count  int    `json:"count"`
	}{Prefix: *prefix, Count: len(keys)}
	encoded, _ := json.Marshal(result)
	fmt.Println(string(encoded))
	if *expected >= 0 && len(keys) != *expected {
		fatal(fmt.Errorf("object count = %d, want %d", len(keys), *expected))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
