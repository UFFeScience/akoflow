ARG BUILDKIT_VERSION=v0.30.0
FROM moby/buildkit:${BUILDKIT_VERSION}

ARG AKOFLOW_VERSION=dev
LABEL org.opencontainers.image.title="AkôFlow BuildKit" \
      org.opencontainers.image.description="Pinned BuildKit daemon used by AkôFlow Desktop" \
      org.opencontainers.image.source="https://github.com/UFFeScience/akoflow" \
      org.opencontainers.image.vendor="UFFeScience" \
      org.opencontainers.image.version="${AKOFLOW_VERSION}"

EXPOSE 1234

HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=12 \
  CMD ["buildctl", "--addr", "tcp://127.0.0.1:1234", "debug", "workers"]

ENTRYPOINT ["buildkitd"]
CMD ["--addr", "tcp://0.0.0.0:1234"]
