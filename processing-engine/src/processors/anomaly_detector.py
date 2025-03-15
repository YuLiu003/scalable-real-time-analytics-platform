from sklearn.ensemble import IsolationForest
import numpy as np

class AnomalyDetector:
    def __init__(self, contamination=0.1):
        self.contamination = contamination
        self.model = IsolationForest(contamination=self.contamination)

    def fit(self, data):
        self.model.fit(data)

    def predict(self, data):
        predictions = self.model.predict(data)
        return [1 if pred == -1 else 0 for pred in predictions]

    def fit_predict(self, data):
        self.fit(data)
        return self.predict(data)