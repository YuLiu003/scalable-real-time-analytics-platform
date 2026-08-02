from __future__ import annotations

import unittest
from dataclasses import replace

from agent_review_optimizer.model import ReviewInvocation, TokenUsage


RUN_ID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
INVOCATION_ID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"


def invocation(**overrides: object) -> ReviewInvocation:
    values = {
        "run_id": RUN_ID,
        "invocation_id": INVOCATION_ID,
        "occurred_at": "2026-08-01T10:00:00-07:00",
        "provider": "codex",
        "model": None,
        "strategy": "routed",
        "reviewer": "general",
        "status": "completed",
        "duration_ms": 100,
        "changed_file_count": 2,
        "proposed_findings": 2,
        "confirmed_findings": 1,
        "false_positive_findings": 1,
        "usage": TokenUsage(100, 80, 20, 5),
    }
    values.update(overrides)
    return ReviewInvocation(**values)  # type: ignore[arg-type]


class TokenUsageTests(unittest.TestCase):
    def test_round_trip_and_addition(self) -> None:
        first = TokenUsage.from_mapping(
            {
                "input_tokens": 100,
                "cached_input_tokens": 80,
                "output_tokens": 20,
                "reasoning_output_tokens": 5,
            }
        )
        second = TokenUsage(10, 2, 4, 1)
        self.assertEqual(
            (first + second).to_mapping(),
            {
                "input_tokens": 110,
                "cached_input_tokens": 82,
                "output_tokens": 24,
                "reasoning_output_tokens": 6,
            },
        )
        without_reasoning = TokenUsage.from_mapping(
            {
                "input_tokens": 1,
                "cached_input_tokens": 0,
                "output_tokens": 1,
                "reasoning_output_tokens": None,
            }
        )
        self.assertIsNone((first + without_reasoning).reasoning_output_tokens)

    def test_rejects_invalid_usage(self) -> None:
        with self.assertRaisesRegex(ValueError, "non-negative"):
            TokenUsage(True, 0, 0, 0)
        with self.assertRaisesRegex(ValueError, "non-negative"):
            TokenUsage(-1, 0, 0, 0)
        with self.assertRaisesRegex(ValueError, "cannot exceed"):
            TokenUsage(1, 2, 0, 0)
        with self.assertRaisesRegex(ValueError, "canonical token fields"):
            TokenUsage.from_mapping({"input_tokens": 1})


