from pydantic import BaseModel
from typing import List, Optional

class AnalyticsData(BaseModel):
    timestamp: str
    value: float
    device_id: str
    anomaly_score: Optional[float] = None

class AnalyticsResult(BaseModel):
    average: float
    max: float
    min: float
    count: int
    anomalies: List[AnalyticsData] = []