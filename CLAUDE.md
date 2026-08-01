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

- **Reference resolution lives in the low-level core** (`resolve.go: GeminiImageCore.ResolveReference`), not in the generator: Vertex AI + `gs://` → direct URI reference (wins over an explicit `FileAPIURI`, because it transfers nothing); else `FileAPIURI` if set; empty `ImageURI` is silently skipped; else **Gemini API → File API upload + URI reference** (cached, singleflighted), **Vertex AI → inline bytes** (Vertex has no File API). `InlineReferences: true` forces the old always-inline behavior for one-shot references. Upload failure falls back to inline with a warning — the upload is a payload-size optimization, not a correctness requirement. Before this, the generator inlined every reference and the *apps* (go-comic-kit, go-veo-orchestrator) each hand-rolled the upload orchestration; ap-comp didn't, so its injected cache was never written to.
- **File API cache contract** (`assets.go`): `EnsureUploaded` caches a single `cachedFile{URI, Name}` under `fileapi:<src>` with the injected TTL. Keep it one entry — splitting URI and Name across two keys lets one expire first, leaving a file that can still be referenced but never deleted. `DeleteFile` can only delete files whose `Name` is still in cache — a cache miss is an error, not a lookup fallback. **This is why `cache` is a required constructor argument** (`ErrCacheRequired`): with a nil cache, `DeleteFile` could never succeed and uploads would leak server-side. `PrepareImageAttachment` consults the same cache before downloading, and goes through `fileAttachment` on both paths so the payload doesn't depend on cache state.
- **`imgutil.GuessMIMEType` returns `""` for unrecognized extensions**, and `buildFileDataPart` omits `MIMEType` in that case. Do not reintroduce an `image/jpeg` fallback — declaring the wrong type is worse than letting the server sniff the content.
- **`EnsureUploaded` is singleflighted** and its shared run is detached from the caller's context (`context.WithoutCancel` + `UploadTimeout`). The cache is only written *after* a successful upload, so without this, concurrent uses of the same reference each upload a duplicate; and without the detach, the first caller canceling would kill the upload every piggybacked caller is waiting on. `ports.ImageCacher` implementations must therefore be safe for concurrent use.
- **File layout**: files are named for what they hold, not for their layer. Low level: `core.go` (type/config/constructor), `resolve.go` (how one reference is sent), `assets.go` (File API lifecycle + cache), `fetch.go` (transport, MIME sniffing, compression strategy), `executor.go` (model call + response parsing). High level: `generator.go`, `reference.go` (collecting many references concurrently), `request.go` (prompt/options/seed). There are no `*_helper.go` files — they turn into dumping grounds.
- **Naming**: `ports.AssetManager.EnsureUploaded(ctx, srcURL)` (fetch-then-upload, cached) is deliberately *not* called `UploadFile` — that name belongs to `gemini.FileManager.UploadFile(ctx, io.Reader, …)`, the low-level SDK call it delegates to.
- **Backend-specific options** (`gemini_helper.go: toOptions`): `PersonGeneration` is set **only** on Vertex AI — including it on the Gemini API backend causes a fatal API error. Safety thresholds also differ by backend (`Off` for Gemini API, `BlockNone` for Vertex).
- **Compression strategy**: when the core is constructed with `Compress: true`, PNG/GIF inputs are re-encoded as JPEG (quality from `GeminiImageCoreConfig.CompressionQuality`, defaulting to `DefaultCompressionQuality` = 75) — both inline parts and File API uploads are fully decoded/re-encoded in memory before being sent (no streaming compression). Non-compressible formats are uploaded directly via `bufio.Reader` without buffering the whole file. Other formats pass through untouched.
- **Negative prompts** are not an API field; `buildFinalPrompt` concatenates them into the prompt text with a `[Negative Prompt]` separator.
- **Reference images are resolved concurrently** (`gemini_helper.go: collectImageAttachments`), because fusion requests pay one GCS/HTTP round trip per reference. Order is preserved (it affects how the model reads the references), a single reference skips the goroutines entirely, and the reported failure is the *first by input index* — with `context.Canceled` deselected, since one failure cancels the rest and would otherwise mask the real cause. This is why `ports.ImageCacher` implementations must be safe for concurrent use.
- **`ImageResponse.UsedSeed` is the requested seed, not one the API reports back** — the API never returns it. With no `Seed` in the request it stays 0, which is a *wrong* reproducibility record (0 is itself a valid seed, so a caller replaying it changes the result). `WithAutoSeed()` makes the kit pick the seed before sending so `UsedSeed` is always truthful; it is opt-in because it changes what is sent to the API. Seeds are bounded to int32 because `go-gemini-client` rejects anything wider (`ErrInvalidSeed`).

## Conventions

- Comments are largely Japanese; error messages are mixed (Japanese in upload validation, English elsewhere) — match the surrounding file.
- Tests use testify + hand-written mocks in `generator/mocks_test.go`; nothing touches the network.
- README.md documents the public API and quick-start wiring (memory cache, downloader examples) — update it when `ports` types or constructor signatures change.

## genai SDK boundary

This kit's public API deliberately contains no `google.golang.org/genai` types — the SDK stays inside `go-gemini-client`.

- Reference images travel as `gemini.Attachment`: inline bytes (`Data` + required `MIMEType`) or a URI reference (`URI` + optional `MIMEType`). `fileAttachment` builds the URI form for both GCS (`gs://`, Vertex only) and File API (`files/...`) sources; `PrepareImageAttachment` returns the inline form after downloading.
- `ExecuteRequest` takes the prompt and attachments separately and calls `gemini.MultimodalGenerator.GenerateWithAttachments`, which places the prompt before the attachments. That ordering was measured against the previous images-then-prompt shape on a real cover-art prompt: the difference was within run-to-run variance, so no ordering knob was added.
- `parseToResponse` reads `Response.Attachments` (MIME type + bytes) rather than walking `RawResponse.Candidates`. Empty/blocked responses are already turned into errors by `go-gemini-client`, so this kit only distinguishes "no image data".
- Safety thresholds come from `gemini.SafetyOff` / `gemini.SafetyBlockNone` (`safetyThreshold` picks per backend — Vertex AI rejects `OFF`), and `ports.Backend` is an alias of `gemini.BackendInspector` so the same one-method interface isn't declared twice.

If you need Part-level control that `Attachment` cannot express, reach for `gemini.GenerateWithParts` — but that pulls genai back into this kit's signatures and every mock that implements them.
