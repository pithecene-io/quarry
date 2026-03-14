#!/usr/bin/env bash
# Bump lockstep version across all components.
# Usage: scripts/version-bump.sh <new-version>
#
# Targets updated:
#   quarry/types/version.go, sdk/package.json, sdk/src/types/events.ts,
#   sdk/test/emit/06-golden/*.json, quarry/adapter/redisstream/*_test.go,
#   docs/CLI_PARITY.json, docs/guides/container.md,
#   README.md, PUBLIC_API.md, SUPPORT.md, docs/IMPLEMENTATION_PLAN.md
#
# Does NOT commit, edit CHANGELOG, or rebuild artifacts.
# The Taskfile task (version:bump) calls this then rebuilds.

set -euo pipefail

# Portable in-place sed (GNU vs BSD)
if sed --version 2>/dev/null | grep -q 'GNU'; then
  sedi() { sed -i "$@"; }
else
  sedi() { sed -i '' "$@"; }
fi

NEW="${1:?Usage: version-bump.sh <new-version>}"
OLD=$(sed -n 's/^const Version = "\([^"]*\)".*/\1/p' quarry/types/version.go)

echo "🔖 Bumping $OLD → $NEW"
echo ""

# Go canonical version
sedi "s/const Version = \"${OLD}\"/const Version = \"${NEW}\"/" \
  quarry/types/version.go
echo "  ✅ quarry/types/version.go"

# SDK package.json
sedi "s/\"version\": \"${OLD}\"/\"version\": \"${NEW}\"/" sdk/package.json
echo "  ✅ sdk/package.json"

# CONTRACT_VERSION in events.ts
sedi "s/export const CONTRACT_VERSION = '${OLD}'/export const CONTRACT_VERSION = '${NEW}'/" \
  sdk/src/types/events.ts
echo "  ✅ sdk/src/types/events.ts"

# Golden test fixtures
for f in sdk/test/emit/06-golden/*.json; do
  sedi "s/\"contract_version\": \"${OLD}\"/\"contract_version\": \"${NEW}\"/g" "$f"
  echo "  ✅ $f"
done

# Go test fixtures (redisstream)
for f in quarry/adapter/redisstream/*_test.go; do
  sedi "s/ContractVersion: \"${OLD}\"/ContractVersion: \"${NEW}\"/g" "$f"
  echo "  ✅ $f"
done

# CLI parity artifact
sedi "s/\"version\": \"${OLD}\"/\"version\": \"${NEW}\"/" docs/CLI_PARITY.json
echo "  ✅ docs/CLI_PARITY.json"

# Container guide image tags
sedi "s/quarry:${OLD}/quarry:${NEW}/g" docs/guides/container.md
sedi "s/quarry:${OLD}-slim/quarry:${NEW}-slim/g" docs/guides/container.md
echo "  ✅ docs/guides/container.md"

# Public-facing docs (install commands, image tags, version refs)
for f in README.md PUBLIC_API.md SUPPORT.md docs/IMPLEMENTATION_PLAN.md; do
  sedi "s/${OLD}/${NEW}/g" "$f"
  echo "  ✅ $f"
done
