ARG PYTHON_IMAGE=python:3.13.14-slim
FROM ${PYTHON_IMAGE}

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app
COPY requirements.txt .
RUN python -m pip install --requirement requirements.txt

COPY portfolio_analytics ./portfolio_analytics
COPY tests ./tests

USER 65532:65532
ENTRYPOINT ["python", "-m", "portfolio_analytics.builder"]
