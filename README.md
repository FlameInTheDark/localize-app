# Localize Desktop

Windows desktop translation and OCR workspace built with Go, Wails v2, React,
Tailwind v4, and shadcn-style UI primitives. It keeps the four Localize flows:
text translation/detection, document translation, image translation, and OCR
extraction, without retaining a translation history.

It can use either a user-managed Ollama server or a Localize-managed llama.cpp
server running only on `127.0.0.1`. llama.cpp models are public GGUF files
downloaded directly from Hugging Face into `%LocalAppData%\Localize\models`.
Speech input is handled by a user-installed whisper.cpp runtime and a selected
local Whisper model; microphone recordings and live speech stay on the device.

## Quick start

Prerequisites: Go 1.26+, Wails v2, Bun, and Windows 10/11 x64.

```powershell
cd H:\Projects\Go\src\github.com\FlameInTheDark\localize-app
bun --cwd frontend install
wails dev
```

For text translation with the default provider, install and start Ollama, then
use **Settings → Catalog** to open its official catalog and pull
`translategemma:latest`. Pull `glm-ocr:latest` for image/document OCR.

## Provider setup

### Ollama

- Default endpoint: `http://127.0.0.1:11434`.
- The app talks only to Ollama's documented local endpoints for health, local
  model lists, generation, and streamed pulls.
- The catalog is intentionally opened in the browser; Localize does not scrape
  an undocumented remote catalog API.

### llama.cpp + Hugging Face

- In **Settings → Runtime**, select an official llama.cpp version and install its
  CPU runtime (and optionally the matching CUDA runtime). The server is stored
  in `%LocalAppData%\Localize\runtime\llama.cpp`, not beside the app, so this
  works identically in development and in installed builds.
- In **Settings → Models**, choose the local directory for GGUF files, search
  public Hugging Face repositories, select a model file, and optionally select
  a matching `mmproj` projection for OCR.
- The app verifies a resumable partial download before atomically publishing it
  to the selected model directory. The install UI reports bytes transferred,
  live download speed, and an estimated remaining time.
- `Auto` runtime mode tries an installed CUDA server only when `nvidia-smi` is
  available. A failed CUDA readiness probe falls back to the installed CPU server. You
  can force CPU or CUDA from Settings.

### Whisper.cpp speech input

- In **Settings → Runtime**, install an official whisper.cpp CPU runtime and,
  optionally, its CUDA runtime. The selected version lives under
  `%LocalAppData%\Localize\runtime\whisper.cpp`.
- In **Settings → Speech**, choose a model directory, download an official
  multilingual Whisper model, select the model and spoken-language policy, and
  choose a microphone after granting Windows microphone permission.
- The **Voice** control in Text opens recording/upload. **Live** transcribes
  detected speech phrases into the text editor at the caret without translating
  automatically. Localize accepts WAV, MP3, FLAC, and OGG input files.

## Document support

PDF, EPUB, MOBI, DOCX, XLSX, and PPTX are extracted as ordered text sections.
When no embedded text exists, MuPDF renders pages and the configured vision/OCR
model reads them. Results remain original/translation text pairs; Localize does
not generate reconstructed Office or PDF files.

Before document translation, open **Settings → Runtime**, choose an official
MuPDF Windows version, install it, select the installed version, and save the
settings. MuPDF is stored under `%LocalAppData%\Localize\runtime\mupdf`; it is
not included in the installer.

## Development checks

```powershell
go vet ./...
go test -race ./...
bun --cwd frontend run check
bun --cwd frontend run build
wails build -debug
```

The tests cover provider contracts, prompt/language handling, settings atomicity
and migration, and local model-store safety.

## Windows installer release

No native runtime is bundled in the installer. llama.cpp, whisper.cpp, and
MuPDF are explicitly selected and installed by the user after launch. Build an
NSIS package with:

```powershell
.\scripts\build-installer.ps1
```

See [licenses/THIRD_PARTY_NOTICES.md](licenses/THIRD_PARTY_NOTICES.md) for the
runtime source and licensing information.

## GitHub Actions releases

Pull requests and pushes to `main` run the Windows verification workflow:
frontend typechecking/building plus Go vet and race-enabled tests. A push to
`main` that follows Conventional Commits creates a semantic GitHub release:
`feat:` releases a minor version, `fix:` releases a patch version, and a
breaking change releases the next major version. The release-assets workflow
then builds and attaches `Localize-<version>-windows-amd64.exe` and the NSIS
installer. Native runtimes remain user-installed and are not release assets.

## Architecture

The Wails-bound `Desktop` service is deliberately thin. Domain workflows depend
on narrow interfaces rather than Wails, HTTP, or child-process code:

- `inference.Client`: health, model listing, and text/vision generation.
- `runtime.LlamaManager`: Localize-owned loopback llama.cpp lifecycle.
- `runtime.WhisperRunner`: short-lived local whisper-cli transcription process.
- `catalog.HuggingFace` and `catalog.LocalModels`: discovery, validated model
  downloads, and local model management.
- `translation.Service` and `documents.Extractor`: shared prompts, chunking,
  extraction, and OCR fallback.
- `operations.Hub`: one typed Wails progress-event contract.

Settings are atomically stored in `%LocalAppData%\Localize\settings.json`.
Source documents, rendered pages, and images are held only for the active
operation and are cleaned up afterward.
