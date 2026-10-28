#!/usr/bin/env bash
# Build the ggtable image for the current commit and push it to GHCR from the
# local machine — no git push or GitHub Actions run needed. Authenticates to
# ghcr.io with the token of the logged-in `gh` CLI.
#
# Prints ONLY the sha image reference on stdout (progress goes to stderr), so
# it can be captured directly:
#
#   IMAGE=$(scripts/publish-image.sh)          # build + push sha-<sha10> and latest
#   scripts/publish-image.sh --no-push         # build only (local testing)
#
# Tags match CI (docker-build.yml): ghcr.io/<owner>/ggtable:sha-<sha10> and :latest.
# One-time setup: the gh token needs the write:packages scope:
#   gh auth refresh -h github.com -s write:packages
set -euo pipefail

PUSH=1
for arg in "$@"; do
  case "${arg}" in
    --no-push) PUSH=0 ;;
    -h|--help) sed -n '2,14p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "Error: unknown argument '${arg}' (expected --no-push)" >&2; exit 2 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}/.."

for tool in git docker gh; do
  command -v "${tool}" >/dev/null 2>&1 || { echo "Error: '${tool}' is not on PATH." >&2; exit 1; }
done

# The tag names a commit, so the build must contain exactly that commit.
if ! git diff --quiet HEAD --; then
  echo "Error: tracked files have uncommitted changes; commit them first so the" >&2
  echo "       image tag (sha-<commit>) matches what is inside the image." >&2
  exit 1
fi

GH_USER="$(gh api user -q .login)" || { echo "Error: 'gh' is not logged in. Run: gh auth login" >&2; exit 1; }
OWNER="${GGTABLE_IMAGE_OWNER:-${GH_USER}}"
OWNER="${OWNER,,}"   # GHCR repository names must be lowercase
SHA10="$(git rev-parse --short=10 HEAD)"
REPO="ghcr.io/${OWNER}/ggtable"
IMAGE="${REPO}:sha-${SHA10}"
LATEST="${REPO}:latest"

echo "Building ${IMAGE} (+ ${LATEST})" >&2
docker build . \
  -t "${IMAGE}" \
  -t "${LATEST}" \
  --build-arg "VERSION=${SHA10}" \
  --label "org.opencontainers.image.source=https://github.com/${OWNER}/ggtable" \
  >&2

if [[ "${PUSH}" -eq 1 ]]; then
  echo "Logging in to ghcr.io as ${GH_USER}" >&2
  if ! gh auth token | docker login ghcr.io -u "${GH_USER}" --password-stdin >&2; then
    echo "Error: ghcr.io login failed. The gh token likely lacks write:packages;" >&2
    echo "       run: gh auth refresh -h github.com -s write:packages" >&2
    exit 1
  fi
  for ref in "${IMAGE}" "${LATEST}"; do
    echo "Pushing ${ref}" >&2
    docker push "${ref}" >&2
  done
fi

echo "${IMAGE}"
