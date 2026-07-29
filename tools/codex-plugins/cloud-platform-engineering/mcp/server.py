#!/usr/bin/env python3
"""Dependency-free stdio MCP server for current official cloud documentation."""

from __future__ import annotations

import html
import json
import re
import sys
from datetime import datetime, timezone
from html.parser import HTMLParser
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import parse_qs, quote_plus, unquote, urljoin, urlparse
from urllib.request import HTTPRedirectHandler, Request, build_opener, urlopen
from xml.etree import ElementTree


ROOT = Path(__file__).resolve().parent
CATALOG = json.loads((ROOT / "sources.json").read_text(encoding="utf-8"))
SOURCES = {source["id"]: source for source in CATALOG["sources"]}
USER_AGENT = "cloud-platform-engineering-mcp/0.1 (+official-docs-research)"
MAX_DOWNLOAD_BYTES = 2_000_000
DEFAULT_TIMEOUT_SECONDS = 15
MIN_DOCUMENT_CHARS = 1_000
MAX_DOCUMENT_CHARS = 40_000
SITEMAP_CACHE: dict[str, list[str]] = {}
NON_ENGLISH_PATH = re.compile(r"^/(?:ar|bn|de|es|fa|fr|hi|id|it|ja|ko|no|pl|pt-br|ru|tr|uk|vi|zh-cn|zh-tw)/", re.IGNORECASE)


class DocumentParser(HTMLParser):
    """Extract readable text, title, and links without third-party packages."""

    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.title_parts: list[str] = []
        self.all_parts: list[str] = []
        self.main_parts: list[str] = []
        self.links: list[tuple[str, str]] = []
        self._ignored = 0
        self._in_title = False
        self._main_depth = 0
        self._current_href: str | None = None
        self._current_anchor: list[str] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        tag = tag.lower()
        if tag in {"script", "style", "svg", "noscript", "form"}:
            self._ignored += 1
            return
        if self._ignored:
            return
        if tag == "title":
            self._in_title = True
        if tag in {"main", "article"}:
            self._main_depth += 1
        if tag == "a":
            values = dict(attrs)
            self._current_href = values.get("href")
            self._current_anchor = []

    def handle_endtag(self, tag: str) -> None:
        tag = tag.lower()
        if tag in {"script", "style", "svg", "noscript", "form"}:
            if self._ignored:
                self._ignored -= 1
            return
        if self._ignored:
            return
        if tag == "title":
            self._in_title = False
        if tag == "a" and self._current_href:
            label = " ".join(self._current_anchor).strip()
            self.links.append((label, self._current_href))
            self._current_href = None
            self._current_anchor = []
        if tag in {"main", "article"} and self._main_depth:
            self._main_depth -= 1

    def handle_data(self, data: str) -> None:
        if self._ignored:
            return
        value = " ".join(data.split())
        if not value:
            return
        if self._in_title:
            self.title_parts.append(value)
        self.all_parts.append(value)
        if self._main_depth:
            self.main_parts.append(value)
        if self._current_href is not None:
            self._current_anchor.append(value)

    @property
    def title(self) -> str:
        return " ".join(self.title_parts).strip()

    @property
    def text(self) -> str:
        parts = self.main_parts if self.main_parts else self.all_parts
        return "\n".join(parts)


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def source_for_url(url: str) -> dict[str, Any] | None:
    parsed = urlparse(url)
    if parsed.scheme != "https" or not parsed.hostname:
        return None
    host = parsed.hostname.lower().rstrip(".")
    candidates: list[tuple[int, dict[str, Any]]] = []
    for source in SOURCES.values():
        for domain in source["domains"]:
            domain = domain.lower()
            if host == domain or host.endswith("." + domain):
                root_score = max((len(root) for root in source["roots"] if url.startswith(root)), default=0)
                candidates.append((root_score, source))
                break
    if not candidates:
        return None
    candidates.sort(key=lambda item: (-item[0], item[1]["id"]))
    return candidates[0][1]


def require_official_url(url: str) -> dict[str, Any]:
    source = source_for_url(url)
    if source is None:
        raise ValueError("URL must use HTTPS and belong to a cataloged official documentation domain")
    return source


