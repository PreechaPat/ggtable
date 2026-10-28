#!/usr/bin/env bash
# Build the ggtable image for the current commit and push it to GHCR from the
# local machine — no git push or GitHub Actions run needed. Authenticates to
# ghcr.io with the token of the logged-in `gh` CLI.
#
# Prints ONLY the sha image reference on stdout (progress goes to stderr), so
# it can be captured directly:
#
#   IMAGE=$(scripts/publish-image.sh)                  # build + push sha-<sha10> and latest
#   scripts/publish-image.sh --version 3.0.5           # also tag :3.0.5
#   scripts/publish-image.sh --no-push                 # build only (local testing)
#
# Version: --version X.Y.Z wins; otherwise, if HEAD carries a vX.Y.Z git tag,
# that is used. The version is added as an image tag and baked into the binary
# (shown in the startup log). Without one, the binary reports sha-<sha10>.
#
# Tags match CI (docker-build.yml): ghcr.io/<owner>/ggtable:sha-<sha10> and :latest.
# One-time setup: the gh token needs the write:packages scope:
#   gh auth refresh -h github.com -s write:packages
set -euo pipefail

PUSH=1
VERSION=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-push) PUSH=0; shift ;;
    --version)
      [[ $# -ge 2 ]] || { echo "Error: --version needs a value, e.g. --version 3.0.5" >&2; exit 2; }
      VERSION="$2"; shift 2 ;;
    --version=*) VERSION="${1#--version=}"; shift ;;
    -h|--help) sed -n '2,19p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "Error: unknown argument '$1' (expected --no-push or --version X.Y.Z)" >&2; exit 2 ;;
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

# Fall back to a vX.Y.Z git tag pointing at HEAD.
if [[ -z "${VERSION}" ]]; then
  GIT_TAG="$(git tag --points-at HEAD --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n1)"
  VERSION="${GIT_TAG#v}"
fi
VERSION="${VERSION#v}"   # accept both 3.0.5 and v3.0.5
if [[ -n "${VERSION}" && ! "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "Error: version '${VERSION}' is not semver-like (expected X.Y.Z or X.Y.Z-suffix)." >&2
  exit 2
fi

GH_USER="$(gh api user -q .login)" || { echo "Error: 'gh' is not logged in. Run: gh auth login" >&2; exit 1; }
OWNER="${GGTABLE_IMAGE_OWNER:-${GH_USER}}"
OWNER="${OWNER,,}"   # GHCR repository names must be lowercase
SHA10="$(git rev-parse --short=10 HEAD)"
REPO="ghcr.io/${OWNER}/ggtable"
IMAGE="${REPO}:sha-${SHA10}"
LATEST="${REPO}:latest"

REFS=("${IMAGE}" "${LATEST}")
if [[ -n "${VERSION}" ]]; then
  REFS+=("${REPO}:${VERSION}")
fi
APP_VERSION="${VERSION:-sha-${SHA10}}"

TAG_ARGS=()
for ref in "${REFS[@]}"; do TAG_ARGS+=(-t "${ref}"); done

echo "Building ${REFS[*]} (version ${APP_VERSION})" >&2
docker build . \
  "${TAG_ARGS[@]}" \
  --build-arg "VERSION=${APP_VERSION}" \
  --label "org.opencontainers.image.source=https://github.com/${OWNER}/ggtable" \
  --label "org.opencontainers.image.revision=$(git rev-parse HEAD)" \
  >&2

if [[ "${PUSH}" -eq 1 ]]; then
  echo "Logging in to ghcr.io as ${GH_USER}" >&2
  if ! gh auth token | docker login ghcr.io -u "${GH_USER}" --password-stdin >&2; then
    echo "Error: ghcr.io login failed. The gh token likely lacks write:packages;" >&2
    echo "       run: gh auth refresh -h github.com -s write:packages" >&2
    exit 1
  fi
  for ref in "${REFS[@]}"; do
    echo "Pushing ${ref}" >&2
    docker push "${ref}" >&2
  done
fi

echo "${IMAGE}"
