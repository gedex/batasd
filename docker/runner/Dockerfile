FROM node:22-bookworm-slim AS node
FROM golang:1.24-bookworm AS go
FROM rust:1.88-bookworm AS rust

FROM ubuntu:24.04

COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules
COPY --from=go /usr/local/go /usr/local/go
COPY --from=rust /usr/local/cargo /usr/local/cargo
COPY --from=rust /usr/local/rustup /usr/local/rustup

ENV DEBIAN_FRONTEND=noninteractive
ENV CARGO_HOME=/usr/local/cargo
ENV RUSTUP_HOME=/usr/local/rustup
ENV PATH=/usr/local/go/bin:/usr/local/cargo/bin:$PATH

RUN apt-get update \
	&& apt-get install -y --no-install-recommends \
		ca-certificates \
		g++ \
		gcc \
		libc6-dev \
		make \
		openjdk-21-jdk-headless \
		php8.3-cli \
		python3 \
	&& rm -rf /var/lib/apt/lists/* \
	&& ln -s /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
	&& ln -s /usr/local/lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx \
	&& groupadd --gid 10001 runner \
	&& useradd --uid 10001 --gid 10001 --create-home --shell /usr/sbin/nologin runner \
	&& mkdir -p /workspace \
	&& chown runner:runner /workspace

WORKDIR /workspace
USER runner:runner