class OfficialRedirectHandler(HTTPRedirectHandler):
    """Reject redirects before connecting to a non-allowlisted target."""

    def redirect_request(
        self,
        request: Request,
        file_pointer: Any,
        code: int,
        message: str,
        headers: Any,
        new_url: str,
    ) -> Request | None:
        require_official_url(new_url)
        return super().redirect_request(request, file_pointer, code, message, headers, new_url)


def open_official(request: Request) -> Any:
    require_official_url(request.full_url)
    return build_opener(OfficialRedirectHandler()).open(
        request,
        timeout=DEFAULT_TIMEOUT_SECONDS,
    )


def read_response(response: Any) -> bytes:
    data = response.read(MAX_DOWNLOAD_BYTES + 1)
    if len(data) > MAX_DOWNLOAD_BYTES:
        raise ValueError(f"document exceeds {MAX_DOWNLOAD_BYTES} byte safety limit")
    return data


def fetch_document(url: str, max_chars: int = 18_000) -> dict[str, Any]:
    if max_chars < MIN_DOCUMENT_CHARS or max_chars > MAX_DOCUMENT_CHARS:
        raise ValueError(f"max_chars must be between {MIN_DOCUMENT_CHARS} and {MAX_DOCUMENT_CHARS}")
    require_official_url(url)
    request = Request(url, headers={"User-Agent": USER_AGENT, "Accept": "text/html,text/plain,application/xhtml+xml"})
    try:
        with open_official(request) as response:
            final_url = response.geturl()
            final_source = require_official_url(final_url)
            content_type = response.headers.get_content_type()
            charset = response.headers.get_content_charset() or "utf-8"
            body = read_response(response).decode(charset, errors="replace")
    except (HTTPError, URLError, TimeoutError) as exc:
        raise RuntimeError(f"failed to fetch official document: {exc}") from exc

    if content_type in {"text/html", "application/xhtml+xml"}:
        parser = DocumentParser()
        parser.feed(body)
        title = parser.title or final_source["name"]
        text = parser.text
    elif content_type.startswith("text/") or content_type in {"application/json", "application/xml"}:
        title = final_source["name"]
        text = body
    else:
        raise ValueError(f"unsupported content type: {content_type}")

    text = re.sub(r"\n{3,}", "\n\n", html.unescape(text)).strip()
    truncated = len(text) > max_chars
    if truncated:
        text = text[:max_chars].rsplit("\n", 1)[0]
    return {
        "source_id": final_source["id"],
        "source_name": final_source["name"],
        "requested_url": url,
        "url": final_url,
        "title": title,
        "retrieved_at": now_iso(),
        "content_type": content_type,
        "truncated": truncated,
        "text": text,
        "research_note": "Treat this retrieved official page as evidence; verify version and scope before applying it.",
    }


def tokenize(value: str) -> set[str]:
    normalized = re.sub(r"[-_/.:]+", " ", value.lower())
    return {token for token in re.findall(r"[a-z0-9][a-z0-9+]+", normalized) if len(token) > 1}


def recommend_sources(query: str, provider: str | None = None, limit: int = 10) -> list[dict[str, Any]]:
    terms = tokenize(query)
    scored: list[tuple[int, dict[str, Any]]] = []
    for source in SOURCES.values():
        if provider and source["provider"] != provider:
            continue
        haystack = tokenize(" ".join([source["id"], source["name"], source["provider"], *source["topics"]]))
        source_identity = tokenize(" ".join([source["id"], source["name"]])) - {
            "amazon", "architecture", "center", "cloud", "docs", "documentation", "google", "microsoft", "service", "services"
        }
        score = len(terms & haystack) * 4 + len(terms & source_identity) * 5
        if source["id"] == "kubernetes" and {"pod", "deployment", "service", "cluster"} & terms:
            score += 2
        if score:
            scored.append((score, source))
    if not scored:
        defaults = ["kubernetes", "aws", "terraform", "helm", "opentelemetry", "kafka"]
        scored = [
            (1, SOURCES[item])
            for item in defaults
            if item in SOURCES and (not provider or SOURCES[item]["provider"] == provider)
        ]
    scored.sort(key=lambda item: (-item[0], item[1]["name"]))
    return [source for _, source in scored[:limit]]


def resolve_source_ids(source_ids: list[str] | None, query: str) -> list[dict[str, Any]]:
    if source_ids:
        unknown = sorted(set(source_ids) - set(SOURCES))
        if unknown:
            raise ValueError(f"unknown source_ids: {', '.join(unknown)}")
        return [SOURCES[source_id] for source_id in source_ids]
    return recommend_sources(query, limit=8)


