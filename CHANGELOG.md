# Changelog

## 1.3.0

- Release Explorer as an independent service with its own browser API, public query index and React frontend. Protocol RPC supplies chain facts, Provenance supplies recognition evidence and Broadcaster supplies optional public service information.
- Preserve independent execution and recognition states, public Note/Credit views, synchronization diagnostics and explicit upstream scope configuration. Explorer holds no upstream signing keys or private submission journals.
- Publish Linux amd64/arm64 container images and Linux/macOS amd64/arm64 CLI archives containing the built frontend, documentation, Apache-2.0 license and SHA-256 checksums.
- Validate clean-source builds and smoke-test release archives and container static assets before publication. Existing Provenance databases, issuer identities, signing keys and published history stay with Provenance during the Explorer cutover.
