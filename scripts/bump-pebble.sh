#!/usr/bin/env bash

# This script may be used to produce a branch bumping the Pebble
# version. The storage team bumps CockroachDB's Pebble dependency
# frequently, and this script automates some of that work.
#
# To bump the Pebble dependency to the corresponding pebble branch HEAD, run:
#
#   ./scripts/bump-pebble.sh
#
# Note that this script has different behaviour based on the branch on which it
# is run.

set -euo pipefail

echoerr() { printf "%s\n" "$*" >&2; }
pushd() { builtin pushd "$@" > /dev/null; }
popd() { builtin popd "$@" > /dev/null; }

BRANCH=release-23.2
PEBBLE_BRANCH=crl-release-23.2

# The script can only be run from a specific branch.
if [ "$(git rev-parse --abbrev-ref HEAD)" != "$BRANCH" ]; then
  echo "This script must be run from the $BRANCH branch."
  exit 1
fi

COCKROACH_DIR="$(go env GOPATH)/src/github.com/cockroachdb/cockroach"
UPSTREAM="upstream"

# Using count since error code is ignored
MATCHING_UPSTREAMS=$(git remote | grep -c ${UPSTREAM} || true )
if [ $MATCHING_UPSTREAMS = 0 ]; then
  echo This script expects the upstream to point to github.com/cockroachdb/cockroach.
  echo However no remote matches \"$UPSTREAM\".
  echo The available remotes are:
  git remote
  read -p "Please enter a remote to use: " UPSTREAM
fi

# Make sure that the cockroachdb remotes match what we expect. The
# `upstream` remote must point to github.com/cockroachdb/cockroach.
pushd "$COCKROACH_DIR"
git submodule update --init --recursive
popd

COCKROACH_UPSTREAM_URL="https://github.com/cockroachdb/cockroach.git"
PEBBLE_UPSTREAM_URL="https://github.com/cockroachdb/pebble.git"

# Ensure the local CockroachDB release branch is up-to-date with
# upstream and grab the current Pebble SHA.
pushd "$COCKROACH_DIR"
git fetch "$COCKROACH_UPSTREAM_URL" "$BRANCH"
git checkout "$BRANCH"
git rebase "$UPSTREAM/$BRANCH"
OLD_SHA=$(grep 'github.com/cockroachdb/pebble' go.mod | grep -o -E '[a-f0-9]{12}$')
popd

# Ensure the local Pebble release branch is up-to-date with upstream,
# and grab the desired Pebble SHA.
PEBBLE_DIR=$(mktemp -d)
trap "rm -rf $PEBBLE_DIR" EXIT

git clone --no-checkout "$PEBBLE_UPSTREAM_URL" "$PEBBLE_DIR"

pushd "$PEBBLE_DIR"
NEW_SHA="${1-}"
if [ -z "$NEW_SHA" ]; then
  NEW_SHA=$(git rev-parse "origin/$PEBBLE_BRANCH")
  echo "Using latest pebble $PEBBLE_BRANCH sha $NEW_SHA."
else
  # Verify that the given commit is in the correct pebble branch.
  if ! git merge-base --is-ancestor $NEW_SHA "origin/$PEBBLE_BRANCH"; then
    echo "Error: $NEW_SHA is not an ancestor of the pebble branch $PEBBLE_BRANCH" >&2
    exit 1
  fi
  echo "Using provided sha $NEW_SHA."
fi

COMMITS=$(git log --pretty='format:%h %s' "$OLD_SHA..$NEW_SHA" |
          grep -v 'Merge pull request' |
          sed 's#^#https://github.com/cockroachdb/pebble/commit/#')
echo "$COMMITS"
popd

COCKROACH_BRANCH="$USER/pebble-${BRANCH}-${NEW_SHA:0:12}"

# Pull in the Pebble module at the desired SHA.
pushd "$COCKROACH_DIR"
./dev generate go
go get "github.com/cockroachdb/pebble@${NEW_SHA}"
go mod tidy
popd

# Create the branch and commit on the CockroachDB repository.
pushd "$COCKROACH_DIR"
./dev generate bazel --mirror
git add go.mod go.sum DEPS.bzl build/bazelutil/distdir_files.bzl
git branch -D "$COCKROACH_BRANCH" || true
git checkout -b "$COCKROACH_BRANCH"
git commit -m "go.mod: bump Pebble to ${NEW_SHA:0:12}

$COMMITS

Release note:
"
# Open an editor to allow the user to set the release note.
git commit --amend
popd