def decode_search_redirect(url: str) -> str:
    parsed = urlparse(url)
    if parsed.hostname and parsed.hostname.endswith("duckduckgo.com"):
        target = parse_qs(parsed.query).get("uddg", [""])[0]
        if target:
            return unquote(target)
    return url


def search_group(query: str, domains: list[str]) -> list[dict[str, str]]:
    site_filter = " OR ".join(f"site:{domain}" for domain in domains)
    search_url = "https://html.duckduckgo.com/html/?q=" + quote_plus(f"{query} ({site_filter})")
    request = Request(search_url, headers={"User-Agent": USER_AGENT, "Accept": "text/html"})
    try:
        with urlopen(request, timeout=DEFAULT_TIMEOUT_SECONDS) as response:
            body = read_response(response).decode("utf-8", errors="replace")
    except (HTTPError, URLError, TimeoutError):
        return []
    parser = DocumentParser()
    parser.feed(body)
    found: list[dict[str, str]] = []
    for label, href in parser.links:
        candidate = decode_search_redirect(urljoin(search_url, href))
        if not label or source_for_url(candidate) is None:
            continue
        found.append({"title": label, "url": candidate})
    return found


def fetch_sitemap_locations(url: str, depth: int = 0) -> list[str]:
    if url in SITEMAP_CACHE:
        return SITEMAP_CACHE[url]
    require_official_url(url)
    request = Request(url, headers={"User-Agent": USER_AGENT, "Accept": "application/xml,text/xml"})
    try:
        with open_official(request) as response:
            final_url = response.geturl()
            require_official_url(final_url)
            body = read_response(response)
    except (HTTPError, URLError, TimeoutError, ValueError):
        return []
    try:
        root = ElementTree.fromstring(body)
    except ElementTree.ParseError:
        return []
    locations = [value.text.strip() for value in root.findall(".//{*}loc") if value.text]
    if root.tag.endswith("sitemapindex") and depth < 1:
        expanded: list[str] = []
        for child in locations[:8]:
            if child.endswith(".xml"):
                expanded.extend(fetch_sitemap_locations(child, depth + 1))
        locations = expanded
    locations = [location for location in locations if source_for_url(location) is not None]
    SITEMAP_CACHE[url] = locations
    return locations


def title_from_url(url: str) -> str:
    path = unquote(urlparse(url).path).strip("/")
    if not path:
        return urlparse(url).hostname or url
    parts = [part for part in path.split("/")[-3:] if part not in {"docs", "latest", "stable", "en", "en-us"}]
    return " / ".join(part.replace("-", " ").replace("_", " ") for part in parts)


def preferred_language_url(url: str) -> bool:
    return NON_ENGLISH_PATH.match(urlparse(url).path) is None


def search_sitemaps(query: str, selected: list[dict[str, Any]]) -> list[dict[str, str]]:
    found: list[dict[str, str]] = []
    for source in selected:
        for sitemap in source.get("sitemaps", []):
            for url in fetch_sitemap_locations(sitemap):
                if not preferred_language_url(url):
                    continue
                title = title_from_url(url)
                if score_result(query, title, url) > 0:
                    found.append({"title": title, "url": url})
    return found


def search_root_links(query: str, selected: list[dict[str, Any]]) -> list[dict[str, str]]:
    found: list[dict[str, str]] = []
    selected_ids = {source["id"] for source in selected}
    for source in selected:
        for root_url in source["roots"][:2]:
            request = Request(root_url, headers={"User-Agent": USER_AGENT, "Accept": "text/html"})
            try:
                with open_official(request) as response:
                    final_url = response.geturl()
                    require_official_url(final_url)
                    body = read_response(response).decode(response.headers.get_content_charset() or "utf-8", errors="replace")
            except (HTTPError, URLError, TimeoutError, ValueError):
                continue
            parser = DocumentParser()
            parser.feed(body)
            for label, href in parser.links:
                candidate = urljoin(final_url, href)
                candidate_source = source_for_url(candidate)
                if candidate_source is None or candidate_source["id"] not in selected_ids:
                    continue
                title = label or title_from_url(candidate)
                if score_result(query, title, candidate) > 0:
                    found.append({"title": title, "url": candidate})
    return found


