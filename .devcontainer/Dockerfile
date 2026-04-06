FROM golang:1.25-bookworm

RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get install -y --no-install-recommends \
        curl \
        ca-certificates \
        gnupg \
        make \
        git \
        sqlite3 \
        unzip \
    && mkdir -p /etc/apt/keyrings \
    && curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg \
    && echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_20.x nodistro main" > /etc/apt/sources.list.d/nodesource.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends nodejs \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN npx -y playwright@$(npm view @playwright/test version) install-deps chromium

RUN groupadd -r vscode && useradd -r -g vscode -m vscode \
    && mkdir -p /home/vscode/.cache/go-build /home/vscode/go/bin /home/vscode/.claude /home/vscode/.opencode \
    && chown -R vscode:vscode /home/vscode

ENV GOPATH=/home/vscode/go
ENV GOROOT=/usr/local/go
ENV GOBIN=/home/vscode/go/bin

RUN go install golang.org/x/tools/gopls@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest

USER vscode
ENV HOME=/home/vscode
ENV PATH="/home/vscode/go/bin:/home/vscode/.npm-global/bin:${PATH}"

RUN npm config set prefix ~/.npm-global \
    && npm install -g @anthropic-ai/claude-code opencode-ai

WORKDIR /workspaces/isdp
