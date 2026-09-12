"""Fetch all initial JS/CSS in parallel and reject truncated HTTP responses."""

import argparse
from concurrent.futures import ThreadPoolExecutor
from html.parser import HTMLParser
import json
import time
from urllib.parse import urljoin, urlsplit
from urllib.request import ProxyHandler, Request, build_opener


class AssetParser(HTMLParser):
    def __init__(self):
        super().__init__()
        self.paths = []

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if tag == "script" and attrs.get("src"):
            self.paths.append(attrs["src"])
        if tag == "link" and attrs.get("rel") in ("modulepreload", "stylesheet"):
            if attrs.get("href"):
                self.paths.append(attrs["href"])


def check_assets(base_url):
    # Check the deployment directly; do not send private host requests to an HTTP proxy.
    opener = build_opener(ProxyHandler({}))
    with opener.open(base_url, timeout=15) as response:
        parser = AssetParser()
        parser.feed(response.read().decode("utf-8"))
    origin = urlsplit(base_url)
    urls = sorted({
        urljoin(base_url, path) for path in parser.paths
        if urlsplit(urljoin(base_url, path))[:2] == origin[:2]
    })
    if not urls:
        raise RuntimeError("No same-origin scripts or stylesheets found in the page")

    def fetch(url):
        started = time.monotonic()
        request = Request(url, headers={"Accept-Encoding": "identity", "Cache-Control": "no-cache"})
        # A separate opener keeps concurrent requests independent, as in a cold browser load.
        with build_opener(ProxyHandler({})).open(request, timeout=90) as response:
            if response.status != 200:
                raise RuntimeError(f"Unexpected HTTP status {response.status} for {url}")
            content = response.read()  # IncompleteRead is a test failure, never silently retried.
            length = response.headers.get("Content-Length")
            if length is not None and len(content) != int(length):
                raise RuntimeError(f"Incomplete asset: {url}")
            if not content:
                raise RuntimeError(f"Empty asset: {url}")
        return {"asset": urlsplit(url).path, "bytes": len(content), "seconds": round(time.monotonic() - started, 3)}

    with ThreadPoolExecutor(max_workers=6) as pool:
        for result in pool.map(fetch, urls):
            print(json.dumps(result), flush=True)
    print(f"PASS: all {len(urls)} initial assets arrived in full", flush=True)


if __name__ == "__main__":
    arguments = argparse.ArgumentParser(description=__doc__)
    arguments.add_argument("url", help="Deployed page URL, including any path prefix")
    check_assets(arguments.parse_args().url)
