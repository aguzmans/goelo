# Credibility rating — conformance vectors (Phase-1 artifact)

These vectors are the **executable contract** for the extracted credibility rating core, and the
first thing to build (before the service, before either app's integration). They are the single
source of truth that:

1. **Prove the migration is safe** — the extracted ELO core must reproduce today's
   `research/source_credibility_db.go` `calculateEloChange()` output *exactly* (K-by-severity + the
   800/2400 clamp).
2. **Bind any cross-language port** — the Go core and any future Python port must pass the *same*
   vectors, so the vectors (not the code) are the contract if we go the "port instead of service" route.
3. **Pin Glicko-2 to the published spec** — `glicko2_vectors.json` is Glickman's own worked example.

## Files
- `elo_vectors.json` — ELO path. Exact integer outputs; `tolerance` is 0 (must match bit-for-bit).
- `glicko2_vectors.json` — Glicko-2 path. Float outputs with an explicit `tolerance`
  (the volatility solver and float order make exact equality unrealistic).

## The determinism rules the core MUST obey (or these vectors can't hold)
- **No clock in the core.** Inactivity aging is driven by an input `periods_elapsed`, never by reading
  wall-time. `updated_at` is a caller concern and is NOT part of the rating state the core reads/writes.
- **`periods_elapsed` semantics (applies with OR without outcomes).** It is the number of **idle** rating
  periods (no games) since the last update, and aging is applied **first, before** the supplied outcomes:
  `φ ← √(φ² + periods_elapsed·σ²)` using the current (pre-batch) σ; σ and rating are unchanged by idle
  periods. The supplied `outcomes[]` are the *current* (active) period and are **not** counted in
  `periods_elapsed` (a gapless cadence passes 0). Empty `outcomes[]` ⇒ the result is exactly the pre-aged
  state — that's why the inactivity case is just this step with no batch. **Age-then-rate is mandatory**
  (never age after rating); the `preage_then_batch_ordering` vector locks it via `rd_after_preage`.
- **No storage, no opponent lookup, no ranking in the core.** `opponent_rating` (ELO) and the
  `outcomes[]` opponents (Glicko-2) are supplied by the caller. Opponent *selection* is caller policy
  (wp = avg of cited sources; stocks = fixed 1500 — see the plan doc §11).
- **Policy stays caller-side.** `audit_floor` and any project-specific clamping are applied by the
  caller *after* the core returns. The core is clamp-free except ELO's fixed 800/2400 chess bounds,
  which are part of the ported formula.
- **ELO and Glicko-2 state are non-interchangeable.** No mid-life algorithm switch.

## How to use them
Author the core against these, then a table-test loads each `cases[]` entry, runs the update, and
asserts `out` within `tolerance`. Suggested acceptance for Phase-1:

- **ELO:** every case matches with tolerance 0. Additionally, generate a randomized cross-check
  (fuzz `current`, `correct`, `opponent_rating`, `severity`) against the *current* `calculateEloChange`
  in `ai-post-to-wp` and assert identical — that is the migration safety net.
- **Glicko-2:** `glickman_paper_example` matches the paper (r'≈1464.06, RD'≈151.52, σ'≈0.05999),
  intermediate `v`/`delta`/`E` within tolerance (catches a wrong `g`/`E`/solver before the final value
  hides it); `inactivity_one_period_rd_grows` shows RD rising 200→200.27 with no games;
  `new_entity_defaults_roundtrip` confirms the 1500/350/0.06 defaults.

## Not covered here (later phases)
- Severity in Glicko-2 (v1 is severity-agnostic; a weighted variant would ship its own vectors).
- Seed packs / normalization (caller-side, separately tested).
- Multi-period batching semantics beyond the single-period `outcomes[]` request.
