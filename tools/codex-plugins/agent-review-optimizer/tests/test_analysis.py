from __future__ import annotations

import json
import unittest

from agent_review_optimizer.analysis import parse_review_jsonl, summarize
from agent_review_optimizer.model import ReviewInvocation, TokenUsage


def record(
    invocation_id: str,
    *,
    run_id: str = "11111111-1111-4111-8111-111111111111",
    model: str | None = None,
    strategy: str = "routed",
    reviewer: str = "general",
    provider: str = "codex",
    status: str = "completed",
    duration_ms: int = 100,
    proposed: int = 1,
    confirmed: int = 1,
    false_positive: int = 0,
    usage: TokenUsage | None = TokenUsage(100, 80, 20, 5),
) -> ReviewInvocation:
    return ReviewInvocation(
        run_id=run_id,
        invocation_id=invocation_id,
        occurred_at="2026-08-01T17:00:00Z",
        provider=provider,
        model=model,
        strategy=strategy,
        reviewer=reviewer,
        status=status,
        duration_ms=duration_ms,
        changed_file_count=2,
        proposed_findings=proposed,
        confirmed_findings=confirmed,
        false_positive_findings=false_positive,
        usage=usage,
    )


class AnalysisTests(unittest.TestCase):
    def test_parses_and_summarizes_observed_cohorts(self) -> None:
        records = [
            record("22222222-2222-4222-8222-222222222222"),
            record(
                "33333333-3333-4333-8333-333333333333",
                duration_ms=200,
                proposed=1,
                confirmed=0,
                false_positive=1,
                usage=TokenUsage(50, 10, 10, 2),
            ),
            record(
                "44444444-4444-4444-8444-444444444444",
                run_id="55555555-5555-4555-8555-555555555555",
                model="gpt-5.6-sol",
                strategy="broadcast",
                reviewer="go",
                status="failed",
                duration_ms=500,
                proposed=0,
                confirmed=0,
                usage=None,
            ),
        ]
        parsed = parse_review_jsonl(
            ["\n", *(json.dumps(item.to_mapping()) for item in records)]
        )
        report = summarize(parsed)
        self.assertEqual(report["run_count"], 2)
        self.assertEqual(report["invocation_count"], 3)
        self.assertEqual(report["evidence_scope"], "sanitized_local_review_records")

        broadcast, routed = report["cohorts"]
        self.assertEqual(broadcast["completed_invocations"], 0)
        self.assertEqual(broadcast["usage_measured_invocations"], 0)
        self.assertIsNone(broadcast["input_tokens"])
        self.assertIsNone(broadcast["reasoning_output_tokens"])
        self.assertEqual(broadcast["confirmed_finding_rate"], None)
        self.assertEqual(broadcast["false_positive_rate"], None)
        self.assertEqual(
            broadcast["uncached_plus_output_tokens_per_confirmed_finding"], None
        )
        self.assertEqual(routed["median_duration_ms"], 150.0)
        self.assertEqual(routed["p95_duration_ms"], 200)
        self.assertEqual(routed["zero_yield_invocations"], 1)
        self.assertEqual(routed["confirmed_finding_rate"], 0.5)
        self.assertEqual(routed["false_positive_rate"], 0.5)
        self.assertEqual(
            routed["uncached_plus_output_tokens_per_confirmed_finding"], 90.0
        )

    def test_rejects_invalid_or_empty_jsonl(self) -> None:
        cases = ((["{"], "invalid review JSONL"), (["[]"], "must be an object"), ([], "at least one"))
        for lines, message in cases:
            with self.subTest(lines=lines), self.assertRaisesRegex(ValueError, message):
                parse_review_jsonl(lines)

    def test_rejects_duplicate_invocation_ids(self) -> None:
        item = record("22222222-2222-4222-8222-222222222222")
        with self.assertRaisesRegex(ValueError, "duplicate invocation_id"):
            summarize([item, item])

    def test_keeps_primary_usage_when_reasoning_usage_is_unknown(self) -> None:
        report = summarize(
            [
                record(
                    "22222222-2222-4222-8222-222222222222",
                    usage=TokenUsage(100, 80, 20, None),
                )
            ]
        )
        cohort = report["cohorts"][0]
        self.assertEqual(cohort["uncached_input_tokens"], 20)
        self.assertEqual(cohort["output_tokens"], 20)
        self.assertIsNone(cohort["reasoning_output_tokens"])
        self.assertEqual(
            cohort["uncached_plus_output_tokens_per_confirmed_finding"], 40.0
        )


if __name__ == "__main__":
    unittest.main()
