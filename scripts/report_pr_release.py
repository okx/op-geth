#!/usr/bin/env python3
"""
Report a released JIRA Epic to the OKone change-reporting API so it is counted
in the PR measurement Grafana dashboard.

Usage:
    python3 scripts/report_pr_release.py XLOP-1045

Required environment variables:
    OKONE_APP_KEY     OKone Open API access key (AK)
    OKONE_APP_SECRET  OKone Open API secret key (SK)

Optional environment variables:
    OKONE_API_BASE_URL  defaults to https://okoneopenapi.okg.com/apigw
    OKONE_AUTO_CLOSE    set to "true" to auto-close the JIRA issue (default: false)

Signing algorithm (HMAC-SHA256):
    signature_string = "METHOD\nPATH\nTIMESTAMP"
    signature = base64(hmac_sha256(signature_string, secret_key))
"""

import hashlib
import hmac
import base64
import os
import re
import sys
import time
import json
from urllib import request, error


JIRA_KEY_RE = re.compile(r"^[A-Z][A-Z0-9_]+-\d+$")

API_PATH = "/v1/change/jira-issue/report"


def _sign(method: str, path: str, timestamp_ms: int, secret_key: str) -> str:
    signature_string = f"{method}\n{path}\n{timestamp_ms}"
    mac = hmac.new(secret_key.encode(), signature_string.encode(), hashlib.sha256)
    return base64.b64encode(mac.digest()).decode()


def report_release(jira_issue: str) -> dict:
    if not JIRA_KEY_RE.match(jira_issue):
        raise ValueError(f"Invalid JIRA issue key: {jira_issue!r}")

    app_key = os.environ.get("OKONE_APP_KEY", "")
    app_secret = os.environ.get("OKONE_APP_SECRET", "")
    base_url = os.environ.get("OKONE_API_BASE_URL", "https://okoneopenapi.okg.com/apigw")
    auto_close = os.environ.get("OKONE_AUTO_CLOSE", "false").lower() == "true"

    if not app_key or not app_secret:
        raise RuntimeError("OKONE_APP_KEY and OKONE_APP_SECRET must be set")

    timestamp_ms = int(time.time() * 1000)
    signature = _sign("POST", API_PATH, timestamp_ms, app_secret)

    url = base_url.rstrip("/") + API_PATH
    payload = json.dumps({"jiraIssue": jira_issue, "autoCloseJiraIssue": auto_close}).encode()

    headers = {
        "Content-Type": "application/json",
        "X-App-Key": app_key,
        "X-Timestamp": str(timestamp_ms),
        "X-Signature": signature,
        "X-Reporter": "gitlab-ci-op-geth",
    }

    req = request.Request(url, data=payload, headers=headers, method="POST")
    try:
        with request.urlopen(req, timeout=30) as resp:
            body = json.loads(resp.read())
    except error.HTTPError as exc:
        body_bytes = exc.read()
        raise RuntimeError(f"HTTP {exc.code}: {body_bytes.decode(errors='replace')}") from exc

    if body.get("code") != 0:
        raise RuntimeError(f"API error: {body}")

    return body


def _extract_jira_keys_from_log(log_output: str) -> list[str]:
    """Extract unique JIRA keys from git log output (looks for 'Jira: KEY' lines)."""
    keys = []
    seen = set()
    for match in re.finditer(r"Jira:\s*([A-Z][A-Z0-9_]+-\d+)", log_output):
        key = match.group(1)
        if key not in seen:
            seen.add(key)
            keys.append(key)
    return keys


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print(f"Usage: {argv[0]} <JIRA-KEY>", file=sys.stderr)
        return 1

    jira_issue = argv[1].strip()
    try:
        result = report_release(jira_issue)
        print(f"Reported {jira_issue} to OKone: {result}")
        return 0
    except Exception as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
