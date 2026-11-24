#!/usr/bin/env python3
"""Check every archive; execute the native one and serve its bundled UI."""
import hashlib
import importlib.util
from pathlib import Path
import platform
import socket
import subprocess
import sys
import tarfile
import tempfile

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location('smoke_http', Path(__file__).with_name('smoke-http.py'))
http = importlib.util.module_from_spec(spec)
spec.loader.exec_module(http)
tag, output = sys.argv[1], Path(sys.argv[2]).resolve()
checksums = dict(line.split()[::-1] for line in (output / f'SHA256SUMS-{tag}.txt').read_text().splitlines())
expected = {f'invisi-explorer_{tag}_{os}_{arch}.tar.gz' for os in ('linux', 'darwin') for arch in ('amd64', 'arm64')}
assert set(checksums) == expected, 'expected exactly four supported platform archives'
host = platform.system().lower() + '_' + {'x86_64': 'amd64', 'aarch64': 'arm64', 'arm64': 'arm64'}[platform.machine()]
for name in sorted(expected):
    archive = output / name
    assert hashlib.sha256(archive.read_bytes()).hexdigest() == checksums[name], name
    with tempfile.TemporaryDirectory() as tmp:
        with tarfile.open(archive) as tar:
            tar.extractall(tmp, filter='data')
        root = Path(tmp) / name.removesuffix('.tar.gz')
        assert (root / 'LICENSE').read_bytes() == Path('LICENSE').read_bytes()
        for asset in ('index.html', 'assets/invisipay-mark.svg', 'fonts/jetbrains-mono-latin.woff2', 'assets/networks/LICENSE'):
            assert (root / 'web/dist' / asset).stat().st_size, asset
        for document in ('README.md', 'api.md', 'cammand.md', 'CHANGELOG.md'):
            assert (root / document).is_file(), document
        if not name.endswith(host + '.tar.gz'):
            continue
        binary = str(root / 'invisi-explorer')
        assert tag[1:] in subprocess.check_output([binary, '--version'], text=True)
        data = str(Path(tmp) / 'data')
        subprocess.run([binary, '--dir', data, 'init'], check=True)
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            port = sock.getsockname()[1]
        process = subprocess.Popen([binary, '--dir', data, 'run', '--listen', f'127.0.0.1:{port}'], cwd=root)
        try:
            http.check(f'http://127.0.0.1:{port}')
        finally:
            process.terminate()
            process.wait(timeout=15)
print('Verified four archives, checksums, licenses, and native CLI/static HTTP serving')
