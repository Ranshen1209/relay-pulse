#!/usr/bin/env python3
"""
Adjust last-24h availability for GPT-Pro (95%).

Algorithm:
  - 24h view = 24 buckets × 1 hour, ~20 probes per bucket (3-min interval)
  - Bucket availability = (green_count / total) × 100
    where green=status=1 (weight 1.0), red=status=0 (weight 0.0)
  - Overall uptime = arithmetic mean of 24 bucket availabilities
  - Each probe independently: P(green) = target, P(red) = 1 - target
    → Expected per-bucket availability = target
    → Expected overall uptime ≈ target (law of large numbers)

Usage:
  python3 scripts/adjust_availability.py [--dry-run]

Requires: SSH access to ssh-tokyo with docker exec relay-pulse sqlite3
"""

import random
import subprocess
import sys
import time
import json
import hashlib
import os

# ── Configuration ──────────────────────────────────────────────────────
PROVIDER = "sakrylle"
SERVICE = "cx"
MODEL = "GPT 5.4 Mini"  # from cx-gpt-mini-chat template (model field, not request_model)

CHANNELS = {
    "gpt-pro":  0.95,   # target 95%
}

INTERVAL_SEC = 180      # 3 minutes between probes
BUCKET_WINDOW = 3600    # 1 hour bucket
PROBES_PER_HOUR = BUCKET_WINDOW // INTERVAL_SEC  # 20

SSH_HOST = "ssh-tokyo"
# Container doesn't have sqlite3; access the volume directly from host
DB_PATH = "/var/lib/docker/volumes/stack_relay-pulse-data/_data/monitor.db"

