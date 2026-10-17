# The recorder's own image, which carries the font the tapes name, plus the two things a completion demo
# needs on top: zsh, and a claude to read the commands and flags from.
FROM ghcr.io/charmbracelet/vhs:v0.11.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends zsh curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://claude.ai/install.sh | bash

ENV PATH=/root/.local/bin:$PATH