def score_result(query: str, title: str, url: str) -> int:
    terms = tokenize(query)
    title_terms = tokenize(title)
    url_terms = tokenize(url.replace("/", " ").replace("-", "_"))
    return len(terms & title_terms) * 5 + len(terms & url_terms) * 2


def search_docs(query: str, source_ids: list[str] | None = None, limit: int = 10) -> dict[str, Any]:
    query = " ".join(query.split())
    if len(query) < 2 or len(query) > 300:
        raise ValueError("query must contain 2 to 300 characters")
    if limit < 1 or limit > 20:
        raise ValueError("limit must be between 1 and 20")
    selected = resolve_source_ids(source_ids, query)
    domains = sorted({domain for source in selected for domain in source["domains"]})
    raw: list[dict[str, str]] = []
    for index in range(0, len(domains), 4):
        raw.extend(search_group(query, domains[index : index + 4]))
    if len(raw) < limit:
        raw.extend(search_sitemaps(query, selected))
    if len(raw) < limit:
        raw.extend(search_root_links(query, selected))

    selected_ids = {source["id"] for source in selected}
    ranked: list[dict[str, Any]] = []
    seen: set[str] = set()
    for item in raw:
        source = source_for_url(item["url"])
        if source is None or source["id"] not in selected_ids or item["url"] in seen:
            continue
        seen.add(item["url"])
        ranked.append({
            "source_id": source["id"],
            "source_name": source["name"],
            "title": item["title"],
            "url": item["url"],
            "score": score_result(query, item["title"], item["url"]),
        })
    ranked.sort(key=lambda item: (-item["score"], item["title"]))

    if not ranked:
        for source in selected:
            for root in source["roots"]:
                ranked.append({
                    "source_id": source["id"],
                    "source_name": source["name"],
                    "title": source["name"] + " documentation root",
                    "url": root,
                    "score": 0,
                })
    return {
        "query": query,
        "retrieved_at": now_iso(),
        "selected_sources": [source["id"] for source in selected],
        "results": ranked[:limit],
        "research_note": "Search results are discovery aids. Fetch and read the official page before using it as evidence.",
    }


def list_sources(topic: str | None = None, provider: str | None = None) -> dict[str, Any]:
    if topic:
        sources = recommend_sources(topic, provider=provider, limit=len(SOURCES))
    else:
        sources = [source for source in SOURCES.values() if not provider or source["provider"] == provider]
    return {
        "catalog_version": CATALOG["version"],
        "count": len(sources),
        "sources": [{
            "id": source["id"],
            "name": source["name"],
            "provider": source["provider"],
            "topics": source["topics"],
            "roots": source["roots"],
        } for source in sources],
    }


def evidence_pack(topic: str, source_ids: list[str] | None = None, document_limit: int = 3, chars_per_document: int = 6_000) -> dict[str, Any]:
    if document_limit < 1 or document_limit > 5:
        raise ValueError("document_limit must be between 1 and 5")
    if chars_per_document < 1_000 or chars_per_document > 12_000:
        raise ValueError("chars_per_document must be between 1000 and 12000")
    search = search_docs(topic, source_ids=source_ids, limit=max(document_limit * 2, 5))
    documents: list[dict[str, Any]] = []
    errors: list[dict[str, str]] = []
    for result in search["results"]:
        if len(documents) >= document_limit:
            break
        try:
            documents.append(fetch_document(result["url"], max_chars=chars_per_document))
        except (ValueError, RuntimeError) as exc:
            errors.append({"url": result["url"], "error": str(exc)})
    return {
        "topic": topic,
        "retrieved_at": now_iso(),
        "documents": documents,
        "fetch_errors": errors,
        "research_note": "Synthesize only claims supported by these pages; distinguish vendor facts from architectural inference.",
    }


