package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/kafkaclient"
)

func main() {
	topic := flag.String("topic", "market.prices", "Kafka topic to inspect")
	expected := flag.Int64("expected", -1, "expected record count")
	timeout := flag.Duration("timeout", 15*time.Second, "maximum inspection time")
	flag.Parse()

	settings, err := kafkaclient.FromEnvironment()
	if err != nil {
		fatal(err)
	}
	config, err := settings.Consumer("topic-inspector")
	if err != nil {
		fatal(err)
	}
	client, err := sarama.NewClient(settings.Brokers, config)
	if err != nil {
		fatal(err)
	}
	defer client.Close()
	partitions, err := client.Partitions(*topic)
	if err != nil {
		fatal(err)
	}

	deadline := time.Now().Add(*timeout)
	for {
		var count int64
		for _, partition := range partitions {
			oldest, err := client.GetOffset(*topic, partition, sarama.OffsetOldest)
			if err != nil {
				fatal(err)
			}
			newest, err := client.GetOffset(*topic, partition, sarama.OffsetNewest)
			if err != nil {
				fatal(err)
			}
			count += newest - oldest
		}
		if *expected < 0 || count == *expected {
			fmt.Printf("topic=%s records=%d partitions=%d\n", *topic, count, len(partitions))
			return
		}
		if count > *expected || time.Now().After(deadline) {
			fatal(fmt.Errorf("record count = %d, want %d", count, *expected))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
