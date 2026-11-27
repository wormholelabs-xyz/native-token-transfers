# syntax=docker.io/docker/dockerfile:1.3@sha256:42399d4635eddd7a9b8a24be879d2f9a930d0ed040a61324cfdf59ef1357b3b2
# Runner image for the Canton <-> Ethereum NTT end-to-end test (Tiltfile.canton-evm).
# Build context: the repository root.
#
# It holds both toolchains: dpm (Daml SDK) with a JDK for the Canton steps, and
# forge/cast for the EVM steps. It builds for the native platform, so it runs without
# emulation on arm64 hosts too.

# The JDK image the wormhole repo's canton/Dockerfile uses.
FROM eclipse-temurin:17-jdk@sha256:91b6210cce02091f6f0798a83ec51aa223828242c5a21a85793bb8c28dc891c4

ARG DAML_SDK_VERSION=3.5.1
ARG FOUNDRY_VERSION=v1.0.0
ARG FOUNDRY_SHA256_AMD64=8c078eaced6bfce76af902def65e36db677842ffbf53f76730647082a5b1e45a
ARG FOUNDRY_SHA256_ARM64=79967815024de284265b9e82b75498b46357e597b93b481b9f4abe88a53eade6
ARG TARGETARCH

RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates jq \
  && rm -rf /var/lib/apt/lists/*

# forge and cast from the foundry release, checked against the pinned sha256.
RUN case "${TARGETARCH}" in \
      amd64) sha256="${FOUNDRY_SHA256_AMD64}" ;; \
      arm64) sha256="${FOUNDRY_SHA256_ARM64}" ;; \
      *) echo "unsupported architecture: ${TARGETARCH}" >&2; exit 1 ;; \
    esac \
  && curl -fsSL -o /tmp/foundry.tar.gz \
    "https://github.com/foundry-rs/foundry/releases/download/${FOUNDRY_VERSION}/foundry_${FOUNDRY_VERSION}_linux_${TARGETARCH}.tar.gz" \
  && echo "${sha256}  /tmp/foundry.tar.gz" | sha256sum -c - \
  && tar -xzf /tmp/foundry.tar.gz -C /usr/local/bin forge cast \
  && rm /tmp/foundry.tar.gz

# dpm and the Daml SDK, as the wormhole repo's canton/Dockerfile installs them.
RUN curl -fsSL https://get.digitalasset.com/install/install.sh | sh
ENV PATH="/root/.dpm/bin:${PATH}"
RUN dpm install "${DAML_SDK_VERSION}"

WORKDIR /ntt

# EVM contracts. Compiling here downloads solc and fills forge's cache, so the
# test's forge create and forge script runs do not compile again. The prod profile
# (via-IR) keeps the NttManager under the 24 KB contract size limit; run.sh uses it too.
ENV FOUNDRY_PROFILE=prod
COPY evm/foundry.toml evm/foundry.toml
COPY evm/lib evm/lib
COPY evm/src evm/src
COPY evm/script evm/script
RUN cd evm && forge build

# Canton NTT packages: ntt, then ntt-test (which holds Test.TiltE2E).
COPY canton/multi-package.yaml canton/multi-package.yaml
COPY canton/dars canton/dars
COPY canton/ntt canton/ntt
COPY canton/test canton/test
RUN cd canton && dpm build --all

COPY canton/e2e/run.sh canton/e2e/run.sh

CMD ["/ntt/canton/e2e/run.sh"]
