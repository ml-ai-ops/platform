FROM codercom/code-server:4.99.4

USER root
ARG TARGETARCH
ARG NODE_VERSION=22.17.0
RUN apt-get update \
 && apt-get install -y --no-install-recommends git curl python3 xz-utils \
 && curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${TARGETARCH}.tar.xz" \
    | tar -xJ --strip-components=1 -C /usr/local \
 && npm install -g @openai/codex@0.142.5 @anthropic-ai/claude-code@2.1.200 \
 && rm -rf /var/lib/apt/lists/* /root/.npm

COPY deploy/workspace/nexus.py /usr/local/bin/nexus
COPY deploy/ide/entrypoint.sh /usr/local/bin/ide-workspace
RUN chmod 0755 /usr/local/bin/nexus /usr/local/bin/ide-workspace \
 && mkdir -p /workspace \
 && chown -R coder:coder /workspace

WORKDIR /workspace
ENTRYPOINT ["dumb-init", "--", "ide-workspace"]
