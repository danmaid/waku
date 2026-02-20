
FROM mcr.microsoft.com/devcontainers/go:1.21

# Remove yarn repo to avoid GPG error
RUN rm -f /etc/apt/sources.list.d/yarn.list

# Install Docker CLI and Compose plugin from Docker official repo
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        gnupg \
        lsb-release \
    && mkdir -p /etc/apt/keyrings \
    && curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian $(lsb_release -cs) stable" > /etc/apt/sources.list.d/docker.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin \
    && rm -rf /var/lib/apt/lists/*

# DinD用: dockerd起動に必要なディレクトリ作成
RUN mkdir -p /var/lib/docker /var/run/docker

# vscodeユーザーをdockerグループに追加
RUN usermod -aG docker vscode

# dockerd起動用スクリプトを追加（後でエントリポイントやpostCreateCommandで使う）
COPY start-dind.sh /usr/local/bin/start-dind.sh
RUN chmod +x /usr/local/bin/start-dind.sh

# (Optional) Install docker-compose if needed
# RUN apt-get update \
#     && apt-get install -y --no-install-recommends docker-compose-plugin \
#     && rm -rf /var/lib/apt/lists/*
