FROM python:3.10-slim

WORKDIR /app

COPY requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r requirements.txt

# Copy the entire src directory
COPY src/ /app/src/

# Make the directory a proper Python package
RUN touch /app/src/__init__.py

ENV PYTHONPATH=/app
ENV PYTHONUNBUFFERED=1

# Specify the path to app.py more explicitly
CMD ["python", "src/app.py"]