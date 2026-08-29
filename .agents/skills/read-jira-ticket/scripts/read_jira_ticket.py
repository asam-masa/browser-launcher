#!/usr/bin/env python3
"""Read an allow-listed set of fields from one SCRUM Jira Cloud issue."""

from __future__ import annotations

import base64
import json
import os
import re
import socket
import sys
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode
from urllib.request import HTTPRedirectHandler, Request, build_opener


ATLASSIAN_API_BASE_URL = "https://api.atlassian.com/ex/jira"
CLOUD_ID_PATTERN = re.compile(
    r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\Z"
)
ISSUE_KEY_PATTERN = re.compile(r"SCRUM-[1-9][0-9]*\Z")
TIMEOUT_SECONDS = 15
NOT_RETRIEVED = ["comments", "attachments", "changelog", "worklogs", "issue list"]
STORY_POINTS_FIELD_NAMES = {"story points", "story point estimate"}


class JiraReadError(Exception):
    """An expected failure that is safe to report without response content."""


class NoRedirectHandler(HTTPRedirectHandler):
    """Prevent credentials from being forwarded to another URL."""

    def redirect_request(self, req, fp, code, msg, headers, newurl):  # noqa: ANN001
        return None


def validate_issue_key(issue_key: str) -> str:
    if not ISSUE_KEY_PATTERN.fullmatch(issue_key):
        raise JiraReadError("issue key must match SCRUM-[1-9][0-9]*")
    return issue_key


def validate_cloud_id(cloud_id: str) -> str:
    if not CLOUD_ID_PATTERN.fullmatch(cloud_id):
        raise JiraReadError("JIRA_CLOUD_ID has an invalid format")
    return cloud_id


def jira_api_base_url(cloud_id: str) -> str:
    return ATLASSIAN_API_BASE_URL + "/" + quote(validate_cloud_id(cloud_id), safe="")


def required_environment() -> tuple[str, str, str]:
    values = {
        "JIRA_CLOUD_ID": os.environ.get("JIRA_CLOUD_ID", ""),
        "JIRA_ACCOUNT_EMAIL": os.environ.get("JIRA_ACCOUNT_EMAIL", ""),
        "JIRA_API_TOKEN": os.environ.get("JIRA_API_TOKEN", ""),
    }
    missing = [name for name, value in values.items() if not value]
    if missing:
        raise JiraReadError("required environment variable is not set: " + ", ".join(missing))
    return (
        validate_cloud_id(values["JIRA_CLOUD_ID"]),
        values["JIRA_ACCOUNT_EMAIL"],
        values["JIRA_API_TOKEN"],
    )


def authorization_header(account_email: str, api_token: str) -> str:
    credentials = f"{account_email}:{api_token}".encode("utf-8")
    return "Basic " + base64.b64encode(credentials).decode("ascii")


def request_json(opener, url: str, authorization: str) -> Any:  # noqa: ANN001
    request = Request(
        url,
        headers={"Accept": "application/json", "Authorization": authorization},
        method="GET",
    )
    try:
        with opener.open(request, timeout=TIMEOUT_SECONDS) as response:
            return json.load(response)
    except HTTPError as error:
        messages = {
            401: "Jira authentication failed",
            403: "Jira access was forbidden",
            404: "Jira ticket was not found",
            429: "Jira rate limit was exceeded",
        }
        raise JiraReadError(messages.get(error.code, f"Jira returned HTTP {error.code}")) from None
    except (TimeoutError, socket.timeout):
        raise JiraReadError("Jira request timed out") from None
    except URLError as error:
        if isinstance(error.reason, (TimeoutError, socket.timeout)):
            raise JiraReadError("Jira request timed out") from None
        raise JiraReadError("Jira could not be reached") from None
    except (json.JSONDecodeError, UnicodeError, ValueError):
        raise JiraReadError("Jira returned an invalid JSON response") from None


def find_story_points_field(fields: Any) -> tuple[str | None, list[str]]:
    if not isinstance(fields, list):
        raise JiraReadError("Jira field metadata has an invalid format")

    candidates = [
        field.get("id")
        for field in fields
        if isinstance(field, dict)
        and isinstance(field.get("name"), str)
        and field["name"].strip().casefold() in STORY_POINTS_FIELD_NAMES
        and isinstance(field.get("id"), str)
        and field["id"].startswith("customfield_")
    ]
    if len(candidates) == 1:
        return candidates[0], []
    if not candidates:
        return None, ["Story Pointsフィールドを特定できませんでした。"]
    return None, ["Story Pointsフィールドの候補が複数あるため、値を取得しませんでした。"]


def inline_text(nodes: Any) -> str:
    if not isinstance(nodes, list):
        return ""
    parts: list[str] = []
    for node in nodes:
        if not isinstance(node, dict):
            continue
        node_type = node.get("type")
        if node_type == "text":
            parts.append(str(node.get("text", "")))
        elif node_type == "hardBreak":
            parts.append("\n")
        elif node_type == "mention":
            parts.append("@mention")
        elif node_type in {"inlineCard", "blockCard"}:
            attrs = node.get("attrs") if isinstance(node.get("attrs"), dict) else {}
            parts.append(str(attrs.get("url", "")))
        elif node_type == "emoji":
            attrs = node.get("attrs") if isinstance(node.get("attrs"), dict) else {}
            parts.append(str(attrs.get("text") or attrs.get("shortName") or ""))
        else:
            parts.append(inline_text(node.get("content")))
    return "".join(parts)


