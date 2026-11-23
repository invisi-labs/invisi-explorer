#!/usr/bin/env python3
"""Verify a running release serves its entry, bundled assets, logo and fonts."""
import re
import sys
import time
import urllib.error
import urllib.request


def check(base):
    def fetch(path):
        with urllib.request.urlopen(base + path, timeout=5) as response:
            assert response.status == 200, path
            return response.read()

    for attempt in range(60):
        try:
            assert b'"ok"' in fetch('/healthz')
            break
        except (OSError, urllib.error.URLError):
            if attempt == 59:
                raise
            time.sleep(0.5)
    index = fetch('/').decode()
    assert '<div id="root">' in index
    assets = re.findall(r'(?:src|href)="(/assets/[^"\s]+)"', index)
    assert assets, 'entry has no bundled assets'
    for asset in assets:
        assert fetch(asset), asset
    assert b'<svg' in fetch('/assets/invisipay-mark.svg')
    assert fetch('/fonts/jetbrains-mono-latin.woff2')[:4] == b'wOF2'
    assert fetch('/diagnostics') == index.encode(), 'SPA route failed'


if __name__ == '__main__':
    check(sys.argv[1].rstrip('/'))
