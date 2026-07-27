ARG DOCKER_CLI_IMAGE
ARG PYTHON_IMAGE
FROM ${DOCKER_CLI_IMAGE} AS docker-cli

FROM ${PYTHON_IMAGE}

ARG TARGETARCH
ARG GO_VERSION
ARG GO_SHA256
ARG KIND_VERSION
ARG KUBERNETES_VERSION
ARG KUBECTL_SHA256
ARG HELM_VERSION
ARG HELM_SHA256
ARG OPENTOFU_VERSION
ARG OPENTOFU_SHA256
ARG ACTIONLINT_VERSION

ENV DEBIAN_FRONTEND=noninteractive
ENV PATH=/usr/local/go/bin:/home/jenkins/.local/bin:${PATH}
ENV PRESUBMIT_BIN_DIR=/usr/local/bin

COPY --from=docker-cli /usr/local/bin/docker /usr/local/bin/docker
COPY --from=docker-cli /usr/local/libexec/docker/cli-plugins/docker-buildx /usr/local/libexec/docker/cli-plugins/docker-buildx
COPY services/portfolio-analytics/requirements.txt /tmp/portfolio-requirements/requirements.txt
COPY services/portfolio-analytics/requirements-dev.txt /tmp/portfolio-requirements/requirements-dev.txt
COPY tools/codex-plugins/cloud-platform-engineering/mcp/requirements-dev.txt /tmp/plugin-requirements.txt

RUN set -eux; \
    apt-get update; \
    apt-get install --yes --no-install-recommends \
      bash ca-certificates curl gcc git jq libc6-dev make openjdk-21-jre-headless \
      openssh-client procps ripgrep tar unzip; \
    rm -rf /var/lib/apt/lists/*; \
    curl --fail --silent --show-error --location \
      "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" \
      --output /tmp/go.tar.gz; \
    printf '%s  %s\n' "${GO_SHA256}" /tmp/go.tar.gz | sha256sum --check --status; \
    tar -C /usr/local -xzf /tmp/go.tar.gz; \
    curl --fail --silent --show-error --location \
      "https://dl.k8s.io/release/${KUBERNETES_VERSION}/bin/linux/${TARGETARCH}/kubectl" \
      --output /usr/local/bin/kubectl; \
    printf '%s  %s\n' "${KUBECTL_SHA256}" /usr/local/bin/kubectl | sha256sum --check --status; \
    chmod 0755 /usr/local/bin/kubectl; \
    curl --fail --silent --show-error --location \
      "https://get.helm.sh/helm-${HELM_VERSION}-linux-${TARGETARCH}.tar.gz" \
      --output /tmp/helm.tar.gz; \
    printf '%s  %s\n' "${HELM_SHA256}" /tmp/helm.tar.gz | sha256sum --check --status; \
    tar -C /tmp -xzf /tmp/helm.tar.gz; \
    install -m 0755 "/tmp/linux-${TARGETARCH}/helm" /usr/local/bin/helm; \
    curl --fail --silent --show-error --location \
      "https://github.com/opentofu/opentofu/releases/download/v${OPENTOFU_VERSION}/tofu_${OPENTOFU_VERSION}_linux_${TARGETARCH}.zip" \
      --output /tmp/tofu.zip; \
    printf '%s  %s\n' "${OPENTOFU_SHA256}" /tmp/tofu.zip | sha256sum --check --status; \
    unzip -q /tmp/tofu.zip -d /tmp/tofu; \
    install -m 0755 /tmp/tofu/tofu /usr/local/bin/tofu; \
    GOBIN=/usr/local/bin go install "sigs.k8s.io/kind@${KIND_VERSION}"; \
    GOBIN=/usr/local/bin go install "github.com/rhysd/actionlint/cmd/actionlint@${ACTIONLINT_VERSION}"; \
    python -m pip install --no-cache-dir \
      --requirement /tmp/portfolio-requirements/requirements-dev.txt \
      --requirement /tmp/plugin-requirements.txt; \
    groupadd --gid 10000 jenkins; \
    useradd --create-home --uid 10000 --gid 10000 --shell /bin/bash jenkins; \
    mkdir -p /home/jenkins/agent; \
    chown -R 10000:10000 /home/jenkins; \
    rm -rf /tmp/*

USER 10000:10000
WORKDIR /home/jenkins/agent

RUN java -version && python --version && go version && \
    test "$(go env CGO_ENABLED)" = 1 && gcc --version && rg --version && docker --version && \
    kind version && kubectl version --client=true && helm version --short && tofu version
