from sqlalchemy import create_engine, Column, Integer, Float, String, DateTime
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker
import pandas as pd

Base = declarative_base()

class TimeSeriesData(Base):
    __tablename__ = 'time_series_data'

    id = Column(Integer, primary_key=True)
    timestamp = Column(DateTime, nullable=False)
    value = Column(Float, nullable=False)
    metric_name = Column(String, nullable=False)

class TimeSeriesDatabase:
    def __init__(self, db_url):
        self.engine = create_engine(db_url)
        Base.metadata.create_all(self.engine)
        self.Session = sessionmaker(bind=self.engine)

    def insert_data(self, timestamp, value, metric_name):
        session = self.Session()
        new_data = TimeSeriesData(timestamp=timestamp, value=value, metric_name=metric_name)
        session.add(new_data)
        session.commit()
        session.close()

    def query_data(self, start_time, end_time, metric_name):
        session = self.Session()
        results = session.query(TimeSeriesData).filter(
            TimeSeriesData.timestamp >= start_time,
            TimeSeriesData.timestamp <= end_time,
            TimeSeriesData.metric_name == metric_name
        ).all()
        session.close()
        return pd.DataFrame([(data.timestamp, data.value) for data in results], columns=['timestamp', 'value'])

class TimeseriesData:
    def __init__(self, tenant_id=None):
        self.tenant_id = tenant_id
    
    def store(self, data):
        # Ensure data has tenant_id for proper isolation
        if self.tenant_id and not data.get('tenant_id'):
            data['tenant_id'] = self.tenant_id
        
        # Existing store logic
        # ...
    
    def query(self, params):
        # Enforce tenant isolation in queries
        if self.tenant_id:
            params['tenant_id'] = self.tenant_id
        
        # Existing query logic
        # ...