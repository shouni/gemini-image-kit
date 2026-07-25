# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Go library for image generation via Gemini / Vertex AI, built on top of `github.com/shouni/go-gemini-client`. It adds what the raw client lacks: fetching reference images from GCS/HTTP, Gemini File API upload with cache-backed lifecycle management, in-memory JPEG compression, and single/multi reference-image generation requests. No main package.

## Commands

```sh
go build ./...
go vet ./...
go test -race ./...                              # CI runs tests with -race
go test ./generator/ -run TestGenerateSingle     # single test
test -z "$(gofmt -l .)"                          # CI fails on unformatted code
golangci-lint run                                # CI uses v2.12.2; config in .golangci.yml
```

## Architecture

Three packages in a ports-and-adapters layout:

- **`ports/`** — interfaces and request/response types only. Consumers inject implementations of `ContentReader` (GCS-style `gs://` access), `Downloader` (HTTP fetch — SSRF protection / domain allow-listing is deliberately the caller's responsibility), and `ImageCacher`. `ImageGenerator` is the public entry interface; `ImageExecutor`/`AssetManager` are the internal seams.
- **`generator/`** — two layers:
  - `GeminiImageCore` (low-level): implements `ImageExecutor` + `AssetManager` against `gemini.GenerativeModel`. Handles fetch → MIME detect → optional compress → upload/inline-part, and parses Gemini responses into `ports.ImageResponse`.
  - `GeminiGenerator` (high-level): depends only on `ports.ImageExecutor` (tests mock it), assembles parts + prompt + options and delegates. `GenerateSingleImage`/`GenerateFusedImage` both funnel into the private `generate`.
- **`imgutil/`** — stateless MIME detection/guessing and PNG/GIF→JPEG compression helpers.

### Behavior that spans files (worth knowing before editing)

- **Image part resolution priority** (`gemini_helper.go: resolveImagePart`): Vertex AI + `gs://` URI → direct `FileData` reference (no transfer); else `FileAPIURI` if set → `FileData`; else download `ReferenceURL` and inline the bytes; empty `ImageURI` is silently skipped. The text prompt is always appended **after** all image parts.
- **File API cache contract** (`core.go`/`core_helper.go`): `EnsureUploaded` caches a single `cachedFile{URI, Name}` under `fileapi:<src>` with the injected TTL. Keep it one entry — splitting URI and Name across two keys lets one expire first, leaving a file that can still be referenced but never deleted. `DeleteFile` can only delete files whose `Name` is still in cache — a cache miss is an error, not a lookup fallback. **This is why `cache` is a required constructor argument** (`ErrCacheRequired`): with a nil cache, `DeleteFile` could never succeed and uploads would leak server-side. `PrepareImagePart` consults the same cache before downloading, and goes through `buildFileDataPart` on both paths so the payload doesn't depend on cache state.
- **`imgutil.GuessMIMEType` returns `""` for unrecognized extensions**, and `buildFileDataPart` omits `MIMEType` in that case. Do not reintroduce an `image/jpeg` fallback — declaring the wrong type is worse than letting the server sniff the content.
- **Naming**: `ports.AssetManager.EnsureUploaded(ctx, srcURL)` (fetch-then-upload, cached) is deliberately *not* called `UploadFile` — that name belongs to `gemini.FileManager.UploadFile(ctx, io.Reader, …)`, the low-level SDK call it delegates to.
- **Backend-specific options** (`gemini_helper.go: toOptions`): `PersonGeneration` is set **only** on Vertex AI — including it on the Gemini API backend causes a fatal API error. Safety thresholds also differ by backend (`Off` for Gemini API, `BlockNone` for Vertex).
- **Compression strategy**: when the core is constructed with `compress=true`, PNG/GIF inputs are re-encoded as JPEG (quality 75, `ImageCompressionQuality`) — both inline parts and File API uploads are fully decoded/re-encoded in memory before being sent (no streaming compression). Non-compressible formats are uploaded directly via `bufio.Reader` without buffering the whole file. Other formats pass through untouched.
- **Negative prompts** are not an API field; `buildFinalPrompt` concatenates them into the prompt text with a `[Negative Prompt]` separator.

## Conventions

- Comments are largely Japanese; error messages are mixed (Japanese in upload validation, English elsewhere) — match the surrounding file.
- Tests use testify + hand-written mocks in `generator/mocks_test.go`; nothing touches the network.
- README.md documents the public API and quick-start wiring (memory cache, downloader examples) — update it when `ports` types or constructor signatures change.
