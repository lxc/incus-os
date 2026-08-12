#!/bin/sh -eu

cd "$(git rev-parse --show-toplevel)"

# Regenerate the API spec (this also installs the swagger tool).
make update-api

# Check that the committed API spec is up to date.
if [ -n "$(git status --porcelain -- doc/rest-api.yaml)" ]; then
  git status -- doc/rest-api.yaml
  echo "==> Please run 'make update-api' and commit the changes to doc/rest-api.yaml"
  exit 1
fi

# Validate the API spec.
swagger validate doc/rest-api.yaml
