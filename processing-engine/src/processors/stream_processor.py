from kafka import KafkaConsumer
import json
import time
from models.analytics import AnalyticsModel

class StreamProcessor:
    def __init__(self, kafka_topic, kafka_bootstrap_servers):
        self.consumer = KafkaConsumer(
            kafka_topic,
            bootstrap_servers=kafka_bootstrap_servers,
            value_deserializer=lambda m: json.loads(m.decode('utf-8')),
            auto_offset_reset='earliest',
            enable_auto_commit=True
        )

    def process_stream(self):
        for message in self.consumer:
            self.handle_message(message.value)

    def handle_message(self, data):
        analytics_data = AnalyticsModel(data)
        self.perform_analysis(analytics_data)

    def perform_analysis(self, analytics_data):
        # Implement your real-time analytics logic here
        print(f"Processing data: {analytics_data}")

if __name__ == "__main__":
    kafka_topic = "your_kafka_topic"
    kafka_bootstrap_servers = "localhost:9092"
    
    processor = StreamProcessor(kafka_topic, kafka_bootstrap_servers)
    processor.process_stream()