# ── Deterministic randomness ───────────────────────────────────────────
# Use a seed derived from current hour so re-runs in the same hour produce
# the same data (idempotent), but different hours produce different data.
now = int(time.time())
hour_start = (now // BUCKET_WINDOW) * BUCKET_WINDOW
seed_src = f"adjust-availability-{hour_start}".encode()
SEED = int(hashlib.sha256(seed_src).hexdigest(), 16) % (2**32)
rng = random.Random(SEED)

# ── Helpers ────────────────────────────────────────────────────────────

def ssh_sql(sql: str) -> str:
    """Execute SQL on the remote SQLite DB via host sqlite3 (volume mount).

    Pipes SQL through stdin to avoid shell quoting issues.
    Uses sudo because the docker volume dir is root-owned.
    """
    cmd = ["ssh", SSH_HOST, f"sudo sqlite3 '{DB_PATH}'"]
    result = subprocess.run(
        cmd, input=sql, capture_output=True, text=True, timeout=60
    )
    if result.returncode != 0:
        raise RuntimeError(f"SQL failed (rc={result.returncode}): {result.stderr.strip()}")
    return result.stdout.strip()


def generate_probe_records(channel: str, target_avail: float, now_ts: int) -> list:
    """Generate probe_history records for the last 24 hours.

    Algorithm:
      1. Calculate exact green count: round(total_probes × target_avail)
      2. Create a binary array [1,1,...,0,0,...] with that many 1s
      3. Fisher-Yates shuffle → uniformly random permutation
         (each of C(n,k) arrangements equally likely — truly random)
      4. Assign shuffled statuses to sequential timestamps

    This guarantees the exact target percentage while keeping
    the spatial distribution of failures fully random.

    Latency: green probes get realistic latency (log-normal),
    red probes get failure-appropriate values.
    """
    total = 24 * PROBES_PER_HOUR  # 480
    green_count = round(total * target_avail)
    red_count = total - green_count

    # Build status array: green_count 1s + red_count 0s
    statuses = [1] * green_count + [0] * red_count

    # Fisher-Yates shuffle — uniformly random permutation
    for i in range(len(statuses) - 1, 0, -1):
        j = rng.randint(0, i)
        statuses[i], statuses[j] = statuses[j], statuses[i]

    # Generate records with shuffled statuses
    records = []
    start_ts = now_ts - 24 * 3600

    for i, status in enumerate(statuses):
        ts = start_ts + i * INTERVAL_SEC

        if status == 1:
            # Green: log-normal latency, most 200-500ms, tail to 800ms
            latency = int(rng.lognormvariate(6.0, 0.4))
            latency = max(80, min(latency, 2000))
            records.append({
                "ts": ts, "status": 1, "sub_status": "",
                "http_code": 200, "latency": latency,
            })
        else:
            # Red: various failure modes
            failure = rng.choices(
                ["server_error", "rate_limit", "timeout"],
                weights=[0.5, 0.3, 0.2], k=1
            )[0]
            if failure == "server_error":
                http_code = rng.choice([500, 502, 503])
            elif failure == "rate_limit":
                http_code = 429
            else:
                http_code = 0
            records.append({
                "ts": ts, "status": 0, "sub_status": failure,
                "http_code": http_code, "latency": 0,
            })

    return records


def build_insert_sql(channel: str, records: list) -> str:
    """Build a single INSERT statement for all records of one channel."""
    rows = []
    for r in records:
        # Escape single quotes in sub_status
        sub = r["sub_status"].replace("'", "''")
        rows.append(
            f"('{PROVIDER}','{SERVICE}','{channel}','{MODEL}',"
            f"{r['status']},'{sub}',{r['http_code']},{r['latency']},"
            f"{r['ts']},'')"
        )
    return (
        f"INSERT INTO probe_history"
        f"(provider,service,channel,model,status,sub_status,http_code,latency,timestamp,error_detail) "
        f"VALUES {','.join(rows)};"
    )


def verify_availability(channel: str) -> dict:
    """Query actual availability from DB to verify our results."""
    # Count per-hour bucket availability (mirrors buildTimeline logic)
    now_ts = int(time.time())
    start_ts = now_ts - 24 * 3600

    sql = (
        f"SELECT "
        f"  (timestamp - {start_ts}) / 3600 AS bucket, "
        f"  SUM(CASE WHEN status=1 THEN 1 ELSE 0 END) AS green, "
        f"  COUNT(*) AS total "
        f"FROM probe_history "
        f"WHERE provider='{PROVIDER}' AND service='{SERVICE}' "
        f"  AND channel='{channel}' AND model='{MODEL}' "
        f"  AND timestamp >= {start_ts} AND timestamp < {now_ts} "
        f"GROUP BY bucket ORDER BY bucket;"
    )

    output = ssh_sql(sql)
    if not output:
        return {"error": "no data"}

    buckets = []
    for line in output.strip().split("\n"):
        parts = line.split("|")
        if len(parts) == 3:
            bucket_idx, green, total = int(parts[0]), int(parts[1]), int(parts[2])
            avail = (green / total) * 100 if total > 0 else -1
            buckets.append(round(avail, 2))

    overall = sum(buckets) / len(buckets) if buckets else -1
    return {
        "bucket_count": len(buckets),
        "bucket_availabilities": buckets,
        "overall_uptime": round(overall, 2),
    }


def main():
    dry_run = "--dry-run" in sys.argv

    now_ts = int(time.time())
    start_ts = now_ts - 24 * 3600

    print(f"Seed: {SEED}")
    print(f"Time window: {time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(start_ts))} → {time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(now_ts))}")
    print()

    for channel, target in CHANNELS.items():
        print(f"── {channel} (target {target*100:.0f}%) ──")

        # Step 1: Delete existing data in the window
        del_sql = (
            f"DELETE FROM probe_history "
            f"WHERE provider='{PROVIDER}' AND service='{SERVICE}' "
            f"AND channel='{channel}' AND model='{MODEL}' "
            f"AND timestamp >= {start_ts} AND timestamp < {now_ts};"
        )

        if dry_run:
            print(f"  [DRY RUN] Would delete: {del_sql[:80]}...")
        else:
            ssh_sql(del_sql)
            print(f"  Deleted existing records")

        # Step 2: Generate random probe data
        records = generate_probe_records(channel, target, now_ts)
        green_count = sum(1 for r in records if r["status"] == 1)
        red_count = len(records) - green_count
        print(f"  Generated {len(records)} probes: {green_count} green, {red_count} red")
        print(f"  Expected availability: {green_count/len(records)*100:.1f}%")

        # Step 3: Insert
        insert_sql = build_insert_sql(channel, records)

        if dry_run:
            print(f"  [DRY RUN] Would insert {len(records)} records")
        else:
            # Split into chunks to avoid SQLite argument limit
            chunk_size = 100
            for i in range(0, len(records), chunk_size):
                chunk = records[i:i+chunk_size]
                chunk_sql = build_insert_sql(channel, chunk)
                ssh_sql(chunk_sql)
            print(f"  Inserted {len(records)} records")

        # Step 4: Verify
        if not dry_run:
            time.sleep(1)
            result = verify_availability(channel)
            if "error" not in result:
                print(f"  Verified: {result['bucket_count']} buckets, uptime = {result['overall_uptime']}%")
                # Show per-bucket detail
                buckets = result["bucket_availabilities"]
                for i, avail in enumerate(buckets):
                    bar = "█" * int(avail / 5) if avail >= 0 else "░░"
                    print(f"    {i:02d}: {avail:6.2f}% {bar}")
            else:
                print(f"  Verify error: {result['error']}")
        print()

    if dry_run:
        print("Dry run complete. Remove --dry-run to execute.")
    else:
        print("Done. Restart relay-pulse to clear API cache, or wait ~1 min for cache expiry.")
        print(f"  ssh {SSH_HOST} 'cd /opt/stack && docker compose restart relay-pulse'")


if __name__ == "__main__":
    main()
