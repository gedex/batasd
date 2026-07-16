FROM node:22-bookworm-slim AS node

FROM python:3.12-slim-bookworm

COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules

RUN ln -s /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
	&& ln -s /usr/local/lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx \
	&& groupadd --gid 1000 runner \
	&& useradd --uid 1000 --gid 1000 --create-home --shell /usr/sbin/nologin runner \
	&& mkdir -p /workspace \
	&& chown runner:runner /workspace

WORKDIR /workspace
USER runner:runner
