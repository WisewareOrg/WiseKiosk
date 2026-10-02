#!/usr/bin/env python3
"""Helpers the bring-up and image-swap harnesses both call.

docs/CI.md § Deployment and bring-up: health poll and configuration fetch are the same assertion
in both harnesses — derive a deadline from an image's own declared healthcheck and poll Docker's
own status for it, and fetch a URL and assert it returns given bytes. Resolving a field off a
manifest through `docker buildx imagetools inspect` is the same call in both, parsing a different
field. How each harness reaches a container's address differs (`docker compose port` against
`kiosk`, `docker port` against a container run directly) and stays with the harness that reaches
it.
"""

import json
import subprocess
import time
import urllib.error
import urllib.request

IMAGE = "ghcr.io/wisewareorg/wisekiosk"
CONFIG_URL = "/config.json"

# Docker's own substitution for a HEALTHCHECK declaration naming no interval, retries or start
# period, in the units `docker image inspect` itself reports (nanoseconds).
DEFAULT_INTERVAL_NS = 30_000_000_000
DEFAULT_RETRIES = 3
DEFAULT_START_PERIOD_NS = 0

HEALTH_POLL_INTERVAL = 1.0


class HarnessError(Exception):
    """A step or assertion a harness did not satisfy."""


def healthcheck_deadline(image_ref):
    """The deadline a container from image_ref is given to reach Docker health status healthy."""
    completed = subprocess.run(
        ["docker", "image", "inspect", "--format", "{{json .Config.Healthcheck}}", image_ref],
        capture_output=True,
        text=True,
    )
    if completed.returncode != 0:
        raise HarnessError(
            f"`docker image inspect {image_ref}` exited {completed.returncode} "
            f"({completed.stderr.strip()})"
        )
    healthcheck = json.loads(completed.stdout.strip()) or {}
    interval = healthcheck.get("Interval") or DEFAULT_INTERVAL_NS
    retries = healthcheck.get("Retries") or DEFAULT_RETRIES
    start_period = healthcheck.get("StartPeriod") or DEFAULT_START_PERIOD_NS
    return (start_period + interval * (retries + 1)) / 1_000_000_000


def wait_healthy(container, deadline_seconds):
    deadline = time.monotonic() + deadline_seconds
    status = None
    while time.monotonic() < deadline:
        completed = subprocess.run(
            ["docker", "inspect", "--format", "{{.State.Health.Status}}", container],
            capture_output=True,
            text=True,
        )
        if completed.returncode != 0:
            raise HarnessError(
                f"`docker inspect {container}` exited {completed.returncode} "
                f"({completed.stderr.strip()})"
            )
        status = completed.stdout.strip()
        if status == "healthy":
            return
        if status == "unhealthy":
            raise HarnessError(f"{container} reported unhealthy before reaching healthy")
        time.sleep(HEALTH_POLL_INTERVAL)
    raise HarnessError(
        f"{container} did not reach healthy within {deadline_seconds:.0f}s, derived from its "
        f"declared healthcheck (last status {status!r})"
    )


def assert_config_served(address, expected):
    """`GET /config.json` at address returns expected, byte for byte."""
    try:
        with urllib.request.urlopen(f"http://{address}{CONFIG_URL}", timeout=5) as response:
            status, body = response.status, response.read()
    except urllib.error.HTTPError as error:
        status, body = error.code, error.read()
    if status != 200:
        raise HarnessError(f"{CONFIG_URL} answered {status}, expected 200")
    if body != expected:
        raise HarnessError(
            f"{CONFIG_URL} served {len(body)} byte(s) that are not the mounted configuration's "
            f"{len(expected)} — the mounted configuration is not what reaches the page"
        )


def imagetools_inspect(image_ref, template_field):
    """template_field off image_ref's manifest, or None with the detail on a failed resolution."""
    completed = subprocess.run(
        [
            "docker",
            "buildx",
            "imagetools",
            "inspect",
            "--format",
            f"{{{{json {template_field}}}}}",
            image_ref,
        ],
        capture_output=True,
        text=True,
    )
    if completed.returncode != 0:
        return None, completed.stderr.strip()
    return json.loads(completed.stdout.strip()), None
