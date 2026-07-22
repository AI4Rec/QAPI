#!/usr/bin/env python3
"""QAPI pooled-capacity collector.

The collector is deliberately read-only toward New API and account state. It
consumes CLIProxyAPI's in-memory usage queue, joins request IDs to New API's
billing log, samples the upstream quota windows, and publishes a redacted JSON
snapshot for the macOS menu-bar client.
"""

from __future__ import annotations

import argparse
import concurrent.futures
import datetime as dt
import hashlib
import json
import math
import os
import re
import signal
import sqlite3
import statistics
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any, Iterable


SCHEMA_VERSION = 1
DEFAULT_MANAGEMENT_URL = "http://127.0.0.1:8317"
WHAM_USAGE_URL = "https://chatgpt.com/backend-api/wham/usage"


def utc_now() -> float:
    return time.time()


def iso_time(timestamp: float | None = None) -> str:
    value = dt.datetime.fromtimestamp(timestamp or utc_now(), dt.timezone.utc)
    return value.isoformat().replace("+00:00", "Z")


def parse_iso_timestamp(value: Any) -> float:
    if isinstance(value, (int, float)):
        return float(value)
    if not value:
        return utc_now()
    text = str(value).strip().replace("Z", "+00:00")
    try:
        return dt.datetime.fromisoformat(text).timestamp()
    except ValueError:
        return utc_now()


def safe_float(value: Any) -> float | None:
    try:
        result = float(value)
    except (TypeError, ValueError):
        return None
    return result if math.isfinite(result) else None


def atomic_json_write(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=path.name + ".", dir=str(path.parent))
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(payload, handle, ensure_ascii=False, separators=(",", ":"))
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.chmod(temporary, 0o644)
        os.replace(temporary, path)
    finally:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass


def read_management_secret(config_path: Path) -> str:
    text = config_path.read_text(encoding="utf-8", errors="replace")
    section = re.search(
        r"(?ms)^remote-management:\s*\n(?P<body>(?:^[ \t]+.*(?:\n|$))*)",
        text,
    )
    if not section:
        raise RuntimeError("remote-management section not found")
    match = re.search(
        r"(?m)^[ \t]+secret-key:\s*[\"']?([^\"'\n#]+)",
        section.group("body"),
    )
    if not match or not match.group(1).strip():
        raise RuntimeError("remote-management secret-key not found")
    return match.group(1).strip()


class ManagementClient:
    def __init__(self, base_url: str, secret: str, timeout: float = 8.0) -> None:
        self.base_url = base_url.rstrip("/")
        self.secret = secret
        self.timeout = timeout

    def request_json(self, path: str, query: dict[str, Any] | None = None) -> Any:
        url = self.base_url + path
        if query:
            url += "?" + urllib.parse.urlencode(query)
        request = urllib.request.Request(
            url,
            headers={
                "Authorization": "Bearer " + self.secret,
                "Accept": "application/json",
                "User-Agent": "qapi-capacity-collector/1",
            },
        )
        with urllib.request.urlopen(request, timeout=self.timeout) as response:
            return json.load(response)

    def list_auth_files(self) -> list[dict[str, Any]]:
        payload = self.request_json("/v0/management/auth-files")
        files = payload.get("files", []) if isinstance(payload, dict) else []
        return [entry for entry in files if isinstance(entry, dict)]

    def download_auth(self, name: str) -> dict[str, Any]:
        payload = self.request_json(
            "/v0/management/auth-files/download", {"name": name}
        )
        if not isinstance(payload, dict):
            raise RuntimeError("invalid auth payload")
        return payload

    def pop_usage(self, count: int = 500) -> list[dict[str, Any]]:
        payload = self.request_json("/v0/management/usage-queue", {"count": count})
        if not isinstance(payload, list):
            return []
        return [entry for entry in payload if isinstance(entry, dict)]


