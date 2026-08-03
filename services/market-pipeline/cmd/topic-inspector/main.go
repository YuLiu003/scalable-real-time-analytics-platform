package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/kafkaclient"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/scale"
)

func main() {
	topic := flag.String("topic", "market.prices", "Kafka topic to inspect")
	expected := flag.Int64("expected", -1, "expected record count")
	timeout := flag.Duration("timeout", 15*time.Second, "maximum inspection time")
	runID := flag.String("run-id", "", "validate only records for this scale run")
	phase := flag.String("phase", "original", "scale run phase to validate")
	group := flag.String("group", "", "validate this consumer group")
	lag := flag.Bool("lag", false, "report committed-offset lag for every topic partition")
	requiredClientHost := flag.String("require-client-host", "", "require this client host in the consumer group")
	expectedMembers := flag.Int("expected-members", 0, "expected consumer group member count")
	expectedPartitions := flag.Int("expected-partitions", 0, "expected assigned topic partition count")
	output := flag.String("output", "text", "text or json")
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
	if *group != "" {
		admin, err := sarama.NewClusterAdminFromClient(client)
		if err != nil {
			fatal(err)
		}
		defer admin.Close()
		if *lag {
			if *expectedPartitions < 1 {
				fatal(fmt.Errorf("--expected-partitions is required with --lag"))
			}
			partitions, err := client.Partitions(*topic)
			if err != nil {
				fatal(err)
			}
			response, err := admin.ListConsumerGroupOffsets(*group, map[string][]int32{*topic: partitions})
			if err != nil {
				fatal(err)
			}
			offsets := make(map[int32]scale.PartitionOffsets, len(partitions))
			for _, partition := range partitions {
				block := response.GetBlock(*topic, partition)
				committed := int64(-1)
				if block != nil && block.Err != sarama.ErrNoError {
					fatal(fmt.Errorf("consumer offset for partition %d is unavailable", partition))
				}
				if block != nil {
					committed = block.Offset
				}
				oldest, err := client.GetOffset(*topic, partition, sarama.OffsetOldest)
				if err != nil {
					fatal(err)
				}
				newest, err := client.GetOffset(*topic, partition, sarama.OffsetNewest)
				if err != nil {
					fatal(err)
				}
				offsets[partition] = scale.PartitionOffsets{Oldest: oldest, Newest: newest, Committed: committed}
			}
			report, err := scale.CalculateLag(offsets, *expectedPartitions)
			if err != nil {
				fatal(err)
			}
			if *output == "json" {
				if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
					fatal(err)
				}
				return
			}
			if *output != "text" {
				fatal(fmt.Errorf("--output must be text or json"))
			}
			fmt.Printf("group=%s topic=%s partitions=%d total_lag=%d max_partition_lag=%d\n", *group, *topic, report.PartitionCount, report.TotalLag, report.MaxPartitionLag)
			return
		}
		descriptions, err := admin.DescribeConsumerGroups([]string{*group})
		if err != nil {
			fatal(err)
		}
		if len(descriptions) != 1 || descriptions[0].Err != sarama.ErrNoError {
			fatal(fmt.Errorf("consumer group %s is unavailable", *group))
		}
		members := make([]scale.GroupMember, 0, len(descriptions[0].Members))
		for _, member := range descriptions[0].Members {
			members = append(members, scale.GroupMember{
				ClientHost: member.ClientHost,
				Assignment: member.MemberAssignment,
			})
		}
		report, err := scale.ValidateConsumerGroup(
			descriptions[0].State,
			members,
			*topic,
			*requiredClientHost,
			*expectedMembers,
			*expectedPartitions,
		)
		if err != nil {
			fatal(err)
		}
		if *output == "json" {
			if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
				fatal(err)
			}
			return
		}
		if *output != "text" {
			fatal(fmt.Errorf("--output must be text or json"))
		}
		fmt.Printf(
			"group=%s state=%s members=%d assigned_partitions=%d replacement_assigned=%t\n",
			*group,
			report.State,
			report.MemberCount,
			report.AssignedPartitionCount,
			report.RequiredClientHostPresent,
		)
		return
	}
	defer client.Close()
	partitions, err := client.Partitions(*topic)
	if err != nil {
		fatal(err)
	}
	if *runID != "" {
		if *expected < 0 {
			fatal(fmt.Errorf("--expected is required with --run-id"))
		}
		records, err := readRecords(client, *topic, partitions, *runID, *phase, time.Now().Add(*timeout))
		if err != nil {
			fatal(err)
		}
		report, err := scale.ValidateRun(records, *runID, *phase, *expected)
		if err != nil {
			fatal(err)
		}
		if *output == "json" {
			if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
				fatal(err)
			}
			return
		}
		if *output != "text" {
			fatal(fmt.Errorf("--output must be text or json"))
		}
		fmt.Printf(
			"run_id=%s phase=%s records=%d instruments=%d partitions=%d ordering_valid=%t\n",
			report.RunID,
			report.Phase,
			report.ObservedRecords,
			report.InstrumentCount,
			report.PartitionCount,
			report.OrderingValid,
		)
		return
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

func readRecords(client sarama.Client, topic string, partitions []int32, runID, phase string, deadline time.Time) ([]scale.Record, error) {
	consumer, err := sarama.NewConsumerFromClient(client)
	if err != nil {
		return nil, fmt.Errorf("create topic consumer: %w", err)
	}
	defer consumer.Close()
	var records []scale.Record
	for _, partition := range partitions {
		oldest, err := client.GetOffset(topic, partition, sarama.OffsetOldest)
		if err != nil {
			return nil, err
		}
		newest, err := client.GetOffset(topic, partition, sarama.OffsetNewest)
		if err != nil {
			return nil, err
		}
		partitionConsumer, err := consumer.ConsumePartition(topic, partition, oldest)
		if err != nil {
			return nil, fmt.Errorf("consume partition %d: %w", partition, err)
		}
		for offset := oldest; offset < newest; offset++ {
			wait := time.Until(deadline)
			if wait <= 0 {
				partitionConsumer.Close()
				return nil, fmt.Errorf("timed out reading topic %s", topic)
			}
			timer := time.NewTimer(wait)
			select {
			case message, ok := <-partitionConsumer.Messages():
				timer.Stop()
				if !ok {
					partitionConsumer.Close()
					return nil, fmt.Errorf("partition %d closed before the captured end offset", partition)
				}
				headers := make(map[string]string, len(message.Headers))
				for _, header := range message.Headers {
					headers[string(header.Key)] = string(header.Value)
				}
				if headers["run_id"] != runID || headers["phase"] != phase {
					continue
				}
				records = append(records, scale.Record{
					Partition: message.Partition,
					Key:       message.Key,
					Value:     message.Value,
					Headers:   headers,
				})
			case consumerError, ok := <-partitionConsumer.Errors():
				timer.Stop()
				if !ok {
					partitionConsumer.Close()
					return nil, fmt.Errorf("partition %d error channel closed before the captured end offset", partition)
				}
				partitionConsumer.Close()
				return nil, consumerError
			case <-timer.C:
				partitionConsumer.Close()
				return nil, fmt.Errorf("timed out reading topic %s", topic)
			}
		}
		if err := partitionConsumer.Close(); err != nil {
			return nil, err
		}
	}
	return records, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
