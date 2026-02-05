#!/usr/bin/env python3
"""
Polling pattern: S3 polling with timestamp filtering.

Usage: python polling-s3.py <bucket> <prefix>

This script demonstrates polling an S3 bucket for new Quarry runs.
It uses object modification timestamps to detect new runs and maintains
a checkpoint file to track processed runs.

Prerequisites:
- boto3 installed (pip install boto3)
- AWS credentials configured

Run this script on a schedule to periodically check for new runs.
"""

import argparse
import json
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path

# boto3 is optional - script provides conceptual example without it
try:
    import boto3
    HAS_BOTO3 = True
except ImportError:
    HAS_BOTO3 = False


def load_checkpoint(checkpoint_path: Path) -> dict:
    """Load checkpoint from file, creating default if missing."""
    if checkpoint_path.exists():
        return json.loads(checkpoint_path.read_text())

    return {
        "last_poll": "1970-01-01T00:00:00Z",
        "processed_runs": [],
    }


def save_checkpoint(checkpoint_path: Path, checkpoint: dict) -> None:
    """Save checkpoint to file."""
    checkpoint_path.write_text(json.dumps(checkpoint, indent=2))


def parse_run_id_from_key(key: str) -> str | None:
    """Extract run_id from S3 key like 'source=.../run_id=xyz/...'"""
    for part in key.split("/"):
        if part.startswith("run_id="):
            return part[7:]  # Remove 'run_id=' prefix
    return None


def poll_s3(bucket: str, prefix: str, checkpoint: dict, lookback_hours: int = 1) -> list[dict]:
    """
    Poll S3 for new runs.

    Returns list of run info dicts for new, unprocessed runs.
    """
    if not HAS_BOTO3:
        print("boto3 not installed - returning simulated results")
        return [
            {
                "run_id": "simulated-run-001",
                "key": f"{prefix}/source=demo/day=2026-02-04/run_id=simulated-run-001/",
                "last_modified": datetime.now(timezone.utc).isoformat(),
            }
        ]

    s3 = boto3.client("s3")
    processed_runs = set(checkpoint.get("processed_runs", []))
    lookback_time = datetime.now(timezone.utc) - timedelta(hours=lookback_hours)

    new_runs = []
    seen_run_ids = set()

    # List objects with the given prefix
    paginator = s3.get_paginator("list_objects_v2")
    for page in paginator.paginate(Bucket=bucket, Prefix=prefix):
        for obj in page.get("Contents", []):
            key = obj["Key"]
            last_modified = obj["LastModified"]

            # Skip old objects
            if last_modified < lookback_time:
                continue

            # Look for run_complete event files
            if "event_type=run_complete" not in key:
                continue

            # Extract run_id
            run_id = parse_run_id_from_key(key)
            if not run_id:
                continue

            # Skip already processed
            if run_id in processed_runs:
                continue

            # Skip duplicates within this poll
            if run_id in seen_run_ids:
                continue

            seen_run_ids.add(run_id)
            new_runs.append({
                "run_id": run_id,
                "key": key,
                "last_modified": last_modified.isoformat(),
            })

    return new_runs


def process_run(bucket: str, run_info: dict) -> None:
    """
    Process a single run.

    Replace this with your actual ETL logic:
    - Read events from S3
    - Transform data
    - Load to target system
    """
    print(f"  Processing run: {run_info['run_id']}")
    print(f"    Key: {run_info['key']}")
    print(f"    Modified: {run_info['last_modified']}")

    # In production, you would:
    # 1. List all event files in the run directory
    # 2. Read and parse each JSONL file
    # 3. Transform the events
    # 4. Load to your target system

    # Example: Read items
    # items_key = run_info['key'].replace('event_type=run_complete', 'event_type=item')
    # response = s3.get_object(Bucket=bucket, Key=items_key)
    # for line in response['Body'].iter_lines():
    #     item = json.loads(line)
    #     transform_and_load(item)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Poll S3 for new Quarry runs",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python polling-s3.py my-bucket quarry-data/source=demo
  python polling-s3.py my-bucket quarry-data --lookback 24
        """,
    )
    parser.add_argument("bucket", help="S3 bucket name")
    parser.add_argument("prefix", help="S3 prefix to poll (e.g., 'quarry-data/source=demo')")
    parser.add_argument(
        "--lookback",
        type=int,
        default=1,
        help="Hours to look back for modified objects (default: 1)",
    )
    parser.add_argument(
        "--checkpoint",
        type=Path,
        default=Path(".quarry-s3-checkpoint.json"),
        help="Checkpoint file path (default: .quarry-s3-checkpoint.json)",
    )
    args = parser.parse_args()

    print(f"Polling S3 for new runs")
    print(f"  Bucket: {args.bucket}")
    print(f"  Prefix: {args.prefix}")
    print(f"  Lookback: {args.lookback} hours")
    print()

    # Load checkpoint
    checkpoint = load_checkpoint(args.checkpoint)
    print(f"Last poll: {checkpoint['last_poll']}")
    print(f"Previously processed: {len(checkpoint['processed_runs'])} runs")
    print()

    # Poll for new runs
    new_runs = poll_s3(args.bucket, args.prefix, checkpoint, args.lookback)
    print(f"Found {len(new_runs)} new runs to process")
    print()

    # Process each new run
    processed_count = 0
    for run_info in new_runs:
        try:
            process_run(args.bucket, run_info)
            checkpoint["processed_runs"].append(run_info["run_id"])
            processed_count += 1
        except Exception as e:
            print(f"  ERROR processing {run_info['run_id']}: {e}")
            # In production: log error, continue or abort based on policy

    # Update checkpoint
    checkpoint["last_poll"] = datetime.now(timezone.utc).isoformat()
    save_checkpoint(args.checkpoint, checkpoint)

    print()
    print(f"Polling complete:")
    print(f"  Processed: {processed_count} runs")
    print(f"  Checkpoint updated: {checkpoint['last_poll']}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
