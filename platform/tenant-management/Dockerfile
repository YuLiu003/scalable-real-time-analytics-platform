FROM python:3.9-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Create non-root user
RUN addgroup --system --gid 1001 appuser \
    && adduser --system --uid 1001 --gid 1001 --no-create-home appuser

# Set permissions
RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 5010

CMD ["python", "src/tenant_service.py"]