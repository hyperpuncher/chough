# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.0.0] - 2026-03-08

### Changed

- First stable release.

## [0.2.7] - 2026-03-07

### Added

- Stdin pipe support (`cat audio.wav | chough`).

### Changed

- Updated dependencies.

## [0.2.6] - 2026-03-05

### Changed

- Split output handling into a separate internal package.
- Deduplicated functions across codebase.

### Fixed

- Winget manifest missing FFmpeg dependency.

## [0.2.5] - 2026-03-04

### Fixed

- Nil pointer panic when sherpa-onnx returns empty result.

## [0.2.4] - 2026-03-03

### Added

- Remote transcription via `CHOUGH_URL` environment variable.

## [0.2.3] - 2026-03-03

### Fixed

- Docker image missing sherpa runtime libraries.

## [0.2.2] - 2026-03-03

### Fixed

- Docker extra_files configuration.

## [0.2.1] - 2026-03-03

### Added

- HTTP server mode (`chough server`).
- Docker images published to GHCR.

### Fixed

- Docker binary and library paths.

## [0.1.12] - 2026-03-03

### Fixed

- Windows Winget installation via shim launcher.

## [0.1.11] - 2026-03-01

### Added

- Supported languages documentation.
- CPU requirements documentation.

### Changed

- Refactored CLI flow and progress pipeline.

## [0.1.10] - 2026-02-28

### Added

- Winget publishing with auto-PR to microsoft/winget-pkgs.

## [0.1.9] - 2026-02-28

### Added

- AUR publishing for Arch Linux.

## [0.1.8] - 2026-02-27

### Fixed

- macOS quarantine attributes on bundled dylibs.

## [0.1.7] - 2026-02-26

### Added

- Homebrew cask in dedicated tap repo.

## [0.1.6] - 2026-02-26

### Fixed

- macOS loader paths in cross-compiled builds.

## [0.1.5] - 2026-02-26

### Changed

- Migrated to goreleaser-cross for releases.

## [0.1.4] - 2026-02-25

### Fixed

- Linux library path resolution.

## [0.1.3] - 2026-02-25

### Fixed

- Bundle all dylibs for macOS (was partial in v0.1.2) and fix rpaths.

## [0.1.2] - 2026-02-25

### Added

- macOS dylib bundling.

### Fixed

- Windows DLL path for sherpa-onnx v1.12.26.

## [0.1.1] - 2026-02-25

### Added

- Support for video files as input (audio is extracted automatically).
- Windows DLL bundling.

### Changed

- Progress bar design with ETA.
- Linux build simplification.

## [0.1.0] - 2026-02-24

### Added

- Initial release.
- Audio transcription via Parakeet TDT 0.6b V3.
- Output formats: text, JSON, VTT.
- Chunked processing for memory efficiency.
- Linux, macOS (Intel/Apple Silicon), and Windows support.

[Unreleased]: https://github.com/hyperpuncher/chough/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/hyperpuncher/chough/compare/v0.2.7...v1.0.0
[0.2.7]: https://github.com/hyperpuncher/chough/compare/v0.2.6...v0.2.7
[0.2.6]: https://github.com/hyperpuncher/chough/compare/v0.2.5...v0.2.6
[0.2.5]: https://github.com/hyperpuncher/chough/compare/v0.2.4...v0.2.5
[0.2.4]: https://github.com/hyperpuncher/chough/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/hyperpuncher/chough/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/hyperpuncher/chough/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/hyperpuncher/chough/compare/v0.1.12...v0.2.1
[0.1.12]: https://github.com/hyperpuncher/chough/compare/v0.1.11...v0.1.12
[0.1.11]: https://github.com/hyperpuncher/chough/compare/v0.1.10...v0.1.11
[0.1.10]: https://github.com/hyperpuncher/chough/compare/v0.1.9...v0.1.10
[0.1.9]: https://github.com/hyperpuncher/chough/compare/v0.1.8...v0.1.9
[0.1.8]: https://github.com/hyperpuncher/chough/compare/v0.1.7...v0.1.8
[0.1.7]: https://github.com/hyperpuncher/chough/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/hyperpuncher/chough/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/hyperpuncher/chough/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/hyperpuncher/chough/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/hyperpuncher/chough/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/hyperpuncher/chough/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/hyperpuncher/chough/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/hyperpuncher/chough/releases/tag/v0.1.0
