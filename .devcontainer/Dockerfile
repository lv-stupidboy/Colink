FROM golang:1.25-bookworm

# ── System packages ──────────────────────────────────────────
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get install -y --no-install-recommends \
        # Shell essentials
        bash-completion \
        less \
        procps \
        psmisc \
        file \
        tree \
        jq \
        vim-tiny \
        # Build tools
        curl \
        ca-certificates \
        gnupg \
        make \
        git \
        sqlite3 \
        sudo \
        unzip \
        # Networking utilities
        dnsutils \
        netcat-openbsd \
        iputils-ping \
        # Misc
        locales \
        man-db \
    && locale-gen en_US.UTF-8 \
    && mkdir -p /etc/apt/keyrings \
    && curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg \
    && echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_20.x nodistro main" > /etc/apt/sources.list.d/nodesource.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends nodejs \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# ── Playwright system deps ───────────────────────────────────
RUN npx -y playwright@$(npm view @playwright/test version) install-deps chromium

# ── User setup ───────────────────────────────────────────────
RUN groupadd -r vscode && useradd -r -g vscode -m vscode \
    && echo "vscode ALL=(root) NOPASSWD:ALL" > /etc/sudoers.d/vscode \
    && chmod 440 /etc/sudoers.d/vscode

# ── Go environment ───────────────────────────────────────────
ENV GOPATH=/home/vscode/go
ENV GOROOT=/usr/local/go
ENV GOBIN=/home/vscode/go/bin
ENV GOMODCACHE=/home/vscode/go/pkg/mod
ENV LANG=en_US.UTF-8
ENV LANGUAGE=en_US:en
ENV LC_ALL=en_US.UTF-8

RUN go install golang.org/x/tools/gopls@latest
RUN go install github.com/go-delve/delve/cmd/dlv@latest
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest || \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /home/vscode/go/bin
RUN mkdir -p /home/vscode/.cache/go-build /home/vscode/go/pkg/mod /home/vscode/go/bin \
    && chown -R vscode:vscode /home/vscode

# ── Switch to vscode user ────────────────────────────────────
USER vscode
ENV HOME=/home/vscode
ENV PATH="/home/vscode/go/bin:/home/vscode/.npm-global/bin:${PATH}"

# ── Node.js global packages ──────────────────────────────────
RUN npm config set prefix ~/.npm-global \
    && npm install -g @anthropic-ai/claude-code opencode-ai

# ── Git config ───────────────────────────────────────────────
RUN git config --global core.autocrlf input \
    && git config --global core.whitespace cr-at-eol \
    && git config --global init.defaultBranch master \
    && git config --global pull.rebase true \
    && git config --global diff.colorMoved zebra

# ── Shell profile ────────────────────────────────────────────
RUN { \
    echo ''; \
    echo '# Go'; \
    echo 'export PATH=$HOME/go/bin:$PATH'; \
    echo 'export GOMODCACHE=$HOME/go/pkg/mod'; \
    echo ''; \
    echo '# Aliases'; \
    echo 'alias ll="ls -alF"'; \
    echo 'alias la="ls -A"'; \
    echo 'alias l="ls -CF"'; \
    echo 'alias g="git"'; \
    echo 'alias gg="git graph --oneline -20 2>/dev/null || git log --oneline -20"'; \
    echo 'alias m="make"'; \
    echo 'alias mr="make run"'; \
    echo 'alias mt="make test"'; \
    echo 'alias mb="make build"'; \
    echo 'alias mc="make clean"'; \
    echo 'alias dc="docker compose"'; \
    echo ''; \
    echo '# PS1'; \
    echo 'export PS1="\\[\\033[0;33m\\]colink\\[\\033[0m\\]:\\[\\033[0;36m\\]\\w\\[\\033[0m\\]\\$ "'; \
    } >> ~/.bashrc \
    && cp ~/.bashrc ~/.profile

WORKDIR /workspaces/isdp
