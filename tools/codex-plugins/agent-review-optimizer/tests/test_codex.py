from __future__ import annotations

import json
import unittest

from agent_review_optimizer.codex import parse_codex_jsonl


class CodexAdapterTests(unittest.TestCase):
    def test_extracts_completed_usage_without_content(self) -> None:
        lines = [
            "\n",
            json.dumps({"type": "thread.started", "thread_id": "private"}),
            json.dumps(
                {
                    "type": "item.completed",
                    "item": {"type": "agent_message", "text": "private code"},
                }
            ),
            json.dumps(
                {
                    "type": "turn.completed",
                    "usage": {
                        "input_tokens": 100,
                        "cached_input_tokens": 80,
                        "output_tokens": 20,
                        "reasoning_output_tokens": 5,
                        "future_provider_field": "ignored",
                    },
                }
            ),
        ]
        terminal = parse_codex_jsonl(lines)
        self.assertEqual(terminal.status, "completed")
        self.assertIsNotNone(terminal.usage)
        self.assertEqual(
            terminal.usage.to_mapping(),  # type: ignore[union-attr]
            {
                "input_tokens": 100,
                "cached_input_tokens": 80,
                "output_tokens": 20,
                "reasoning_output_tokens": 5,
            },
        )

    def test_extracts_failed_terminal_with_optional_usage(self) -> None:
        without_usage = parse_codex_jsonl([json.dumps({"type": "turn.failed"})])
        self.assertEqual(without_usage.status, "failed")
        self.assertIsNone(without_usage.usage)

        with_usage = parse_codex_jsonl(
            [
                json.dumps(
                    {
                        "type": "turn.failed",
                        "usage": {
                            "input_tokens": 10,
                            "cached_input_tokens": 2,
                            "output_tokens": 4,
                            "reasoning_output_tokens": 1,
                        },
                    }
                )
            ]
        )
        self.assertEqual(with_usage.usage.input_tokens, 10)  # type: ignore[union-attr]

    def test_preserves_missing_reasoning_usage_as_unknown(self) -> None:
        terminal = parse_codex_jsonl(
            [
                json.dumps(
                    {
                        "type": "turn.completed",
                        "usage": {
                            "input_tokens": 10,
                            "cached_input_tokens": 2,
                            "output_tokens": 4,
                        },
                    }
                )
            ]
        )
        self.assertIsNone(terminal.usage.reasoning_output_tokens)  # type: ignore[union-attr]

    def test_rejects_invalid_json_and_event_shapes(self) -> None:
        cases = (
            (["{"], "invalid Codex JSONL"),
            (["[]"], "invalid Codex event"),
            ([json.dumps({"item": {}})], "invalid Codex event"),
            ([json.dumps({"type": "turn.completed"})], "missing Codex usage"),
            (
                [json.dumps({"type": "turn.completed", "usage": {}})],
                "missing Codex usage",
            ),
        )
        for lines, message in cases:
            with self.subTest(lines=lines), self.assertRaisesRegex(ValueError, message):
                parse_codex_jsonl(lines)

    def test_rejects_stream_without_completed_usage(self) -> None:
        with self.assertRaisesRegex(ValueError, "no terminal turn"):
            parse_codex_jsonl([json.dumps({"type": "turn.started"})])

    def test_rejects_multiple_terminal_events(self) -> None:
        completed = {
            "type": "turn.completed",
            "usage": {
                "input_tokens": 1,
                "cached_input_tokens": 0,
                "output_tokens": 0,
                "reasoning_output_tokens": 0,
            },
        }
        with self.assertRaisesRegex(ValueError, "multiple terminal"):
            parse_codex_jsonl([json.dumps(completed), json.dumps(completed)])


if __name__ == "__main__":
    unittest.main()
