#!/bin/bash
# Polling pattern: Filesystem polling with checkpoint
#
# Usage: ./polling-fs.sh <storage-path>
#
# This script demonstrates polling a local filesystem storage path for new
# Quarry runs. It maintains a checkpoint file to track processed runs and
# avoid reprocessing.
#
# Run this script on a schedule (cron, systemd timer, etc.) to periodically
# check for new runs.

set -euo pipefail

STORAGE_PATH="${1:?Usage: $0 <storage-path>}"
CHECKPOINT_FILE="${STORAGE_PATH}/.quarry-checkpoint"

# Initialize checkpoint if it doesn't exist
if [[ ! -f "${CHECKPOINT_FILE}" ]]; then
  echo '{"last_poll":"1970-01-01T00:00:00Z","processed_runs":[]}' > "${CHECKPOINT_FILE}"
fi

# Read current checkpoint
CHECKPOINT=$(cat "${CHECKPOINT_FILE}")
LAST_POLL=$(echo "${CHECKPOINT}" | jq -r '.last_poll')

echo "Polling for new runs since: ${LAST_POLL}"
echo "Storage path: ${STORAGE_PATH}"

# Find run directories with completed runs
# Look for run_complete event files newer than checkpoint
PROCESSED_COUNT=0

# Find all run_id directories
for source_dir in "${STORAGE_PATH}"/source=*; do
  [[ -d "${source_dir}" ]] || continue

  for category_dir in "${source_dir}"/category=*; do
    [[ -d "${category_dir}" ]] || continue

    for day_dir in "${category_dir}"/day=*; do
      [[ -d "${day_dir}" ]] || continue

      for run_dir in "${day_dir}"/run_id=*; do
        [[ -d "${run_dir}" ]] || continue

        # Extract run_id from directory name
        RUN_ID=$(basename "${run_dir}" | sed 's/run_id=//')

        # Check if this run has a complete event
        COMPLETE_FILE="${run_dir}/event_type=run_complete/data.jsonl"
        if [[ ! -f "${COMPLETE_FILE}" ]]; then
          continue
        fi

        # Check if already processed
        if echo "${CHECKPOINT}" | jq -e ".processed_runs | index(\"${RUN_ID}\")" > /dev/null 2>&1; then
          continue
        fi

        # Process this run
        echo "Processing run: ${RUN_ID}"
        echo "  Path: ${run_dir}"

        # Count items in the run
        ITEMS_FILE="${run_dir}/event_type=item/data.jsonl"
        if [[ -f "${ITEMS_FILE}" ]]; then
          ITEM_COUNT=$(wc -l < "${ITEMS_FILE}")
          echo "  Items: ${ITEM_COUNT}"
        fi

        # Add your processing logic here:
        # - Read events from ${run_dir}/event_type=*/data.jsonl
        # - Transform and load to target system
        # - Handle errors appropriately

        # Mark as processed (add to checkpoint)
        CHECKPOINT=$(echo "${CHECKPOINT}" | jq ".processed_runs += [\"${RUN_ID}\"]")
        PROCESSED_COUNT=$((PROCESSED_COUNT + 1))

      done
    done
  done
done

# Update checkpoint with current timestamp
CURRENT_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
CHECKPOINT=$(echo "${CHECKPOINT}" | jq ".last_poll = \"${CURRENT_TIME}\"")

# Write updated checkpoint
echo "${CHECKPOINT}" > "${CHECKPOINT_FILE}"

echo ""
echo "Polling complete:"
echo "  Processed: ${PROCESSED_COUNT} new runs"
echo "  Checkpoint updated: ${CURRENT_TIME}"
