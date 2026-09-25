"""CLI adapter example for long-stocks-advisor's caller-owned Redis state."""
from __future__ import annotations

import json
import os
import subprocess
import sys


def rate_entity(redis_client, entity_key: str, score: float, periods_elapsed: int = 0) -> dict:
    """Rate one resolved signal, then persist the returned state in Redis.

    The caller should invoke this only after its own prediction resolver returns
    a terminal win/loss. A win maps to 1.0 and a loss to 0.0. The fixed 1500
    opponent is the stocks project's v1 policy.
    """
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
    completed = subprocess.run(
        ["credrate", "rate"], input=json.dumps(request), text=True,
        capture_output=True, check=True,
    )
    result = json.loads(completed.stdout)
    redis_client.set(redis_key, json.dumps(result["state"], separators=(",", ":")))
    return result


if __name__ == "__main__":
    # Demonstration: connect to local Redis and record one resolved winning call.
    # Production code should pass its existing Redis client and the actual resolver result.
    from redis import Redis

    key = sys.argv[1] if len(sys.argv) > 1 else "manager:example"
    redis_client = Redis.from_url(
        os.environ.get("REDIS_URL", "redis://localhost:6379/0"), decode_responses=True
    )
    score = float(os.environ.get("EXAMPLE_SCORE", "1"))
    print(json.dumps(rate_entity(redis_client, key, score), indent=2))
