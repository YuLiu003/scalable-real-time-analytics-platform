from datetime import datetime, timedelta

class RetentionPolicy:
    def __init__(self, retention_period_days):
        self.retention_period = timedelta(days=retention_period_days)

    def is_data_expired(self, data_timestamp):
        return datetime.now() - data_timestamp > self.retention_period

    def clean_expired_data(self, data_records):
        current_time = datetime.now()
        return [
            record for record in data_records
            if current_time - record['timestamp'] <= self.retention_period
        ]

# Example usage
if __name__ == "__main__":
    retention_policy = RetentionPolicy(retention_period_days=30)
    sample_data = [
        {'id': 1, 'timestamp': datetime.now() - timedelta(days=10)},
        {'id': 2, 'timestamp': datetime.now() - timedelta(days=40)},
        {'id': 3, 'timestamp': datetime.now() - timedelta(days=20)},
    ]

    cleaned_data = retention_policy.clean_expired_data(sample_data)
    print(f"Cleaned Data: {cleaned_data}")