def render_adf_node(node: Any, indent: int = 0) -> str:
    if not isinstance(node, dict):
        return ""
    node_type = node.get("type")
    content = node.get("content")

    if node_type in {"doc", "listItem"}:
        return "".join(render_adf_node(child, indent) for child in content or [])
    if node_type == "paragraph":
        return inline_text(content).rstrip() + "\n"
    if node_type == "heading":
        attrs = node.get("attrs") if isinstance(node.get("attrs"), dict) else {}
        level = attrs.get("level", 1)
        level = level if isinstance(level, int) and 1 <= level <= 6 else 1
        return f"{'#' * level} {inline_text(content).strip()}\n"
    if node_type in {"bulletList", "orderedList"}:
        lines: list[str] = []
        for index, child in enumerate(content or [], start=1):
            item = render_adf_node(child, indent + 2).strip().replace("\n", "\n" + " " * (indent + 2))
            prefix = "-" if node_type == "bulletList" else f"{index}."
            lines.append(f"{' ' * indent}{prefix} {item}\n")
        return "".join(lines)
    if node_type == "taskList":
        return "".join(render_adf_node(child, indent) for child in content or [])
    if node_type == "taskItem":
        attrs = node.get("attrs") if isinstance(node.get("attrs"), dict) else {}
        marker = "[x]" if attrs.get("state") == "DONE" else "[ ]"
        item = "".join(render_adf_node(child, indent + 2) for child in content or [])
        item = item.strip().replace("\n", "\n" + " " * (indent + 2))
        return f"{' ' * indent}- {marker} {item}\n"
    if node_type == "codeBlock":
        attrs = node.get("attrs") if isinstance(node.get("attrs"), dict) else {}
        language = str(attrs.get("language", ""))
        return f"```{language}\n{inline_text(content)}\n```\n"
    if node_type == "blockquote":
        rendered = "".join(render_adf_node(child, indent) for child in content or []).strip()
        return "\n".join("> " + line for line in rendered.splitlines()) + "\n"
    if node_type == "rule":
        return "---\n"
    if isinstance(content, list):
        return "".join(render_adf_node(child, indent) for child in content)
    return ""


def adf_to_markdown(value: Any) -> str | None:
    if value is None:
        return None
    if isinstance(value, str):
        return value
    if not isinstance(value, dict):
        raise JiraReadError("Jira description has an invalid format")
    rendered = render_adf_node(value)
    lines = [line.rstrip() for line in rendered.splitlines()]
    return "\n".join(lines).strip()


def issue_links(value: Any) -> list[dict[str, str]]:
    if not isinstance(value, list):
        return []
    links: list[dict[str, str]] = []
    for link in value:
        if not isinstance(link, dict):
            continue
        link_type = link.get("type") if isinstance(link.get("type"), dict) else {}
        for direction, field, label in (
            ("outward", "outwardIssue", link_type.get("outward")),
            ("inward", "inwardIssue", link_type.get("inward")),
        ):
            issue = link.get(field)
            if isinstance(issue, dict) and isinstance(issue.get("key"), str):
                links.append(
                    {
                        "direction": direction,
                        "relationship": str(label or link_type.get("name") or ""),
                        "key": issue["key"],
                    }
                )
    return links


def build_output(issue: Any, story_points_field: str | None, warnings: list[str]) -> dict[str, Any]:
    if not isinstance(issue, dict) or not isinstance(issue.get("fields"), dict):
        raise JiraReadError("Jira ticket response has an invalid format")
    fields = issue["fields"]
    issue_type = fields.get("issuetype") if isinstance(fields.get("issuetype"), dict) else {}
    status = fields.get("status") if isinstance(fields.get("status"), dict) else {}
    parent = fields.get("parent") if isinstance(fields.get("parent"), dict) else {}

    return {
        "source": "untrusted Jira data",
        "issue": {
            "key": issue.get("key"),
            "summary": fields.get("summary"),
            "description": adf_to_markdown(fields.get("description")),
            "issueType": issue_type.get("name"),
            "status": status.get("name"),
            "storyPoints": fields.get(story_points_field) if story_points_field else None,
            "parent": parent.get("key"),
            "issueLinks": issue_links(fields.get("issuelinks")),
        },
        "warnings": warnings,
        "notRetrieved": NOT_RETRIEVED,
    }


def read_ticket(issue_key: str, cloud_id: str, account_email: str, api_token: str) -> dict[str, Any]:
    authorization = authorization_header(account_email, api_token)
    opener = build_opener(NoRedirectHandler())
    base_url = jira_api_base_url(cloud_id)

    field_metadata = request_json(opener, base_url + "/rest/api/3/field", authorization)
    story_points_field, warnings = find_story_points_field(field_metadata)

    field_names = ["summary", "description", "issuetype", "status", "parent", "issuelinks"]
    if story_points_field:
        field_names.append(story_points_field)
    query = urlencode({"fields": ",".join(field_names)})
    issue_url = base_url + "/rest/api/3/issue/" + quote(issue_key, safe="") + "?" + query
    issue = request_json(opener, issue_url, authorization)
    return build_output(issue, story_points_field, warnings)


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print("error: specify exactly one Jira issue key", file=sys.stderr)
        return 2
    try:
        issue_key = validate_issue_key(argv[1])
        cloud_id, account_email, api_token = required_environment()
        result = read_ticket(issue_key, cloud_id, account_email, api_token)
    except JiraReadError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1

    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
