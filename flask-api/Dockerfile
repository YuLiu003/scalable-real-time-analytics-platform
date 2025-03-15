FROM python:3.9-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY src/ .

EXPOSE 5000

# Use python directly for development or gunicorn for production
CMD ["python", "app.py"]
# CMD ["gunicorn", "--bind", "0.0.0.0:5000", "app:app"]
