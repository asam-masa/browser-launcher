#!/usr/bin/env python3
"""Unit tests for read_jira_ticket.py that do not access Jira."""

from __future__ import annotations

import io
import json
import socket
import unittest
from urllib.error import HTTPError
from urllib.request import Request

import read_jira_ticket as target


class FakeResponse:
    def __init__(self, value):  # noqa: ANN001
        self.body = io.BytesIO(json.dumps(value).encode("utf-8"))

    def __enter__(self):
        return self.body

    def __exit__(self, exc_type, exc_value, traceback):  # noqa: ANN001
        return False


class FakeOpener:
    def __init__(self, value):  # noqa: ANN001
        self.value = value
        self.request: Request | None = None
        self.timeout: int | None = None

    def open(self, request: Request, timeout: int):
        self.request = request
        self.timeout = timeout
        return FakeResponse(self.value)


class ErrorOpener:
    def __init__(self, error):  # noqa: ANN001
        self.error = error

    def open(self, request: Request, timeout: int):
        raise self.error


class ReadJiraTicketTest(unittest.TestCase):
    def test_validate_issue_key(self):
        self.assertEqual(target.validate_issue_key("SCRUM-26"), "SCRUM-26")
        for value in ("SCRUM-0", "OTHER-26", "SCRUM-26/attachments", "https://example.com"):
            with self.subTest(value=value), self.assertRaises(target.JiraReadError):
                target.validate_issue_key(value)

    def test_validate_base_url(self):
        self.assertEqual(
            target.validate_base_url("https://kurosahari.atlassian.net/"),
            target.ALLOWED_BASE_URL,
        )
        with self.assertRaises(target.JiraReadError):
            target.validate_base_url("https://example.atlassian.net")

    def test_request_json_uses_get_and_does_not_embed_credentials_in_url(self):
        opener = FakeOpener({"ok": True})
        result = target.request_json(opener, "https://example.test/rest/api/3/field", "Basic secret")

        self.assertEqual(result, {"ok": True})
        self.assertIsNotNone(opener.request)
        self.assertEqual(opener.request.get_method(), "GET")
        self.assertEqual(opener.request.full_url, "https://example.test/rest/api/3/field")
        self.assertEqual(opener.request.get_header("Authorization"), "Basic secret")
        self.assertEqual(opener.timeout, target.TIMEOUT_SECONDS)

    def test_request_json_classifies_expected_http_errors(self):
        cases = {
            401: "Jira authentication failed",
            403: "Jira access was forbidden",
            404: "Jira ticket was not found",
            429: "Jira rate limit was exceeded",
        }
        for status, expected_message in cases.items():
            with self.subTest(status=status):
                error = HTTPError("https://example.test", status, "error", {}, None)
                with self.assertRaisesRegex(target.JiraReadError, expected_message):
                    target.request_json(ErrorOpener(error), "https://example.test", "Basic secret")

    def test_request_json_reports_timeout(self):
        with self.assertRaisesRegex(target.JiraReadError, "Jira request timed out"):
            target.request_json(
                ErrorOpener(socket.timeout()),
                "https://example.test",
                "Basic secret",
            )

    def test_redirects_are_rejected(self):
        handler = target.NoRedirectHandler()
        request = Request("https://kurosahari.atlassian.net/rest/api/3/field")

        self.assertIsNone(
            handler.redirect_request(
                request,
                None,
                302,
                "Found",
                {},
                "https://example.test/redirected",
            )
        )

    def test_find_story_points_field(self):
        field_id, warnings = target.find_story_points_field(
            [{"id": "customfield_10016", "name": "Story Points"}]
        )
        self.assertEqual(field_id, "customfield_10016")
        self.assertEqual(warnings, [])

        field_id, warnings = target.find_story_points_field(
            [{"id": "customfield_10016", "name": "Story point estimate"}]
        )
        self.assertEqual(field_id, "customfield_10016")
        self.assertEqual(warnings, [])

        field_id, warnings = target.find_story_points_field([])
        self.assertIsNone(field_id)
        self.assertEqual(len(warnings), 1)

        field_id, warnings = target.find_story_points_field(
            [
                {"id": "customfield_1", "name": "Story Points"},
                {"id": "customfield_2", "name": "Story Points"},
            ]
        )
        self.assertIsNone(field_id)
        self.assertEqual(len(warnings), 1)

    def test_adf_to_markdown(self):
        description = {
            "type": "doc",
            "version": 1,
            "content": [
                {
                    "type": "heading",
                    "attrs": {"level": 2},
                    "content": [{"type": "text", "text": "Purpose"}],
                },
                {
                    "type": "paragraph",
                    "content": [{"type": "text", "text": "Jiraを安全に参照する。"}],
                },
                {
                    "type": "bulletList",
                    "content": [
                        {
                            "type": "listItem",
                            "content": [
                                {
                                    "type": "paragraph",
                                    "content": [{"type": "text", "text": "GETだけを使う"}],
                                }
                            ],
                        }
                    ],
                },
                {
                    "type": "taskList",
                    "content": [
                        {
                            "type": "taskItem",
                            "attrs": {"state": "DONE"},
                            "content": [
                                {
                                    "type": "paragraph",
                                    "content": [
                                        {"type": "text", "text": "設計を確認する"},
                                        {"type": "hardBreak"},
                                        {
                                            "type": "mention",
                                            "attrs": {"text": "個人名", "id": "account-id"},
                                        },
                                    ],
                                }
                            ],
                        }
                    ],
                },
            ],
        }
        self.assertEqual(
            target.adf_to_markdown(description),
            "## Purpose\nJiraを安全に参照する。\n- GETだけを使う\n- [x] 設計を確認する\n  @mention",
        )

    def test_build_output_allows_only_expected_fields(self):
        issue = {
            "key": "SCRUM-26",
            "fields": {
                "summary": "Jiraを参照する",
                "description": None,
                "issuetype": {"name": "Task"},
                "status": {"name": "To Do"},
                "customfield_10016": 3,
                "parent": {"key": "SCRUM-1"},
                "issuelinks": [
                    {
                        "type": {"name": "Relates", "outward": "relates to"},
                        "outwardIssue": {"key": "SCRUM-25", "fields": {"summary": "hidden"}},
                    }
                ],
                "comment": {"comments": [{"body": "must not leak"}]},
                "attachment": [{"filename": "secret.txt"}],
            },
        }
        result = target.build_output(issue, "customfield_10016", [])

        self.assertEqual(
            set(result["issue"]),
            {"key", "summary", "description", "issueType", "status", "storyPoints", "parent", "issueLinks"},
        )
        self.assertEqual(result["issue"]["storyPoints"], 3)
        self.assertEqual(result["issue"]["issueLinks"][0]["key"], "SCRUM-25")
        serialized = json.dumps(result, ensure_ascii=False)
        self.assertNotIn("must not leak", serialized)
        self.assertNotIn("secret.txt", serialized)


if __name__ == "__main__":
    unittest.main()