class ReviewInvocationTests(unittest.TestCase):
    def test_round_trip_normalizes_time_and_preserves_null_model(self) -> None:
        record = invocation()
        self.assertEqual(record.occurred_at, "2026-08-01T17:00:00Z")
        self.assertEqual(
            ReviewInvocation.from_mapping(record.to_mapping()).to_mapping(),
            record.to_mapping(),
        )
        self.assertEqual(replace(record, model="gpt-5.6-sol").model, "gpt-5.6-sol")

    def test_rejects_record_shape_and_version(self) -> None:
        value = invocation().to_mapping()
        value["prompt"] = "must not be accepted"
        with self.assertRaisesRegex(ValueError, "missing or unsupported"):
            ReviewInvocation.from_mapping(value)

        value = invocation().to_mapping()
        value["schema_version"] = 2
        with self.assertRaisesRegex(ValueError, "schema_version"):
            ReviewInvocation.from_mapping(value)
        value["schema_version"] = True
        with self.assertRaisesRegex(ValueError, "schema_version"):
            ReviewInvocation.from_mapping(value)

        value = invocation().to_mapping()
        value["usage"] = "not-an-object"
        with self.assertRaisesRegex(ValueError, "usage must be null or an object"):
            ReviewInvocation.from_mapping(value)

    def test_rejects_invalid_identifiers_and_timestamp(self) -> None:
        for bad_id in (
            1,
            "not-a-uuid",
            RUN_ID.upper(),
            "aaaaaaaa-aaaa-1aaa-8aaa-aaaaaaaaaaaa",
        ):
            with self.subTest(bad_id=bad_id), self.assertRaisesRegex(
                ValueError, "canonical UUID"
            ):
                invocation(run_id=bad_id)
        with self.assertRaisesRegex(ValueError, "canonical UUID"):
            invocation(invocation_id="bad")
        for bad_time, message in (
            (1, "RFC3339"),
            ("not-a-time", "RFC3339"),
            ("2026-08-01T10:00:00", "timezone"),
        ):
            with self.subTest(bad_time=bad_time), self.assertRaisesRegex(
                ValueError, message
            ):
                invocation(occurred_at=bad_time)

    def test_rejects_invalid_dimensions(self) -> None:
        cases = (
            ({"provider": 1}, "provider"),
            ({"provider": "Codex"}, "provider"),
            ({"provider": "claude"}, "provider"),
            ({"model": 1}, "model"),
            ({"model": "bad model"}, "model"),
            ({"strategy": "all"}, "strategy"),
            ({"reviewer": "database"}, "reviewer"),
            ({"status": "unknown"}, "status"),
            ({"duration_ms": True}, "duration_ms"),
            ({"duration_ms": -1}, "duration_ms"),
            ({"changed_file_count": True}, "changed_file_count"),
            ({"changed_file_count": 0}, "changed_file_count"),
            ({"proposed_findings": -1}, "proposed_findings"),
            ({"usage": {}}, "usage"),
        )
        for overrides, message in cases:
            with self.subTest(overrides=overrides), self.assertRaisesRegex(
                ValueError, message
            ):
                invocation(**overrides)

    def test_requires_final_adjudication_and_valid_failed_outcomes(self) -> None:
        with self.assertRaisesRegex(ValueError, "full finding adjudication"):
            invocation(
                proposed_findings=1,
                confirmed_findings=0,
                false_positive_findings=0,
            )
        with self.assertRaisesRegex(ValueError, "cannot declare findings"):
            invocation(status="failed")
        with self.assertRaisesRegex(ValueError, "require token usage"):
            invocation(usage=None)

        failed = invocation(
            status="failed",
            proposed_findings=0,
            confirmed_findings=0,
            false_positive_findings=0,
            usage=None,
        )
        self.assertIsNone(
            ReviewInvocation.from_mapping(failed.to_mapping()).usage
        )

    def test_enforces_deterministic_and_agent_dimensions(self) -> None:
        deterministic = invocation(
            provider="none",
            strategy="deterministic-only",
            reviewer="deterministic",
            proposed_findings=0,
            confirmed_findings=0,
            false_positive_findings=0,
            usage=TokenUsage(0, 0, 0, 0),
        )
        self.assertEqual(deterministic.provider, "none")

        cases = (
            ({"strategy": "deterministic-only"}, "provider none"),
            (
                {
                    "provider": "none",
                    "model": "tool",
                    "strategy": "deterministic-only",
                },
                "cannot declare a model",
            ),
            (
                {"provider": "none", "strategy": "deterministic-only"},
                "deterministic reviewer",
            ),
            (
                {
                    "provider": "none",
                    "strategy": "deterministic-only",
                    "reviewer": "deterministic",
                },
                "zero token usage",
            ),
            (
                {
                    "provider": "none",
                    "strategy": "deterministic-only",
                    "reviewer": "deterministic",
                    "proposed_findings": 0,
                    "confirmed_findings": 0,
                    "false_positive_findings": 0,
                    "usage": TokenUsage(0, 0, 0, None),
                },
                "zero token usage",
            ),
            ({"provider": "none"}, "require a provider"),
            ({"reviewer": "deterministic"}, "agent reviewer"),
        )
        for overrides, message in cases:
            with self.subTest(overrides=overrides), self.assertRaisesRegex(
                ValueError, message
            ):
                invocation(**overrides)


if __name__ == "__main__":
    unittest.main()
