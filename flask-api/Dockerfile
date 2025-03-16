FROM python:3.10-slim

WORKDIR /app

# Copy requirements file
COPY flask-api/requirements.txt /app/requirements.txt

# Install dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy source code
COPY flask-api/src/ /app/src/

EXPOSE 5000

CMD ["python", "-m", "src.app"]