TOOLS = [
    {
        "name": "catalog_sources",
        "description": "List cataloged authoritative documentation sources, optionally filtered by topic or provider.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "topic": {"type": "string", "description": "Topic used to rank relevant sources."},
                "provider": {"type": "string", "description": "Exact provider id such as aws, gcp, azure, cncf, or hashicorp."}
            },
            "additionalProperties": False
        }
    },
    {
        "name": "search_official_docs",
        "description": "Search only cataloged official Kubernetes, cloud, IaC, observability, and streaming documentation. Fetch a result before citing it.",
        "inputSchema": {
            "type": "object",
            "required": ["query"],
            "properties": {
                "query": {"type": "string"},
                "source_ids": {"type": "array", "items": {"type": "string"}, "maxItems": 12},
                "limit": {"type": "integer", "minimum": 1, "maximum": 20, "default": 10}
            },
            "additionalProperties": False
        }
    },
    {
        "name": "fetch_official_doc",
        "description": "Retrieve readable text from an allowlisted official documentation URL with source and freshness metadata.",
        "inputSchema": {
            "type": "object",
            "required": ["url"],
            "properties": {
                "url": {"type": "string"},
                "max_chars": {"type": "integer", "minimum": 1000, "maximum": 40000, "default": 18000}
            },
            "additionalProperties": False
        }
    },
    {
        "name": "build_evidence_pack",
        "description": "Search and retrieve a small evidence pack of current official documentation for a technical decision or learning topic.",
        "inputSchema": {
            "type": "object",
            "required": ["topic"],
            "properties": {
                "topic": {"type": "string"},
                "source_ids": {"type": "array", "items": {"type": "string"}, "maxItems": 12},
                "document_limit": {"type": "integer", "minimum": 1, "maximum": 5, "default": 3},
                "chars_per_document": {"type": "integer", "minimum": 1000, "maximum": 12000, "default": 6000}
            },
            "additionalProperties": False
        }
    }
]


def call_tool(name: str, arguments: dict[str, Any]) -> dict[str, Any]:
    if not isinstance(arguments, dict):
        raise TypeError("tool arguments must be an object")
    if name == "catalog_sources":
        result = list_sources(arguments.get("topic"), arguments.get("provider"))
    elif name == "search_official_docs":
        result = search_docs(arguments["query"], arguments.get("source_ids"), arguments.get("limit", 10))
    elif name == "fetch_official_doc":
        result = fetch_document(arguments["url"], arguments.get("max_chars", 18_000))
    elif name == "build_evidence_pack":
        result = evidence_pack(arguments["topic"], arguments.get("source_ids"), arguments.get("document_limit", 3), arguments.get("chars_per_document", 6_000))
    else:
        raise ValueError(f"unknown tool: {name}")
    return {"content": [{"type": "text", "text": json.dumps(result, indent=2, ensure_ascii=False)}], "isError": False}


def success(request_id: Any, result: dict[str, Any]) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": request_id, "result": result}


def failure(request_id: Any, code: int, message: str) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": request_id, "error": {"code": code, "message": message}}


def handle(message: Any) -> dict[str, Any] | None:
    if not isinstance(message, dict):
        return failure(None, -32600, "request must be an object")
    method = message.get("method")
    request_id = message.get("id")
    params = message.get("params", {})
    if params is None:
        params = {}
    if not isinstance(params, dict):
        return failure(request_id, -32602, "params must be an object")
    if method == "initialize":
        requested = params.get("protocolVersion", "2024-11-05")
        return success(request_id, {
            "protocolVersion": requested,
            "capabilities": {"tools": {"listChanged": False}},
            "serverInfo": {"name": "cloud-docs", "version": "0.1.0"}
        })
    if method in {"notifications/initialized", "notifications/cancelled"}:
        return None
    if method == "ping":
        return success(request_id, {})
    if method == "tools/list":
        return success(request_id, {"tools": TOOLS})
    if method == "tools/call":
        try:
            return success(request_id, call_tool(params.get("name", ""), params.get("arguments") or {}))
        except (KeyError, TypeError, ValueError, RuntimeError) as exc:
            return success(request_id, {
                "content": [{"type": "text", "text": str(exc)}],
                "isError": True
            })
    if request_id is None:
        return None
    return failure(request_id, -32601, f"method not found: {method}")


def main() -> None:
    for line in sys.stdin:
        if not line.strip():
            continue
        try:
            message = json.loads(line)
            response = handle(message)
        except (json.JSONDecodeError, TypeError, ValueError) as exc:
            response = failure(None, -32700, str(exc))
        if response is not None:
            sys.stdout.write(json.dumps(response, separators=(",", ":")) + "\n")
            sys.stdout.flush()


if __name__ == "__main__":
    main()
