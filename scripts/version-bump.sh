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

NEW="${1:?Usage: version-bump.sh <new-version>}"
OLD=$(grep -oP 'const Version = "\K[^"]+' quarry/types/version.go)

echo "🔖 Bumping $OLD → $NEW"
echo ""

# Go canonical version
sed -i "s/const Version = \"${OLD}\"/const Version = \"${NEW}\"/" \
  quarry/types/version.go
echo "  ✅ quarry/types/version.go"

# SDK package.json
sed -i "s/\"version\": \"${OLD}\"/\"version\": \"${NEW}\"/" sdk/package.json
echo "  ✅ sdk/package.json"

# CONTRACT_VERSION in events.ts
sed -i "s/export const CONTRACT_VERSION = '${OLD}'/export const CONTRACT_VERSION = '${NEW}'/" \
  sdk/src/types/events.ts
echo "  ✅ sdk/src/types/events.ts"

# Golden test fixtures
for f in sdk/test/emit/06-golden/*.json; do
  sed -i "s/\"contract_version\": \"${OLD}\"/\"contract_version\": \"${NEW}\"/g" "$f"
  echo "  ✅ $f"
done

# Go test fixtures (redisstream)
for f in quarry/adapter/redisstream/*_test.go; do
  sed -i "s/ContractVersion: \"${OLD}\"/ContractVersion: \"${NEW}\"/g" "$f"
  echo "  ✅ $f"
done

# CLI parity artifact
sed -i "s/\"version\": \"${OLD}\"/\"version\": \"${NEW}\"/" docs/CLI_PARITY.json
echo "  ✅ docs/CLI_PARITY.json"

# Container guide image tags
sed -i "s/quarry:${OLD}/quarry:${NEW}/g" docs/guides/container.md
sed -i "s/quarry:${OLD}-slim/quarry:${NEW}-slim/g" docs/guides/container.md
echo "  ✅ docs/guides/container.md"

# Public-facing docs (install commands, image tags, version refs)
for f in README.md PUBLIC_API.md SUPPORT.md docs/IMPLEMENTATION_PLAN.md; do
  sed -i "s/${OLD}/${NEW}/g" "$f"
  echo "  ✅ $f"
done
