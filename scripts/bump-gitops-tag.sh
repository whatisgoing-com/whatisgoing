#!/bin/sh
set -eu

SERVICE="$1"
TAG="$2"
GITOPS_REPO="github.com/whatisgoing-com/whatisgoing-gitops.git"
VALUES_FILE="apps/${SERVICE}/values.yaml"

WORKDIR=$(mktemp -d)
git clone "https://x-access-token:${GITOPS_TOKEN}@${GITOPS_REPO}" "$WORKDIR"
cd "$WORKDIR"

sed -i.bak -E "s/^(\s*tag:\s*).*/\1\"${TAG}\"/" "$VALUES_FILE"
rm -f "${VALUES_FILE}.bak"

git config user.name "drone-ci"
git config user.email "drone-ci@whatisgoing.com"
git add "$VALUES_FILE"
git commit -m "chore(${SERVICE}): bump image tag to ${TAG}"
git push origin main
