# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-02-18

### Added
- Initial release of Dynamic Proxy
- REST API for managing virtual host configurations
- DoH (DNS over HTTPS) resolver
- Auto-generated TLS certificate management
- DNS server (UDP/TCP) with custom resolution
- Web UI for managing proxy settings
- Apache httpd integration support
- Support for multiple virtual hosts with different backends

### Features
- Dynamic virtual host routing
- Automatic TLS certificate generation and management
- DNS-based host resolution with upstream fallback
- JSON-based configuration API
- Web interface for configuration management
- Support for HTTPS proxy with self-signed certificates

### Technical Details
- Built with Go 1.21
- Uses gorilla/mux for HTTP routing
- Uses miekg/dns for DNS functionality
- Cross-platform compatible

## Planned Features (Future)

- [ ] Persistent configuration storage
- [ ] Load balancing across multiple backends
- [ ] Request/response middleware support
- [ ] Advanced monitoring and logging
- [ ] Configuration file versioning
- [ ] API authentication and authorization
