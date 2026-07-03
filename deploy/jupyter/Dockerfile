# Jupyter workbench: notebooks + a browser terminal wired into the platform.
# Build from the repository root:
#   docker build -f deploy/jupyter/Dockerfile -t mlaiops-jupyter .
#
# ML library versions are pinned to the serving image
# (deploy/mlflow/Dockerfile: mlflow 3.1.1, scikit-learn 1.7.0) so models
# trained in a notebook load under `mlflow models serve` without
# training-serving skew.
FROM python:3.11-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends git curl fuse3 s3fs \
 && printf 'user_allow_other\n' >> /etc/fuse.conf \
 && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir \
    "jupyterlab>=4.2,<5" \
    "scikit-learn==1.7.0" \
    "mlflow==3.1.1" \
    "pandas>=2.2" \
    "pyarrow>=16" \
    "boto3>=1.34" \
    "prefect>=3.0" \
    "redis>=5"

ARG TARGETARCH
ARG NODE_VERSION=22.17.0
RUN apt-get update \
 && apt-get install -y --no-install-recommends xz-utils \
 && curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${TARGETARCH}.tar.xz" \
    | tar -xJ --strip-components=1 -C /usr/local \
 && npm install -g @openai/codex@0.142.5 @anthropic-ai/claude-code@2.1.200 \
 && rm -rf /var/lib/apt/lists/* /root/.npm

# The platform SDK and the runnable platform modules (features, realtime,
# pipelines, agents) so notebooks and the terminal can drive the whole stack.
COPY python/pyproject.toml /opt/platform/python/pyproject.toml
COPY python/mlaiops_sdk /opt/platform/python/mlaiops_sdk
COPY python/features /opt/platform/python/features
COPY python/realtime /opt/platform/python/realtime
COPY python/pipelines /opt/platform/python/pipelines
COPY python/agents /opt/platform/python/agents
RUN pip install --no-cache-dir "/opt/platform/python[agents]"
RUN pip install --no-cache-dir "jupyter-ai==3.0.1"

COPY deploy/jupyter/quickstart.ipynb /opt/seed/quickstart.ipynb
COPY deploy/jupyter/entrypoint.sh /usr/local/bin/workbench
COPY deploy/workspace/nexus.py /usr/local/bin/nexus
RUN chmod +x /usr/local/bin/workbench \
 && chmod +x /usr/local/bin/nexus \
 && useradd --create-home --uid 1000 dev \
 && mkdir -p /workspace /workspace/object-store \
 && chown -R dev:dev /workspace

ENV PYTHONPATH=/opt/platform/python \
    SHELL=/bin/bash

WORKDIR /workspace
EXPOSE 8888
ENTRYPOINT ["workbench"]
