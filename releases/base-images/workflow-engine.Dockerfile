FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    unzip \
    sqlite3 \
    ssh \
    sshpass \
    rsync \
    && rm -rf /var/lib/apt/lists/*
