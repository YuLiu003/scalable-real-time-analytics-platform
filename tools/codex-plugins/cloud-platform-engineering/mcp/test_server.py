#!/usr/bin/env python3

import io
import json
import os
import runpy
import unittest
from unittest import mock
from urllib.error import URLError

import server


class FakeHeaders:
    def __init__(self, content_type: str = "text/html", charset: str | None = None) -> None:
        self.content_type = content_type
        self.charset = charset

    def get_content_type(self) -> str:
        return self.content_type

    def get_content_charset(self) -> str | None:
        return self.charset


class FakeResponse:
    def __init__(
        self,
        body: bytes | str,
        url: str,
        content_type: str = "text/html",
        charset: str | None = None,
    ) -> None:
        self.body = body.encode(charset or "utf-8") if isinstance(body, str) else body
        self.url = url
        self.headers = FakeHeaders(content_type, charset)

    def __enter__(self) -> "FakeResponse":
        return self

    def __exit__(self, *_args: object) -> None:
        return None

    def geturl(self) -> str:
        return self.url

    def read(self, _limit: int) -> bytes:
        return self.body


class CloudDocsServerTests(unittest.TestCase):
    def setUp(self) -> None:
        server.SITEMAP_CACHE.clear()

    def test_catalog_is_unique_and_broad(self) -> None:
        source_ids = list(server.SOURCES)
        self.assertEqual(len(source_ids), len(set(source_ids)))
        self.assertGreaterEqual(len(source_ids), 30)
        for required in {"kubernetes", "aws", "gcp", "azure", "terraform", "kafka", "opentelemetry"}:
            self.assertIn(required, server.SOURCES)

    def test_document_parser_extracts_content_links_and_ignored_elements(self) -> None:
        parser = server.DocumentParser()
        parser.handle_endtag("style")
        parser.handle_endtag("main")
        parser._ignored = 1
        parser.handle_starttag("div", [])
        parser.handle_endtag("div")
        parser._ignored = 0
        parser.feed(
            "<html><head><title> Cloud  Doc </title><style>hidden</style></head>"
            "<body><nav>Noise</nav><script><span>ignored</span></script>"
            "<main><h1>Useful</h1><a href='/guide'> Guide </a><a>no href</a>"
            "<article><p>Evidence</p></article><form>ignored form</form></main></body></html>"
        )
        parser.handle_data("   ")

        self.assertEqual(parser.title, "Cloud Doc")
        self.assertEqual(parser.text, "Useful\nGuide\nno href\nEvidence")
        self.assertEqual(parser.links, [("Guide", "/guide")])

        fallback = server.DocumentParser()
        fallback.feed("<div>All content</div>")
        self.assertEqual(fallback.text, "All content")

    def test_url_allowlist_and_source_resolution(self) -> None:
        self.assertIsNone(server.source_for_url("http://kubernetes.io/docs/"))
        self.assertIsNone(server.source_for_url("https:///missing-host"))
        self.assertIsNone(server.source_for_url("https://example.com/kubernetes"))
        self.assertEqual(server.source_for_url("https://docs.aws.amazon.com/")["id"], "aws")
        self.assertEqual(
            server.source_for_url("https://docs.aws.amazon.com/eks/latest/userguide/what-is-eks.html")["id"],
            "aws-eks",
        )
        self.assertEqual(server.source_for_url("https://subdomain.kubernetes.io/docs/")["id"], "kubernetes")
        self.assertEqual(server.require_official_url("https://developer.hashicorp.com/vault/docs")["id"], "vault")
        with self.assertRaisesRegex(ValueError, "cataloged official"):
            server.require_official_url("https://example.com/")

    def test_read_response_enforces_download_limit(self) -> None:
        response = FakeResponse(b"small", "https://kubernetes.io/docs/")
        self.assertEqual(server.read_response(response), b"small")

        oversized = FakeResponse(b"x" * (server.MAX_DOWNLOAD_BYTES + 1), "https://kubernetes.io/docs/")
        with self.assertRaisesRegex(ValueError, "safety limit"):
            server.read_response(oversized)

    def test_official_opener_validates_initial_and_redirect_urls(self) -> None:
        request = server.Request("https://kubernetes.io/docs/")
        response = FakeResponse("ok", request.full_url)
        opener = mock.Mock()
        opener.open.return_value = response
        with mock.patch.object(server, "build_opener", return_value=opener) as build:
            self.assertIs(server.open_official(request), response)
        self.assertIsInstance(build.call_args.args[0], server.OfficialRedirectHandler)
        opener.open.assert_called_once_with(request, timeout=server.DEFAULT_TIMEOUT_SECONDS)

        with self.assertRaisesRegex(ValueError, "cataloged official"):
            server.open_official(server.Request("https://example.com/"))

        handler = server.OfficialRedirectHandler()
        with mock.patch.object(
            server.HTTPRedirectHandler,
            "redirect_request",
            return_value=request,
        ) as parent:
            self.assertIs(
                handler.redirect_request(
                    request,
                    None,
                    302,
                    "Found",
                    {},
                    "https://kubernetes.io/docs/concepts/",
                ),
                request,
            )
        parent.assert_called_once()
        with self.assertRaisesRegex(ValueError, "cataloged official"):
            handler.redirect_request(request, None, 302, "Found", {}, "http://127.0.0.1/")

    def test_fetch_document_parses_html_and_truncates(self) -> None:
        response = FakeResponse(
            "<html><head><title>Service</title></head><main>A &amp; B\n\n\n\nSecond line that is long</main></html>",
            "https://kubernetes.io/docs/concepts/services-networking/service/",
            charset="utf-8",
        )
        with mock.patch.object(server, "open_official", return_value=response):
            document = server.fetch_document(response.url, 1_010)

        self.assertEqual(document["source_id"], "kubernetes")
        self.assertEqual(document["title"], "Service")
        self.assertEqual(document["text"], "A & B Second line that is long")
        self.assertFalse(document["truncated"])
        self.assertIn("retrieved_at", document)

        long_body = "<main>" + ("word\n" * 400) + "</main>"
        with mock.patch.object(
            server,
            "open_official",
            return_value=FakeResponse(long_body, "https://kubernetes.io/docs/"),
        ):
            truncated = server.fetch_document("https://kubernetes.io/docs/", 1_000)
        self.assertTrue(truncated["truncated"])
        self.assertLessEqual(len(truncated["text"]), 1_000)

    def test_fetch_document_handles_fallback_types_and_failures(self) -> None:
        cases = [
            ("application/xhtml+xml", "<main>HTML</main>", "Kubernetes Documentation"),
            ("text/plain", "plain", "Kubernetes Documentation"),
            ("application/json", '{"ok": true}', "Kubernetes Documentation"),
            ("application/xml", "<root />", "Kubernetes Documentation"),
        ]
        for content_type, body, expected_title in cases:
            with self.subTest(content_type=content_type), mock.patch.object(
                server,
                "open_official",
                return_value=FakeResponse(body, "https://kubernetes.io/docs/", content_type),
            ):
                document = server.fetch_document("https://kubernetes.io/docs/")
                self.assertEqual(document["title"], expected_title)

        with mock.patch.object(
            server,
            "open_official",
            return_value=FakeResponse(b"\x00\x01", "https://kubernetes.io/docs/", "application/octet-stream"),
        ):
            with self.assertRaisesRegex(ValueError, "unsupported content type"):
                server.fetch_document("https://kubernetes.io/docs/")

        with mock.patch.object(server, "open_official", side_effect=URLError("offline")):
            with self.assertRaisesRegex(RuntimeError, "failed to fetch"):
                server.fetch_document("https://kubernetes.io/docs/")

        with mock.patch.object(
            server,
            "open_official",
            return_value=FakeResponse("redirect", "https://example.com/", "text/plain"),
        ):
            with self.assertRaisesRegex(ValueError, "cataloged official"):
                server.fetch_document("https://kubernetes.io/docs/")

        for invalid in (999, 40_001):
            with self.subTest(max_chars=invalid), self.assertRaisesRegex(ValueError, "max_chars"):
                server.fetch_document("https://kubernetes.io/docs/", invalid)

    def test_tokenization_scoring_and_titles(self) -> None:
        self.assertEqual(server.tokenize("A pod/service: v1+beta"), {"pod", "service", "v1+beta"})
        self.assertEqual(server.title_from_url("https://kubernetes.io"), "kubernetes.io")
        self.assertEqual(
            server.title_from_url("https://kubernetes.io/docs/latest/network-policy/reference_page/"),
            "network policy / reference page",
        )
        self.assertEqual(server.title_from_url("not-a-url"), "not a url")
        self.assertGreater(server.score_result("network policy", "Network Policy", "https://example/network-policy"), 0)
        self.assertEqual(server.score_result("unrelated", "Network Policy", "https://example/network-policy"), 0)
        self.assertTrue(server.preferred_language_url("https://kubernetes.io/docs/"))
        self.assertFalse(server.preferred_language_url("https://kubernetes.io/fr/docs/"))

    def test_recommend_sources_filters_and_falls_back(self) -> None:
        cases = {
            "Cloud SQL backups": "gcp",
            "Azure Service Bus reliability": "azure",
            "S3 encryption": "aws",
            "Gateway API routing": "gateway-api",
            "PostgreSQL replication": "postgresql",
            "OPA policy": "opa",
        }
        for query, expected in cases.items():
            with self.subTest(query=query):
                self.assertEqual(server.recommend_sources(query, limit=3)[0]["id"], expected)

        self.assertTrue(server.recommend_sources("pod deployment", limit=3))
        defaults = server.recommend_sources("zzzz-no-match")
        self.assertEqual(defaults[0]["id"], "aws")
        self.assertTrue(all(source["provider"] == "aws" for source in server.recommend_sources("zzzz", "aws")))
        self.assertEqual(server.recommend_sources("zzzz", "missing-provider"), [])
        self.assertEqual(len(server.recommend_sources("cloud", limit=1)), 1)

    def test_resolve_source_ids(self) -> None:
        self.assertEqual(
            [source["id"] for source in server.resolve_source_ids(["kubernetes", "aws"], "ignored")],
            ["kubernetes", "aws"],
        )
        with self.assertRaisesRegex(ValueError, "missing, unknown"):
            server.resolve_source_ids(["unknown", "missing"], "ignored")
        self.assertTrue(server.resolve_source_ids(None, "kubernetes"))

    def test_decode_search_redirect(self) -> None:
        target = "https://kubernetes.io/docs/"
        redirect = "https://duckduckgo.com/l/?uddg=https%3A%2F%2Fkubernetes.io%2Fdocs%2F"
        self.assertEqual(server.decode_search_redirect(redirect), target)
        no_target = "https://duckduckgo.com/l/?q=kubernetes"
        self.assertEqual(server.decode_search_redirect(no_target), no_target)
        self.assertEqual(server.decode_search_redirect(target), target)
        self.assertEqual(server.decode_search_redirect("/relative"), "/relative")

    def test_search_group_extracts_only_official_labeled_links(self) -> None:
        body = """
        <main>
          <a href="https://kubernetes.io/docs/">Kubernetes Guide</a>
          <a href="https://example.com/">Untrusted</a>
          <a href="https://kubernetes.io/"></a>
          <a href="https://duckduckgo.com/l/?uddg=https%3A%2F%2Fkubernetes.io%2Fdocs%2Fconcepts%2F">Concepts</a>
        </main>
        """
        with mock.patch.object(
            server,
            "urlopen",
            return_value=FakeResponse(body, "https://html.duckduckgo.com/html/"),
        ):
            results = server.search_group("kubernetes", ["kubernetes.io"])
        self.assertEqual([result["title"] for result in results], ["Kubernetes Guide", "Concepts"])

        with mock.patch.object(server, "urlopen", side_effect=TimeoutError):
            self.assertEqual(server.search_group("kubernetes", ["kubernetes.io"]), [])

    def test_fetch_sitemap_locations_handles_cache_indexes_and_errors(self) -> None:
        sitemap_url = "https://kubernetes.io/sitemap.xml"
        urlset = """
        <urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
          <url><loc>https://kubernetes.io/docs/</loc></url>
          <url><loc>https://example.com/</loc></url>
          <url><loc></loc></url>
        </urlset>
        """
        with mock.patch.object(
            server,
            "open_official",
            return_value=FakeResponse(urlset, sitemap_url, "application/xml"),
        ):
            self.assertEqual(server.fetch_sitemap_locations(sitemap_url), ["https://kubernetes.io/docs/"])
            self.assertEqual(server.fetch_sitemap_locations(sitemap_url), ["https://kubernetes.io/docs/"])

        server.SITEMAP_CACHE.clear()
        index = """
        <sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
          <sitemap><loc>https://kubernetes.io/child.xml</loc></sitemap>
          <sitemap><loc>https://kubernetes.io/not-expanded</loc></sitemap>
        </sitemapindex>
        """
        original = server.fetch_sitemap_locations
        with mock.patch.object(
            server,
            "fetch_sitemap_locations",
            return_value=["https://kubernetes.io/docs/child/"],
        ) as recursive:
            with mock.patch.object(
                server,
                "open_official",
                return_value=FakeResponse(index, sitemap_url, "application/xml"),
            ):
                locations = original(sitemap_url)
        self.assertEqual(locations, ["https://kubernetes.io/docs/child/"])
        recursive.assert_called_once_with("https://kubernetes.io/child.xml", 1)

        server.SITEMAP_CACHE.clear()
        with mock.patch.object(
            server,
            "open_official",
            return_value=FakeResponse(index, sitemap_url, "application/xml"),
        ):
            depth_limited = server.fetch_sitemap_locations(sitemap_url, depth=1)
        self.assertEqual(depth_limited, ["https://kubernetes.io/child.xml", "https://kubernetes.io/not-expanded"])

        with self.assertRaises(ValueError):
            server.fetch_sitemap_locations("https://example.com/sitemap.xml")
        for response in (
            URLError("offline"),
            FakeResponse("<invalid", sitemap_url, "application/xml"),
            FakeResponse("x", "https://example.com/", "application/xml"),
            FakeResponse(b"x" * (server.MAX_DOWNLOAD_BYTES + 1), sitemap_url, "application/xml"),
        ):
            server.SITEMAP_CACHE.clear()
            context = (
                mock.patch.object(server, "open_official", side_effect=response)
                if isinstance(response, Exception)
                else mock.patch.object(server, "open_official", return_value=response)
            )
            with context:
                self.assertEqual(server.fetch_sitemap_locations(sitemap_url), [])

    def test_search_sitemaps_filters_language_and_relevance(self) -> None:
        selected = [
            server.SOURCES["kubernetes"],
            {"id": "none", "sitemaps": [], "domains": [], "roots": []},
        ]
        urls = [
            "https://kubernetes.io/fr/docs/pods/",
            "https://kubernetes.io/docs/unrelated/",
            "https://kubernetes.io/docs/pods/",
        ]
        with mock.patch.object(server, "fetch_sitemap_locations", return_value=urls):
            results = server.search_sitemaps("pods", selected)
        self.assertEqual(results, [{"title": "pods", "url": "https://kubernetes.io/docs/pods/"}])

    def test_search_root_links_filters_sources_and_scores(self) -> None:
        body = """
        <main>
          <a href="/docs/pods/">Pods</a>
          <a href="/docs/unrelated/">Nothing Relevant</a>
          <a href="https://example.com/">Pods external</a>
          <a href="https://learn.microsoft.com/en-us/azure/aks/">Pods elsewhere</a>
          <a href="/docs/pods/reference"></a>
        </main>
        """
        response = FakeResponse(body, "https://kubernetes.io/docs/")
        with mock.patch.object(server, "open_official", return_value=response):
            results = server.search_root_links("pods", [server.SOURCES["kubernetes"]])
        self.assertEqual(len(results), 2)
        self.assertEqual(results[0]["title"], "Pods")
        self.assertEqual(results[1]["title"], "pods / reference")

        with mock.patch.object(server, "open_official", side_effect=URLError("offline")):
            self.assertEqual(server.search_root_links("pods", [server.SOURCES["kubernetes"]]), [])

        with mock.patch.object(
            server,
            "open_official",
            return_value=FakeResponse(body, "https://example.com/"),
        ):
            self.assertEqual(server.search_root_links("pods", [server.SOURCES["kubernetes"]]), [])

    def test_search_docs_validates_and_ranks_results(self) -> None:
        for query in ("x", "x" * 301):
            with self.subTest(query_length=len(query)), self.assertRaisesRegex(ValueError, "2 to 300"):
                server.search_docs(query)
        for limit in (0, 21):
            with self.subTest(limit=limit), self.assertRaisesRegex(ValueError, "1 and 20"):
                server.search_docs("pods", limit=limit)

        selected = [server.SOURCES["kubernetes"]]
        raw = [
            {"title": "Pods", "url": "https://kubernetes.io/docs/pods/"},
            {"title": "Duplicate", "url": "https://kubernetes.io/docs/pods/"},
            {"title": "Untrusted", "url": "https://example.com/"},
            {"title": "Other source", "url": "https://learn.microsoft.com/en-us/azure/aks/"},
        ]
        with (
            mock.patch.object(server, "resolve_source_ids", return_value=selected),
            mock.patch.object(server, "search_group", return_value=raw),
            mock.patch.object(server, "search_sitemaps") as sitemaps,
            mock.patch.object(server, "search_root_links") as roots,
        ):
            result = server.search_docs("  pods   ", limit=1)
        self.assertEqual(result["query"], "pods")
        self.assertEqual(len(result["results"]), 1)
        self.assertEqual(result["results"][0]["title"], "Pods")
        sitemaps.assert_not_called()
        roots.assert_not_called()

    def test_search_docs_uses_fallback_searches_and_roots(self) -> None:
        selected = [server.SOURCES["kubernetes"]]
        with (
            mock.patch.object(server, "resolve_source_ids", return_value=selected),
            mock.patch.object(server, "search_group", return_value=[]),
            mock.patch.object(server, "search_sitemaps", return_value=[]),
            mock.patch.object(server, "search_root_links", return_value=[]),
        ):
            result = server.search_docs("pods", limit=2)
        self.assertEqual(result["results"][0]["score"], 0)
        self.assertEqual(result["results"][0]["url"], "https://kubernetes.io/docs/")

        many_domains = [{**server.SOURCES["kubernetes"], "domains": [f"d{i}.example" for i in range(5)]}]
        with (
            mock.patch.object(server, "resolve_source_ids", return_value=many_domains),
            mock.patch.object(server, "search_group", return_value=[]) as groups,
            mock.patch.object(server, "search_sitemaps", return_value=[]),
            mock.patch.object(server, "search_root_links", return_value=[]),
        ):
            server.search_docs("pods", limit=2)
        self.assertEqual(groups.call_count, 2)

    def test_list_sources(self) -> None:
        all_sources = server.list_sources()
        self.assertEqual(all_sources["count"], len(server.SOURCES))
        aws_sources = server.list_sources(provider="aws")
        self.assertTrue(aws_sources["sources"])
        self.assertTrue(all(item["provider"] == "aws" for item in aws_sources["sources"]))
        topic_sources = server.list_sources(topic="Kafka", provider="apache")
        self.assertEqual(topic_sources["sources"][0]["id"], "kafka")

    def test_evidence_pack_validates_collects_errors_and_stops_at_limit(self) -> None:
        for limit in (0, 6):
            with self.subTest(document_limit=limit), self.assertRaisesRegex(ValueError, "document_limit"):
                server.evidence_pack("pods", document_limit=limit)
        for chars in (999, 12_001):
            with self.subTest(chars=chars), self.assertRaisesRegex(ValueError, "chars_per_document"):
                server.evidence_pack("pods", chars_per_document=chars)

        results = {
            "results": [
                {"url": "https://kubernetes.io/docs/fail/"},
                {"url": "https://kubernetes.io/docs/ok/"},
                {"url": "https://kubernetes.io/docs/not-fetched/"},
            ]
        }
        document = {"url": "https://kubernetes.io/docs/ok/"}
        with (
            mock.patch.object(server, "search_docs", return_value=results) as search,
            mock.patch.object(server, "fetch_document", side_effect=[RuntimeError("offline"), document]) as fetch,
        ):
            pack = server.evidence_pack("pods", document_limit=1)
        self.assertEqual(pack["documents"], [document])
        self.assertEqual(pack["fetch_errors"][0]["error"], "offline")
        self.assertEqual(fetch.call_count, 2)
        search.assert_called_once_with("pods", source_ids=None, limit=5)

        with mock.patch.object(server, "search_docs", return_value={"results": []}):
            empty = server.evidence_pack("pods")
        self.assertEqual(empty["documents"], [])

    def test_call_tool_routes_and_serializes(self) -> None:
        routes = [
            ("catalog_sources", {}, "list_sources"),
            ("search_official_docs", {"query": "pods"}, "search_docs"),
            ("fetch_official_doc", {"url": "https://kubernetes.io/docs/"}, "fetch_document"),
            ("build_evidence_pack", {"topic": "pods"}, "evidence_pack"),
        ]
        for name, arguments, target in routes:
            with self.subTest(name=name), mock.patch.object(server, target, return_value={"route": target}):
                response = server.call_tool(name, arguments)
                self.assertFalse(response["isError"])
                self.assertEqual(json.loads(response["content"][0]["text"]), {"route": target})

        with self.assertRaisesRegex(TypeError, "arguments"):
            server.call_tool("catalog_sources", [])
        with self.assertRaisesRegex(ValueError, "unknown tool"):
            server.call_tool("unknown", {})

    def test_protocol_helpers_and_handle(self) -> None:
        self.assertEqual(server.success(1, {"ok": True})["result"], {"ok": True})
        self.assertEqual(server.failure(1, -1, "bad")["error"]["message"], "bad")

        invalid_request = server.handle([])
        self.assertEqual(invalid_request["error"]["code"], -32600)
        invalid_params = server.handle({"id": 1, "method": "tools/call", "params": []})
        self.assertEqual(invalid_params["error"]["code"], -32602)

        initialized = server.handle({"id": 1, "method": "initialize"})
        self.assertEqual(initialized["result"]["protocolVersion"], "2024-11-05")
        requested = server.handle(
            {"id": 2, "method": "initialize", "params": {"protocolVersion": "custom"}}
        )
        self.assertEqual(requested["result"]["protocolVersion"], "custom")
        self.assertEqual(server.handle({"id": 2, "method": "ping", "params": None})["result"], {})
        self.assertIsNone(server.handle({"method": "notifications/initialized"}))
        self.assertIsNone(server.handle({"method": "notifications/cancelled"}))
        self.assertEqual(server.handle({"id": 3, "method": "ping"})["result"], {})
        self.assertEqual(
            [tool["name"] for tool in server.handle({"id": 4, "method": "tools/list"})["result"]["tools"]],
            ["catalog_sources", "search_official_docs", "fetch_official_doc", "build_evidence_pack"],
        )

        with mock.patch.object(server, "call_tool", return_value={"content": [], "isError": False}):
            called = server.handle(
                {"id": 5, "method": "tools/call", "params": {"name": "catalog_sources"}}
            )
        self.assertFalse(called["result"]["isError"])

        errored = server.handle({"id": 6, "method": "tools/call", "params": {"name": "unknown"}})
        self.assertTrue(errored["result"]["isError"])
        self.assertIsNone(server.handle({"method": "unknown-notification"}))
        self.assertEqual(server.handle({"id": 7, "method": "unknown"})["error"]["code"], -32601)

    def test_main_processes_json_lines_and_entrypoint(self) -> None:
        stdin = io.StringIO(
            "\n"
            '{"jsonrpc":"2.0","id":1,"method":"ping"}\n'
            '{"jsonrpc":"2.0","method":"notifications/initialized"}\n'
            "not json\n"
            "[]\n"
        )
        stdout = io.StringIO()
        with mock.patch.object(server.sys, "stdin", stdin), mock.patch.object(server.sys, "stdout", stdout):
            server.main()
        responses = [json.loads(line) for line in stdout.getvalue().splitlines()]
        self.assertEqual(len(responses), 3)
        self.assertEqual(responses[0]["result"], {})
        self.assertEqual(responses[1]["error"]["code"], -32700)
        self.assertEqual(responses[2]["error"]["code"], -32600)

        entrypoint_output = io.StringIO()
        with mock.patch("sys.stdin", io.StringIO("")), mock.patch("sys.stdout", entrypoint_output):
            runpy.run_path(server.__file__, run_name="__main__")
        self.assertEqual(entrypoint_output.getvalue(), "")

    @unittest.skipUnless(os.environ.get("RUN_CLOUD_DOCS_NETWORK_TESTS") == "1", "network test is opt-in")
    def test_live_official_document_fetch(self) -> None:
        document = server.fetch_document(
            "https://kubernetes.io/docs/concepts/services-networking/service/",
            2_000,
        )
        self.assertEqual(document["source_id"], "kubernetes")
        self.assertIn("Service", document["title"])
        self.assertGreater(len(document["text"]), 500)


if __name__ == "__main__":
    unittest.main()
