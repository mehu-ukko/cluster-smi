#!/usr/bin/env bash

# Creates and pushes the next release tag (vX.Y.Z). Pushing the tag triggers
# the release job in .github/workflows/build.yml.
#
# The next version is suggested from the Conventional Commits since the last tag:
#   type!: ... / BREAKING CHANGE  -> major
#   feat: ...                     -> minor
#   anything else                 -> patch

set -euo pipefail

REMOTE="${REMOTE:-origin}"
BRANCH="${BRANCH:-master}"

function die() {
    printf 'error: %s\n' "$*" >&2
    exit 1
}

# preconditions
[[ "$(git rev-parse --abbrev-ref HEAD)" == "${BRANCH}" ]] || die "not on branch ${BRANCH}"
git diff --quiet HEAD || die "working tree has uncommitted changes"

git fetch --quiet --tags "${REMOTE}" "${BRANCH}"
[[ "$(git rev-parse HEAD)" == "$(git rev-parse "${REMOTE}/${BRANCH}")" ]] \
    || die "${BRANCH} is not in sync with ${REMOTE}/${BRANCH}"

# current (old) version
OLD_TAG=$(git describe --tags --abbrev=0 --match 'v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null || true)
if [[ -n "${OLD_TAG}" ]]; then
    RANGE="${OLD_TAG}..HEAD"
else
    OLD_TAG="v0.0.0"
    RANGE="HEAD"
fi

IFS=. read -r MAJOR MINOR PATCH <<< "${OLD_TAG#v}"

COMMITS=$(git log --no-merges --format='%h %s' "${RANGE}")
[[ -n "${COMMITS}" ]] || die "no new commits since ${OLD_TAG}"

# suggest next version
MESSAGES=$(git log --no-merges --format='%s%n%b' "${RANGE}")
if grep -qE '^[a-z]+(\([^)]*\))?!:|^BREAKING[ -]CHANGE:' <<< "${MESSAGES}"; then
    BUMP="major"
    NEW_VERSION="$((MAJOR + 1)).0.0"
elif grep -qE '^feat(\([^)]*\))?:' <<< "${MESSAGES}"; then
    BUMP="minor"
    NEW_VERSION="${MAJOR}.$((MINOR + 1)).0"
else
    BUMP="patch"
    NEW_VERSION="${MAJOR}.${MINOR}.$((PATCH + 1))"
fi

printf 'Current version: %s\n\n' "${OLD_TAG}"
printf 'Commits since %s:\n%s\n\n' "${OLD_TAG}" "$(sed 's/^/  /' <<< "${COMMITS}")"
printf 'Suggested version: v%s (%s)\n\n' "${NEW_VERSION}" "${BUMP}"

read -r -p "Release v${NEW_VERSION}? [Y/n/<other version>] " ANSWER
case "${ANSWER}" in
    ""|[Yy]|[Yy][Ee][Ss]) ;;
    [Nn]|[Nn][Oo]) echo "aborted"; exit 0 ;;
    *) NEW_VERSION="${ANSWER#v}" ;;
esac

[[ "${NEW_VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "invalid version '${NEW_VERSION}' (expected X.Y.Z)"

[[ "$(printf '%s\n' "${OLD_TAG#v}" "${NEW_VERSION}" | sort -V | tail -n 1)" == "${NEW_VERSION}" \
    && "${OLD_TAG#v}" != "${NEW_VERSION}" ]] || die "v${NEW_VERSION} is not newer than ${OLD_TAG}"

NEW_TAG="v${NEW_VERSION}"
git rev-parse -q --verify "refs/tags/${NEW_TAG}" > /dev/null && die "tag ${NEW_TAG} already exists"

# create annotated tag with the commit list as message and push it
git tag -a "${NEW_TAG}" -F - <<EOL
${NEW_TAG}

${COMMITS}
EOL
git push "${REMOTE}" "${NEW_TAG}"

printf '\nPushed %s, release workflow: https://github.com/mehu-ukko/cluster-smi/actions\n' "${NEW_TAG}"
