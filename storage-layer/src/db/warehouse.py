from sqlalchemy import create_engine, Column, Integer, String, Float
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker

Base = declarative_base()

class DataWarehouse(Base):
    __tablename__ = 'data_warehouse'

    id = Column(Integer, primary_key=True)
    metric_name = Column(String, nullable=False)
    value = Column(Float, nullable=False)
    timestamp = Column(String, nullable=False)

class Warehouse:
    def __init__(self, db_url):
        self.engine = create_engine(db_url)
        Base.metadata.create_all(self.engine)
        self.Session = sessionmaker(bind=self.engine)

    def insert_data(self, metric_name, value, timestamp):
        session = self.Session()
        new_data = DataWarehouse(metric_name=metric_name, value=value, timestamp=timestamp)
        session.add(new_data)
        session.commit()
        session.close()

    def query_data(self, metric_name):
        session = self.Session()
        results = session.query(DataWarehouse).filter_by(metric_name=metric_name).all()
        session.close()
        return results

    def delete_data(self, metric_name):
        session = self.Session()
        session.query(DataWarehouse).filter_by(metric_name=metric_name).delete()
        session.commit()
        session.close()