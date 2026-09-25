"""HTTP adapter example for long-stocks-advisor's caller-owned Redis state.

Same contract as rate_entity.py, but talks to a running `credserve` (POST /rate)
instead of spawning the CLI. Use this when a long-lived service is cheaper than a
process per rating (e.g. high call volume). Stdlib only — no `requests` dependency.

Run the service first:
    go run ./cmd/credserve            # listens on 127.0.0.1:8080 by default
    python examples/long-stocks-advisor/rate_entity_http.py manager:buffett
"""
from __future__ import annotations

import json
import os
import sys
import urllib.request


def rate_entity(redis_client, entity_key: str, score: float, periods_elapsed: int = 0) -> dict:
    """Rate one resolved signal via the HTTP service, then persist state in Redis.

    Call this only after the prediction resolver returns a terminal win/loss: a win
    maps to 1.0 and a loss to 0.0. The fixed 1500 opponent is the stocks project's
    v1 policy.
    """
    base_url = os.environ.get("CREDSERVE_URL", "http://127.0.0.1:8080").rstrip("/")
    redis_key = f"credibility:glicko2:{entity_key}"
    saved = redis_client.get(redis_key)
    current = json.loads(saved) if saved else {
        "algo": "glicko2", "rating": 1500.0, "rd": 350.0, "vol": 0.06, "n": 0
    }
    request = {
        "algo": "glicko2",
        "current": current,
        "periods_elapsed": periods_elapsed,
        "outcomes": [{"opponent_rating": 1500.0, "opponent_rd": 350.0, "score": score}],
        "params": {"tau": 0.5},
    }
    req = urllib.request.Request(
        f"{base_url}/rate",
        data=json.dumps(request).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req) as resp:  # noqa: S310 (trusted internal service)
        result = json.load(resp)
    redis_client.set(redis_key, json.dumps(result["state"], separators=(",", ":")))
    return result


if __name__ == "__main__":
    # Demonstration: connect to local Redis and record one resolved winning call
    # against a running credserve. Production code passes its existing Redis client
    # and the actual resolver result.
    from redis import Redis

    key = sys.argv[1] if len(sys.argv) > 1 else "manager:example"
    redis_client = Redis.from_url(
        os.environ.get("REDIS_URL", "redis://localhost:6379/0"), decode_responses=True
    )
    score = float(os.environ.get("EXAMPLE_SCORE", "1"))
    print(json.dumps(rate_entity(redis_client, key, score), indent=2))