def fetch_wham_usage(auth_payload: dict[str, Any], timeout: float = 15.0) -> dict[str, Any]:
    access_token = str(auth_payload.get("access_token") or "").strip()
    account_id = str(
        auth_payload.get("account_id")
        or auth_payload.get("chatgpt_account_id")
        or ""
    ).strip()
    if not access_token:
        raise RuntimeError("credential has no access token")
    request = urllib.request.Request(
        WHAM_USAGE_URL,
        headers={
            "Authorization": "Bearer " + access_token,
            "ChatGPT-Account-Id": account_id,
            "Accept": "application/json",
            "User-Agent": "qapi-capacity-collector/1",
        },
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        payload = json.load(response)
    if not isinstance(payload, dict):
        raise RuntimeError("invalid quota response")
    return payload


class MonitorDatabase:
    def __init__(self, path: Path) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        self.path = path
        self.connection = sqlite3.connect(str(path), timeout=5)
        self.connection.row_factory = sqlite3.Row
        self.connection.execute("PRAGMA journal_mode=WAL")
        self.connection.execute("PRAGMA synchronous=NORMAL")
        self.connection.execute("PRAGMA busy_timeout=5000")
        self._initialize()

    def _initialize(self) -> None:
        self.connection.executescript(
            """
            CREATE TABLE IF NOT EXISTS accounts (
                auth_index TEXT PRIMARY KEY,
                alias TEXT NOT NULL UNIQUE,
                provider TEXT NOT NULL DEFAULT '',
                status TEXT NOT NULL DEFAULT 'unknown',
                disabled INTEGER NOT NULL DEFAULT 0,
                unavailable INTEGER NOT NULL DEFAULT 0,
                plan_type TEXT NOT NULL DEFAULT '',
                success_count INTEGER NOT NULL DEFAULT 0,
                failed_count INTEGER NOT NULL DEFAULT 0,
                last_seen REAL NOT NULL,
                last_quota_ok REAL
            );

            CREATE TABLE IF NOT EXISTS usage_events (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                request_id TEXT NOT NULL,
                auth_index TEXT NOT NULL,
                event_ts REAL NOT NULL,
                model TEXT NOT NULL DEFAULT '',
                input_tokens INTEGER NOT NULL DEFAULT 0,
                output_tokens INTEGER NOT NULL DEFAULT 0,
                reasoning_tokens INTEGER NOT NULL DEFAULT 0,
                cached_tokens INTEGER NOT NULL DEFAULT 0,
                total_tokens INTEGER NOT NULL DEFAULT 0,
                failed INTEGER NOT NULL DEFAULT 0,
                status_code INTEGER NOT NULL DEFAULT 0,
                qapi_quota REAL,
                joined INTEGER NOT NULL DEFAULT 0,
                UNIQUE(request_id, auth_index, event_ts, failed)
            );
            CREATE INDEX IF NOT EXISTS idx_usage_pending
                ON usage_events(joined, request_id);
            CREATE INDEX IF NOT EXISTS idx_usage_auth_time
                ON usage_events(auth_index, event_ts);

            CREATE TABLE IF NOT EXISTS new_api_log_joins (
                log_id INTEGER PRIMARY KEY,
                usage_event_id INTEGER NOT NULL UNIQUE,
                joined_at REAL NOT NULL
            );

            CREATE TABLE IF NOT EXISTS account_output_events (
                usage_event_id INTEGER PRIMARY KEY,
                auth_index TEXT NOT NULL,
                output_quota REAL NOT NULL,
                attributed_at REAL NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_account_output_auth
                ON account_output_events(auth_index);

            INSERT OR IGNORE INTO account_output_events(
                usage_event_id, auth_index, output_quota, attributed_at
            )
            SELECT id, auth_index, qapi_quota, strftime('%s','now')
            FROM usage_events
            WHERE joined=1 AND failed=0 AND qapi_quota IS NOT NULL;

            CREATE TABLE IF NOT EXISTS quota_samples (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                auth_index TEXT NOT NULL,
                sampled_at REAL NOT NULL,
                window_seconds INTEGER NOT NULL,
                used_percent REAL NOT NULL,
                reset_at REAL,
                allowed INTEGER NOT NULL DEFAULT 1,
                limit_reached INTEGER NOT NULL DEFAULT 0,
                plan_type TEXT NOT NULL DEFAULT '',
                calibrated INTEGER NOT NULL DEFAULT 0,
                UNIQUE(auth_index, sampled_at, window_seconds)
            );
            CREATE INDEX IF NOT EXISTS idx_quota_auth_window
                ON quota_samples(auth_index, window_seconds, sampled_at);

            CREATE TABLE IF NOT EXISTS capacity_observations (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                auth_index TEXT NOT NULL,
                window_seconds INTEGER NOT NULL,
                sample_id INTEGER NOT NULL UNIQUE,
                units_per_percent REAL NOT NULL,
                delta_percent REAL NOT NULL,
                quota_units REAL NOT NULL,
                created_at REAL NOT NULL
            );

            CREATE TABLE IF NOT EXISTS capacity_models (
                auth_index TEXT NOT NULL,
                window_seconds INTEGER NOT NULL,
                units_per_percent REAL NOT NULL,
                sample_count INTEGER NOT NULL,
                confidence REAL NOT NULL,
                dispersion REAL NOT NULL,
                updated_at REAL NOT NULL,
                PRIMARY KEY(auth_index, window_seconds)
            );

            CREATE TABLE IF NOT EXISTS pool_snapshots (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                generated_at REAL NOT NULL,
                remaining_percent REAL,
                remaining_quota REAL,
                full_capacity REAL,
                confidence REAL NOT NULL,
                active_count INTEGER NOT NULL,
                disabled_count INTEGER NOT NULL,
                join_rate REAL NOT NULL,
                status TEXT NOT NULL
            );
            """
        )
        self.connection.commit()

    def _next_alias(self) -> str:
        rows = self.connection.execute("SELECT alias FROM accounts").fetchall()
        maximum = 0
        for row in rows:
            match = re.fullmatch(r"A(\d+)", row["alias"] or "")
            if match:
                maximum = max(maximum, int(match.group(1)))
        return f"A{maximum + 1}"

    def update_accounts(self, auth_entries: Iterable[dict[str, Any]]) -> None:
        now = utc_now()
        for entry in auth_entries:
            auth_index = str(entry.get("auth_index") or "").strip()
            if not auth_index:
                continue
            existing = self.connection.execute(
                "SELECT alias FROM accounts WHERE auth_index=?", (auth_index,)
            ).fetchone()
            alias = existing["alias"] if existing else self._next_alias()
            self.connection.execute(
                """
                INSERT INTO accounts(
                    auth_index, alias, provider, status, disabled, unavailable,
                    success_count, failed_count, last_seen
                ) VALUES(?,?,?,?,?,?,?,?,?)
                ON CONFLICT(auth_index) DO UPDATE SET
                    provider=excluded.provider,
                    status=excluded.status,
                    disabled=excluded.disabled,
                    unavailable=excluded.unavailable,
                    success_count=excluded.success_count,
                    failed_count=excluded.failed_count,
                    last_seen=excluded.last_seen
                """,
                (
                    auth_index,
                    alias,
                    str(entry.get("provider") or entry.get("type") or ""),
                    str(entry.get("status") or "unknown"),
                    int(bool(entry.get("disabled"))),
                    int(bool(entry.get("unavailable"))),
                    int(entry.get("success") or 0),
                    int(entry.get("failed") or 0),
                    now,
                ),
            )
        self.connection.commit()

    def insert_usage_records(self, records: Iterable[dict[str, Any]]) -> int:
        inserted = 0
        for record in records:
            request_id = str(record.get("request_id") or "").strip()
            auth_index = str(record.get("auth_index") or "").strip()
            if not request_id or not auth_index:
                continue
            tokens = record.get("tokens") if isinstance(record.get("tokens"), dict) else {}
            fail = record.get("fail") if isinstance(record.get("fail"), dict) else {}
            values = (
                request_id,
                auth_index,
                parse_iso_timestamp(record.get("timestamp")),
                str(record.get("model") or ""),
                int(tokens.get("input_tokens") or 0),
                int(tokens.get("output_tokens") or 0),
                int(tokens.get("reasoning_tokens") or 0),
                int(tokens.get("cached_tokens") or tokens.get("cache_read_tokens") or 0),
                int(tokens.get("total_tokens") or 0),
                int(bool(record.get("failed"))),
                int(fail.get("status_code") or 0),
            )
            cursor = self.connection.execute(
                """
                INSERT OR IGNORE INTO usage_events(
                    request_id, auth_index, event_ts, model,
                    input_tokens, output_tokens, reasoning_tokens,
                    cached_tokens, total_tokens, failed, status_code
                ) VALUES(?,?,?,?,?,?,?,?,?,?,?)
                """,
                values,
            )
            inserted += max(cursor.rowcount, 0)
        self.connection.commit()
        return inserted

    def join_new_api_logs(self, new_api_db: Path, limit: int = 500) -> tuple[int, int]:
        pending_rows = self.connection.execute(
            """
            SELECT *
            FROM usage_events
            WHERE joined=0 AND failed=0
            ORDER BY event_ts
            LIMIT ?
            """,
            (limit,),
        ).fetchall()
        if not pending_rows:
            return 0, 0
        source = sqlite3.connect(
            f"file:{new_api_db}?mode=ro", uri=True, timeout=2
        )
        source.row_factory = sqlite3.Row
        source.execute("PRAGMA busy_timeout=2000")
        matched = 0
        try:
            for event in pending_rows:
                request_id = event["request_id"]
                billing = source.execute(
                    """
                    SELECT id, quota FROM logs
                    WHERE request_id=?
                    ORDER BY id DESC LIMIT 1
                    """,
                    (request_id,),
                ).fetchone()
                if billing is None:
                    # New API and CLIProxy use different request-id namespaces in
                    # this deployment. Token counts are emitted by both sides and
                    # provide an exact, non-sensitive secondary join key.
                    candidates = source.execute(
                        """
                        SELECT id, quota, created_at FROM logs
                        WHERE model_name=?
                          AND prompt_tokens=?
                          AND completion_tokens=?
                          AND created_at BETWEEN ? AND ?
                        ORDER BY ABS(created_at-?) ASC, id ASC
                        LIMIT 8
                        """,
                        (
                            event["model"],
                            event["input_tokens"],
                            event["output_tokens"],
                            int(event["event_ts"]) - 5,
                            int(event["event_ts"]) + 120,
                            event["event_ts"],
                        ),
                    ).fetchall()
                    for candidate in candidates:
                        already_used = self.connection.execute(
                            "SELECT 1 FROM new_api_log_joins WHERE log_id=?",
                            (candidate["id"],),
                        ).fetchone()
                        if already_used is None:
                            billing = candidate
                            break
                if billing is None:
                    continue
                quota = float(billing["quota"] or 0)
                self.connection.execute(
                    "UPDATE usage_events SET qapi_quota=?, joined=1 WHERE id=?",
                    (quota, event["id"]),
                )
                self.connection.execute(
                    """
                    INSERT OR IGNORE INTO new_api_log_joins(log_id, usage_event_id, joined_at)
                    VALUES(?,?,?)
                    """,
                    (billing["id"], event["id"], utc_now()),
                )
                self.connection.execute(
                    """
                    INSERT OR IGNORE INTO account_output_events(
                        usage_event_id, auth_index, output_quota, attributed_at
                    ) VALUES(?,?,?,?)
                    """,
                    (event["id"], event["auth_index"], quota, utc_now()),
                )
                self.connection.execute(
                    """
                    UPDATE usage_events SET qapi_quota=COALESCE(qapi_quota,0), joined=1
                    WHERE request_id=? AND joined=0
                    """,
                    (request_id,),
                )
                matched += 1
        finally:
            source.close()
        self.connection.commit()
        return matched, len(pending_rows)

    def insert_quota_payload(
        self, auth_index: str, payload: dict[str, Any], sampled_at: float
    ) -> int:
        rate_limit = payload.get("rate_limit")
        if not isinstance(rate_limit, dict):
            rate_limit = payload.get("rate_limits")
        if not isinstance(rate_limit, dict):
            return 0
        plan_type = str(payload.get("plan_type") or "")
        self.connection.execute(
            "UPDATE accounts SET plan_type=?, last_quota_ok=? WHERE auth_index=?",
            (plan_type, sampled_at, auth_index),
        )
        inserted = 0
        for key in ("primary_window", "secondary_window"):
            window = rate_limit.get(key)
            if not isinstance(window, dict):
                continue
            seconds = int(window.get("limit_window_seconds") or 0)
            used = safe_float(window.get("used_percent"))
            if seconds <= 0 or used is None:
                continue
            cursor = self.connection.execute(
                """
                INSERT OR IGNORE INTO quota_samples(
                    auth_index, sampled_at, window_seconds, used_percent,
                    reset_at, allowed, limit_reached, plan_type
                ) VALUES(?,?,?,?,?,?,?,?)
                """,
                (
                    auth_index,
                    sampled_at,
                    seconds,
                    max(0.0, min(100.0, used)),
                    safe_float(window.get("reset_at")),
                    int(bool(rate_limit.get("allowed", True))),
                    int(bool(rate_limit.get("limit_reached", False))),
                    plan_type,
                ),
            )
            inserted += max(cursor.rowcount, 0)
        self.connection.commit()
        return inserted

    def calibrate_new_samples(self) -> int:
        samples = self.connection.execute(
            "SELECT * FROM quota_samples WHERE calibrated=0 ORDER BY sampled_at"
        ).fetchall()
        observations = 0
        for sample in samples:
            baseline = self.connection.execute(
                """
                SELECT * FROM quota_samples
                WHERE auth_index=? AND window_seconds=? AND sampled_at<?
                  AND ABS(COALESCE(reset_at,0)-COALESCE(?,0))<2
                  AND used_percent < ?
                ORDER BY sampled_at DESC LIMIT 1
                """,
                (
                    sample["auth_index"],
                    sample["window_seconds"],
                    sample["sampled_at"],
                    sample["reset_at"],
                    sample["used_percent"],
                ),
            ).fetchone()
            if baseline is not None:
                delta = float(sample["used_percent"] - baseline["used_percent"])
                quota_row = self.connection.execute(
                    """
                    SELECT COALESCE(SUM(qapi_quota),0) AS quota_units
                    FROM usage_events
                    WHERE auth_index=? AND joined=1 AND failed=0
                      AND event_ts>? AND event_ts<=?
                    """,
                    (
                        sample["auth_index"],
                        baseline["sampled_at"],
                        sample["sampled_at"],
                    ),
                ).fetchone()
                quota_units = float(quota_row["quota_units"] or 0)
                if 0.25 <= delta <= 60 and quota_units > 0:
                    units_per_percent = quota_units / delta
                    if math.isfinite(units_per_percent) and units_per_percent > 0:
                        self.connection.execute(
                            """
                            INSERT OR IGNORE INTO capacity_observations(
                                auth_index, window_seconds, sample_id,
                                units_per_percent, delta_percent, quota_units, created_at
                            ) VALUES(?,?,?,?,?,?,?)
                            """,
                            (
                                sample["auth_index"],
                                sample["window_seconds"],
                                sample["id"],
                                units_per_percent,
                                delta,
                                quota_units,
                                utc_now(),
                            ),
                        )
                        observations += 1
            self.connection.execute(
                "UPDATE quota_samples SET calibrated=1 WHERE id=?", (sample["id"],)
            )
        self.connection.commit()
        self.rebuild_capacity_models()
        return observations

    def rebuild_capacity_models(self) -> None:
        keys = self.connection.execute(
            "SELECT DISTINCT auth_index, window_seconds FROM capacity_observations"
        ).fetchall()
        for key in keys:
            rows = self.connection.execute(
                """
                SELECT units_per_percent FROM capacity_observations
                WHERE auth_index=? AND window_seconds=?
                ORDER BY created_at DESC LIMIT 20
                """,
                (key["auth_index"], key["window_seconds"]),
            ).fetchall()
            values = [float(row["units_per_percent"]) for row in rows]
            if not values:
                continue
            median = statistics.median(values)
            deviations = [abs(value - median) for value in values]
            mad = statistics.median(deviations) if deviations else 0.0
            dispersion = min(1.0, mad / median) if median > 0 else 1.0
            sample_factor = min(1.0, len(values) / 6.0)
            confidence = sample_factor * max(0.2, 1.0 - dispersion)
            self.connection.execute(
                """
                INSERT INTO capacity_models(
                    auth_index, window_seconds, units_per_percent,
                    sample_count, confidence, dispersion, updated_at
                ) VALUES(?,?,?,?,?,?,?)
                ON CONFLICT(auth_index, window_seconds) DO UPDATE SET
                    units_per_percent=excluded.units_per_percent,
                    sample_count=excluded.sample_count,
                    confidence=excluded.confidence,
                    dispersion=excluded.dispersion,
                    updated_at=excluded.updated_at
                """,
                (
                    key["auth_index"],
                    key["window_seconds"],
                    median,
                    len(values),
                    confidence,
                    dispersion,
                    utc_now(),
                ),
            )
        self.connection.commit()

    def _cohort_models(self) -> dict[tuple[str, int], tuple[float, float]]:
        rows = self.connection.execute(
            """
            SELECT a.plan_type, m.window_seconds, m.units_per_percent, m.confidence
            FROM capacity_models m JOIN accounts a USING(auth_index)
            WHERE m.units_per_percent>0
            """
        ).fetchall()
        grouped: dict[tuple[str, int], list[tuple[float, float]]] = defaultdict(list)
        for row in rows:
            grouped[(row["plan_type"] or "", row["window_seconds"])].append(
                (float(row["units_per_percent"]), float(row["confidence"]))
            )
        result: dict[tuple[str, int], tuple[float, float]] = {}
        for key, values in grouped.items():
            result[key] = (
                statistics.median(item[0] for item in values),
                min(0.45, statistics.median(item[1] for item in values) * 0.6),
            )
        return result

    def build_snapshot(self) -> dict[str, Any]:
        now = utc_now()
        # CLIProxy may temporarily mark a credential as "error" after its
        # short window is exhausted. It still belongs to the capacity pool if
        # the upstream quota endpoint remains readable; removing it would
        # shrink the denominator and could make the total bar rise at zero.
        valid_cutoff = now - 300
        active_rows = self.connection.execute(
            """
            SELECT * FROM accounts
            WHERE disabled=0 AND last_quota_ok>=?
            ORDER BY alias
            """,
            (valid_cutoff,),
        ).fetchall()
        disabled_count = self.connection.execute(
            "SELECT COUNT(*) FROM accounts WHERE disabled=1 OR status='disabled'"
        ).fetchone()[0]
        unknown_count = self.connection.execute(
            """
            SELECT COUNT(*) FROM accounts
            WHERE disabled=0 AND (last_quota_ok IS NULL OR last_quota_ok<?)
            """,
            (valid_cutoff,),
        ).fetchone()[0]
        model_rows = self.connection.execute("SELECT * FROM capacity_models").fetchall()
        models = {
            (row["auth_index"], row["window_seconds"]): row for row in model_rows
        }
        cohorts = self._cohort_models()
        total_remaining = 0.0
        total_full = 0.0
        weighted_confidence = 0.0
        governing_resets: list[float] = []
        constraint_counts: Counter[int] = Counter()
        account_details: list[dict[str, Any]] = []
        calibrated_accounts = 0
        freshest_sample = 0.0

        for account in active_rows:
            windows = self.connection.execute(
                """
                SELECT q.* FROM quota_samples q
                JOIN (
                    SELECT window_seconds, MAX(sampled_at) AS latest
                    FROM quota_samples WHERE auth_index=? GROUP BY window_seconds
                ) latest
                ON latest.window_seconds=q.window_seconds
                   AND latest.latest=q.sampled_at
                WHERE q.auth_index=?
                ORDER BY q.window_seconds
                """,
                (account["auth_index"], account["auth_index"]),
            ).fetchall()
            available_windows = []
            raw_windows = []
            for window in windows:
                freshest_sample = max(freshest_sample, float(window["sampled_at"]))
                raw_windows.append(
                    {
                        "window_seconds": int(window["window_seconds"]),
                        "remaining_percent": max(
                            0.0, 100.0 - float(window["used_percent"])
                        ),
                        "reset_at": window["reset_at"],
                    }
                )
                model = models.get((account["auth_index"], window["window_seconds"]))
                estimated_from_cohort = False
                if model is not None:
                    units = float(model["units_per_percent"])
                    confidence = float(model["confidence"])
                    sample_count = int(model["sample_count"])
                else:
                    cohort = cohorts.get(
                        (account["plan_type"] or "", window["window_seconds"])
                    )
                    if cohort is None:
                        continue
                    units, confidence = cohort
                    sample_count = 0
                    estimated_from_cohort = True
                remaining_percent = max(0.0, 100.0 - float(window["used_percent"]))
                available_windows.append(
                    {
                        "window_seconds": int(window["window_seconds"]),
                        "remaining_percent": remaining_percent,
                        "remaining_quota": units * remaining_percent,
                        "full_capacity": units * 100.0,
                        "reset_at": window["reset_at"],
                        "confidence": confidence,
                        "sample_count": sample_count,
                        "cohort": estimated_from_cohort,
                    }
                )

            detail: dict[str, Any] = {
                "alias": account["alias"],
                "status": account["status"],
                "windows": raw_windows,
                "calibrated": False,
            }
            if available_windows:
                governing = min(
                    available_windows, key=lambda item: item["remaining_quota"]
                )
                total_remaining += governing["remaining_quota"]
                total_full += governing["full_capacity"]
                weighted_confidence += (
                    governing["confidence"] * governing["full_capacity"]
                )
                constraint_counts[governing["window_seconds"]] += 1
                if governing["reset_at"]:
                    governing_resets.append(float(governing["reset_at"]))
                calibrated_accounts += 1
                detail.update(
                    {
                        "calibrated": True,
                        "governing_window_seconds": governing["window_seconds"],
                        "governing_remaining_percent": governing[
                            "remaining_percent"
                        ],
                        "governing_reset_at": governing["reset_at"],
                        "confidence": governing["confidence"],
                    }
                )
            account_details.append(detail)

        output_rows = self.connection.execute(
            """
            SELECT auth_index, COALESCE(SUM(output_quota),0) AS output_quota
            FROM account_output_events
            GROUP BY auth_index
            """
        ).fetchall()
        account_outputs = [
            {
                "asset_key": "cpa:"
                + hashlib.sha256(row["auth_index"].encode("utf-8")).hexdigest()[:16],
                "cumulative_output_quota": float(row["output_quota"] or 0),
            }
            for row in output_rows
        ]

        event_counts = self.connection.execute(
            """
            SELECT COUNT(*) total, SUM(CASE WHEN joined=1 THEN 1 ELSE 0 END) joined
            FROM usage_events WHERE event_ts>?
            """,
            (now - 3600,),
        ).fetchone()
        total_events = int(event_counts["total"] or 0)
        joined_events = int(event_counts["joined"] or 0)
        join_rate = joined_events / total_events if total_events else 0.0
        burn_row = self.connection.execute(
            """
            SELECT COALESCE(SUM(qapi_quota),0) quota
            FROM usage_events
            WHERE joined=1 AND failed=0 AND event_ts>?
            """,
            (now - 900,),
        ).fetchone()
        burn_per_minute = float(burn_row["quota"] or 0) / 15.0

        remaining_percent = None
        confidence = 0.0
        eta_seconds = None
        if total_full > 0:
            remaining_percent = max(0.0, min(100.0, total_remaining / total_full * 100))
            confidence = weighted_confidence / total_full
            confidence *= 0.5 + 0.5 * join_rate if total_events else 0.5
            if burn_per_minute > 0:
                eta_seconds = total_remaining / burn_per_minute * 60.0

        sample_age = None if freshest_sample <= 0 else max(0.0, now - freshest_sample)
        stale = sample_age is None or sample_age > 180
        coverage = calibrated_accounts / len(active_rows) if active_rows else 0.0
        if not active_rows:
            status = "critical"
        elif remaining_percent is None or confidence < 0.35 or coverage < 0.8:
            status = "estimating"
        elif stale:
            status = "stale"
        else:
            next_reset = min(governing_resets) if governing_resets else None
            lasts_to_reset = (
                eta_seconds is None
                or next_reset is None
                or now + eta_seconds >= next_reset
            )
            if remaining_percent <= 5 or (eta_seconds is not None and eta_seconds <= 3600):
                status = "critical"
            elif not lasts_to_reset or remaining_percent <= 25:
                status = "warning"
            else:
                status = "healthy"

        constraints = [
            {"window_seconds": seconds, "accounts": count}
            for seconds, count in sorted(constraint_counts.items())
        ]
        snapshot = {
            "schema_version": SCHEMA_VERSION,
            "generated_at": iso_time(now),
            "pool": {
                "remaining_percent": remaining_percent,
                "remaining_quota": total_remaining if total_full > 0 else None,
                "full_capacity": total_full if total_full > 0 else None,
                "confidence": confidence,
                "status": status,
                "estimated": True,
                "burn_rate_per_minute": burn_per_minute,
                "eta_seconds": eta_seconds,
                "next_governing_reset_at": min(governing_resets)
                if governing_resets
                else None,
            },
            "accounts": {
                "active": len(active_rows),
                "disabled": int(disabled_count),
                "unknown": int(unknown_count),
                "calibrated": calibrated_accounts,
            },
            "constraints": constraints,
            "account_details": account_details,
            "account_outputs": account_outputs,
            "data_quality": {
                "request_join_rate": join_rate,
                "events_last_hour": total_events,
                "quota_sample_age_seconds": sample_age,
                "capacity_coverage": coverage,
            },
        }
        self.connection.execute(
            """
            INSERT INTO pool_snapshots(
                generated_at, remaining_percent, remaining_quota, full_capacity,
                confidence, active_count, disabled_count, join_rate, status
            ) VALUES(?,?,?,?,?,?,?,?,?)
            """,
            (
                now,
                remaining_percent,
                total_remaining if total_full > 0 else None,
                total_full if total_full > 0 else None,
                confidence,
                len(active_rows),
                int(disabled_count),
                join_rate,
                status,
            ),
        )
        self.connection.commit()
        return snapshot

    def prune(self) -> None:
        now = utc_now()
        self.connection.execute("DELETE FROM usage_events WHERE event_ts<?", (now - 7 * 86400,))
        self.connection.execute(
            "DELETE FROM quota_samples WHERE sampled_at<?", (now - 30 * 86400,)
        )
        self.connection.execute(
            "DELETE FROM pool_snapshots WHERE generated_at<?", (now - 90 * 86400,)
        )
        self.connection.commit()


class Collector:
    def __init__(self, args: argparse.Namespace) -> None:
        secret = read_management_secret(args.management_config)
        self.management = ManagementClient(args.management_url, secret)
        self.database = MonitorDatabase(args.database)
        self.new_api_db = args.new_api_db
        self.snapshot_path = args.snapshot
        self.auth_entries: list[dict[str, Any]] = []
        self.stopping = False
        self.last_auth_poll = 0.0
        self.last_quota_poll = 0.0
        self.last_prune = 0.0
        self.auth_interval = args.auth_interval
        self.quota_interval = args.quota_interval

    def stop(self, *_: Any) -> None:
        self.stopping = True

    def poll_auth(self) -> None:
        self.auth_entries = self.management.list_auth_files()
        self.database.update_accounts(self.auth_entries)
        self.last_auth_poll = utc_now()

    def drain_usage(self) -> int:
        total = 0
        for _ in range(20):
            records = self.management.pop_usage(500)
            if not records:
                break
            total += self.database.insert_usage_records(records)
            if len(records) < 500:
                break
        return total

    def poll_quotas(self) -> int:
        active = [
            entry
            for entry in self.auth_entries
            if not entry.get("disabled") and str(entry.get("status")) != "disabled"
        ]

        def fetch(entry: dict[str, Any]) -> tuple[str, dict[str, Any] | None, str | None]:
            auth_index = str(entry.get("auth_index") or "")
            name = str(entry.get("name") or "")
            try:
                credential = self.management.download_auth(name)
                return auth_index, fetch_wham_usage(credential), None
            except urllib.error.HTTPError as error:
                return auth_index, None, f"http_{error.code}"
            except Exception as error:  # noqa: BLE001 - daemon boundary
                return auth_index, None, type(error).__name__

        inserted = 0
        sampled_at = utc_now()
        with concurrent.futures.ThreadPoolExecutor(max_workers=min(6, max(1, len(active)))) as pool:
            for auth_index, payload, error in pool.map(fetch, active):
                if payload is not None:
                    inserted += self.database.insert_quota_payload(
                        auth_index, payload, sampled_at
                    )
                elif error:
                    print(
                        f"quota sample failed auth={auth_index[:8]} error={error}",
                        file=sys.stderr,
                        flush=True,
                    )
        self.last_quota_poll = utc_now()
        return inserted

    def publish(self) -> dict[str, Any]:
        snapshot = self.database.build_snapshot()
        atomic_json_write(self.snapshot_path, snapshot)
        return snapshot

    def cycle(self, force: bool = False) -> dict[str, Any]:
        now = utc_now()
        if force or now - self.last_auth_poll >= self.auth_interval:
            self.poll_auth()
        self.drain_usage()
        self.database.join_new_api_logs(self.new_api_db)
        if force or now - self.last_quota_poll >= self.quota_interval:
            self.poll_quotas()
        self.database.calibrate_new_samples()
        if force or now - self.last_prune >= 3600:
            self.database.prune()
            self.last_prune = now
        return self.publish()

    def run(self, once: bool = False) -> None:
        if once:
            self.cycle(force=True)
            return
        while not self.stopping:
            started = time.monotonic()
            try:
                self.cycle()
            except Exception as error:  # noqa: BLE001 - keep the monitor alive
                print(
                    f"collector cycle failed: {type(error).__name__}: {error}",
                    file=sys.stderr,
                    flush=True,
                )
            elapsed = time.monotonic() - started
            time.sleep(max(0.2, 2.0 - elapsed))


def self_test() -> None:
    assert parse_iso_timestamp("2026-07-18T00:00:00Z") > 0
    assert safe_float("12.5") == 12.5
    assert safe_float("bad") is None
    with tempfile.TemporaryDirectory() as directory:
        database = MonitorDatabase(Path(directory) / "monitor.db")
        database.update_accounts(
            [
                {"auth_index": "a", "status": "active", "disabled": False},
                {"auth_index": "b", "status": "disabled", "disabled": True},
            ]
        )
        database.connection.execute(
            "UPDATE accounts SET last_quota_ok=? WHERE auth_index='a'", (utc_now(),)
        )
        database.connection.commit()
        snapshot = database.build_snapshot()
        assert snapshot["accounts"]["active"] == 1
        assert snapshot["accounts"]["disabled"] == 1
        assert snapshot["pool"]["remaining_percent"] is None
        assert snapshot["pool"]["status"] == "estimating"
    print("self-test passed")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--database", type=Path, default=Path("/var/lib/qapi-monitor/monitor.db")
    )
    parser.add_argument(
        "--snapshot", type=Path, default=Path("/run/qapi-monitor/status.json")
    )
    parser.add_argument("--management-url", default=DEFAULT_MANAGEMENT_URL)
    parser.add_argument(
        "--management-config",
        type=Path,
        default=Path("/etc/cliproxyapi/config.yaml"),
    )
    parser.add_argument(
        "--new-api-db", type=Path, default=Path("/opt/new-api/data/new-api.db")
    )
    parser.add_argument("--auth-interval", type=float, default=30.0)
    parser.add_argument("--quota-interval", type=float, default=60.0)
    parser.add_argument("--once", action="store_true")
    parser.add_argument("--self-test", action="store_true")
    return parser


def main() -> None:
    args = build_parser().parse_args()
    if args.self_test:
        self_test()
        return
    collector = Collector(args)
    signal.signal(signal.SIGTERM, collector.stop)
    signal.signal(signal.SIGINT, collector.stop)
    collector.run(once=args.once)


if __name__ == "__main__":
    main()
