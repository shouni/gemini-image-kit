# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Go library for image generation via Gemini / Vertex AI, built on top of `github.com/shouni/go-gemini-client`. It adds what the raw client lacks: fetching reference images from GCS/HTTP, Gemini File API upload with cache-backed reuse, in-memory JPEG compression, request-level rate limiting / concurrency / timeouts, and unified single/multi reference-image generation. No main package.

## Commands

```sh
go build ./...
go vet ./...
go test -race ./...                              # CI runs tests with -race
go test ./generator/ -run TestGenerateBatch      # single test
test -z "$(gofmt -l .)"                          # CI fails on unformatted code
golangci-lint run                                # CI uses v2.12.2; config in .golangci.yml
```

## Architecture

Two public packages plus one internal, in a ports-and-adapters layout:

- **`ports/`** — interfaces and request/response types only. Consumers inject implementations of `ContentReader` (GCS-style `gs://` access), `Downloader` (HTTP fetch — SSRF protection / domain allow-listing is deliberately the caller's responsibility), and `ImageCacher` (`Get`/`Set` only). `ImageGenerator` (one method: `Generate`) is the public entry interface, `BatchImageGenerator` adds `GenerateBatch`. There are no internal-seam interfaces in `ports` anymore — `imageExecutor` is a package-private seam inside `generator`.
- **`generator/`** — two layers:
  - `GeminiImageCore` (low-level): fetch → MIME detect → optional compress → upload/inline-part against `gemini.Model`, and parses Gemini responses into `ports.ImageResponse`. Its executor/resolver methods are unexported; external callers never invoke them directly.
  - `GeminiGenerator` (high-level): `Generate(ctx, ImageRequest)` and `GenerateBatch`. The old `GenerateSingleImage`/`GenerateFusedImage` split was a false dichotomy (both funneled into one private `generate`), so v1.13 merged them: `Images` with one element is the old single, several is the old fusion, empty is text-only.
- **`internal/imgutil/`** — stateless MIME detection/guessing and PNG/GIF→JPEG compression helpers. Internal because no external repo ever imported it.

### Behavior that spans files (worth knowing before editing)

- **Reference resolution lives in the low-level core** (`resolve.go: resolveReference`): Vertex AI + `gs://` → direct URI reference (wins over an explicit `FileAPIURI`, because it transfers nothing); else `FileAPIURI` if set; empty `ImageURI` is silently skipped; else **Gemini API → File API upload + URI reference** (cached, singleflighted), **Vertex AI → inline bytes** (Vertex has no File API). `InlineReferences: true` forces always-inline for one-shot references. Upload failure falls back to inline with a warning — the upload is a payload-size optimization, not a correctness requirement. **Exception: when the failure is the caller's own cancellation, the error propagates instead** — falling back would refetch the same image only to die of the same cancellation (`TestResolveReferenceCancelDoesNotFallBack` pins this).
- **File API cache contract** (`assets.go`): `ensureUploaded` caches the uploaded URI (a plain string) under `fileapi:<src>` with the injected TTL. `prepareImageAttachment` consults the same cache before downloading, and goes through `fileAttachment` on both paths so the payload doesn't depend on cache state. **`Cache` is a required constructor argument** (`ErrCacheRequired`) because upload reuse is the kit's main cost optimization. (`DeleteFile` and the `{URI, Name}` cache entry it required were removed in v1.13 — no consumer ever called it; use `gemini.Client.DeleteFile` directly if cleanup is ever needed.)
- **`imgutil.GuessMIMEType` returns `""` for unrecognized extensions**, and `fileAttachment` omits `MIMEType` in that case. Do not reintroduce an `image/jpeg` fallback — declaring the wrong type is worse than letting the server sniff the content. Query strings are stripped before extension matching (signed URLs used to defeat the guess entirely).
- **`gs://` detection is delegated to `go-utils/urlpath.IsGCSURI`** (`fetch.go: isGCSURI` is a one-line forwarder, kept unexported because a URI-scheme predicate isn't this kit's concern). Don't reinstate a local `strings.HasPrefix`: callers validate their own input with the same function, and a diverging predicate produces "the form accepted it but generation rejected it". Note it lowercases, so `GS://` counts.
- **`ensureUploaded` is singleflighted** and its shared run is detached from the caller's context (`context.WithoutCancel` + `UploadTimeout`). The cache is only written *after* a successful upload, so without this, concurrent uses of the same reference each upload a duplicate; and without the detach, the first caller canceling would kill the upload every piggybacked caller is waiting on. `ports.ImageCacher` implementations must therefore be safe for concurrent use.
- **`CacheTTL` is deliberately not defaulted.** 0 is a meaningful value to the cache the consumers actually use: `ttlcache.DefaultTTL` *is* 0, meaning "use the TTL configured on the cache", so substituting a kit default would override the caller's configuration. `CompressionQuality`, `UploadTimeout`, `FetchTimeout`, and `MaxReferenceBytes` are defaulted because the kit itself consumes them and has nobody to defer to. `TestCacheTTLIsPassedThroughUnchanged` pins this. Watch the other end too: a cache whose default outlives the File API's server-side retention will keep handing back dead `files/...` URIs.
- **Inline fetches are bounded** (`prepareImageAttachment`): `FetchTimeout` (default 1m) caps one fetch, `MaxReferenceBytes` (default 32 MiB) caps the bytes read — an unbounded `io.ReadAll` of a remote URL is a memory and latency hazard.
- **File layout**: files are named for what they hold, not for their layer. Low level: `core.go` (type/config/constructor), `resolve.go` (how one reference is sent), `assets.go` (File API upload + cache), `fetch.go` (transport, MIME sniffing, compression strategy), `executor.go` (model call + response parsing). High level: `generator.go` (entry points + guard options), `reference.go` (collecting many references concurrently), `request.go` (prompt/options/seed). There are no `*_helper.go` files — they turn into dumping grounds.
- **Backend-specific option defaults** (`request.go: toOptions`): `ports.GenerationOptions` embeds `gemini.GenerateOptions`, so `toOptions` only fills what the caller left unset — `SafetySettings` (per-backend threshold: `Off` for Gemini API, `BlockNone` for Vertex, which rejects `OFF`) and `PersonGeneration` (`AllowAll` on Vertex; always stripped on the Gemini API backend, which doesn't support it). Caller-provided values are respected — the kit no longer overwrites them.
- **Request guards live in `GeminiGenerator`** (`WithRateLimit`, `WithMaxConcurrency`, `WithRequestTimeout`): before v1.13 every consumer (go-veo-orchestrator, go-comic-kit) re-implemented rate limiting + errgroup around the kit. `GenerateBatch` preserves order, **keeps partial successes on failure** (each image already cost money), and reports failures via `joinBatchErrors` — each one tagged `requests[i]:`, with cancel-before-start folded into a single `N request(s) not started` line instead of N identical `context.Canceled`s.
- **Ordering inside `Generate` is deliberate** and three-way: `prepare` (validation + prompt build + auto-seed, no I/O) runs **before** `limiter.Wait`, so an invalid request neither burns a rate-limit slot nor waits a full interval to be rejected (`TestGenerateValidatesBeforeRateLimit`); `limiter.Wait` runs **outside** `requestContext`, so queueing time doesn't count against `WithRequestTimeout`; reference resolution runs **inside** it, because that's the part doing I/O. `GenerateBatch` acquires the semaphore before `go`, not inside it — otherwise a 1000-request batch at concurrency 1 spawns 1000 goroutines that only wait.
- **Compression strategy**: when the core is constructed with `Compress: true`, PNG/GIF inputs are re-encoded as JPEG (quality from `GeminiImageCoreConfig.CompressionQuality`, defaulting to 75) — both inline parts and File API uploads are fully decoded/re-encoded in memory before being sent (no streaming compression). Non-compressible formats are uploaded directly via `bufio.Reader` without buffering the whole file.
- **Negative prompts** are not an API field; `buildFinalPrompt` concatenates them into the prompt text with a `[Negative Prompt]` separator. (Downstream prompt code — e.g. ap-story's design prompts — depends on this exact rendering; treat the separator as a compatibility contract.)
- **Reference images are resolved concurrently** (`reference.go: collectImageAttachments`), because fusion requests pay one GCS/HTTP round trip per reference. Order is preserved (it affects how the model reads the references), a single reference skips the goroutines entirely, and the reported failure is the *first by input index* — with `context.Canceled` deselected, since one failure cancels the rest and would otherwise mask the real cause.
- **`ImageResponse.UsedSeed` is the requested seed, not one the API reports back** — the API never returns it. **Auto-seeding is the default since v1.13**: with no `Seed` in the request the kit picks one before sending, so `UsedSeed` is always a truthful reproducibility record (it used to be opt-in via `WithAutoSeed`, and the consumers that forgot it recorded a useless 0). `WithoutAutoSeed()` restores the old behavior for callers that manage seeds themselves. Seeds are bounded to int32 because `go-gemini-client` rejects anything wider (`ErrInvalidSeed`).
- **`ImageResponse` carries `Model`, `Prompt`, and `Usage`** alongside the bytes, so consumers can record cost and provenance without re-deriving them from the request.

## Conventions

- Comments are largely Japanese; sentinel errors are English with an `imagekit:` prefix — match the surrounding file.
- Tests use testify + hand-written mocks in `generator/mocks_test.go` (`stubExecutor` fakes the package-private `imageExecutor` seam via `newStubGenerator`); nothing touches the network.
- README.md documents the public API and quick-start wiring (memory cache, downloader examples) — update it when `ports` types or constructor signatures change.

## genai SDK boundary

This kit's public API deliberately contains no `google.golang.org/genai` types — the SDK stays inside `go-gemini-client`.

- Reference images travel as `gemini.Attachment`: inline bytes (`Data` + required `MIMEType`) or a URI reference (`URI` + optional `MIMEType`). `fileAttachment` builds the URI form for both GCS (`gs://`, Vertex only) and File API (`files/...`) sources; on a cache hit it returns the URI form, otherwise `prepareImageAttachment` returns the inline form after downloading.
- `executeRequest` takes the prompt and attachments separately and calls `gemini.Generator.GenerateWithAttachments`, which places the prompt before the attachments. That ordering was measured against the previous images-then-prompt shape on a real cover-art prompt: the difference was within run-to-run variance, so no ordering knob was added.
- `parseToResponse` reads `Response.Attachments` (MIME type + bytes). Empty/blocked responses are already turned into errors by `go-gemini-client`, so this kit only distinguishes "no image data".
