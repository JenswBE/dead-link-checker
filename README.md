[![Test, build and publish](https://github.com/JenswBE/dead-link-checker/actions/workflows/test-build-publish.yml/badge.svg?branch=main)](https://github.com/JenswBE/dead-link-checker/actions/workflows/test-build-publish.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/jenswbe/dead-link-checker)](https://hub.docker.com/r/jenswbe/dead-link-checker)

# Dead Link Checker (DeLiC)

Dead link checker written in Golang using [Colly](https://github.com/gocolly/colly).

## 1. Config

### Full example

```yaml
# Required
sites:
  - url: https://jensw.be
    ignored_links: # Optional, list of regex's which should be ignored
      - ^https://jensw.be/do-not-visit-me.*
    notify: # Optional, send notification to these notifiers by name
      - email_technical_en

# Optional, globally ignored links (see sites.ignored_links)
ignored_links:
  - ^https://jensw.be/also-not-visit-me.*

# Optional, can also be set as environment variable VERBOSE.
# Default is False.
verbose: False

# Optional, run every night.
# If omitted or empty, DeLiC will check the provided sites once and exit.
cron: "0 0 * * *"

# Optional, endpoint to send GET request to on a successful run.
# If omitted or empty, health check request will be skipped.
health_check:
  url: "https://example.com/delic-health-check"

# Optional, see https://github.com/containrrr/shoutrrr/blob/main/docs/services/overview.md for supported URL's.
# If using "smtp", make sure to set option "usehtml=true".
notifiers:
  - name: email_technical_en
    url: smtp://smpt4dev:smpt4dev@localhost:8025/?from=delic@localhost&to=admin1@localhost,admin2@localhost&usehtml=true&subject=Broken%20links%20found
    template_name: "technical_en" # Currently only "technical_en" and "simple_nl" supported
```

### Minimal example

Probably only useful with `--json` flag.

```yaml
sites:
  - url: https://jensw.be
```

## 2. Run DeLiC

By default, DeLiC tries to read the config file at `./config.yml`.

```bash
docker run --user "$(id -u):$(id -g)" -v /path/to/config.yml:/config.yml:ro,z jenswbe/dead-link-checker
```

or with Docker Compose:

```yaml
services:
  delic:
    image: docker.io/jenswbe/dead-link-checker:latest
    user: "${UID}:${GID}"
    volumes:
      - "./config.yml:/config.yml:ro,z"
```

## 3. CLI arguments

```
-c, --config    Location of the config file
    --json      Print all site reports as JSON to stdout
    --now       Overrides cron and forces an immediate check
-v, --verbose   Enable verbose output
```

# Development

## Running E2E tests

```bash
cd e2e
docker compose up -d
go test --tags e2e ./...
```
