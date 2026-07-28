# Third-party notices

This document identifies third-party software used by Localize. The Localize
project license is provided separately in the repository-root `LICENSE` file.

## Included in Localize releases

| Component | License | Source |
| --- | --- | --- |
| Wails v2 | MIT | https://github.com/wailsapp/wails |
| React and React DOM | MIT | https://github.com/facebook/react |
| Tailwind CSS | MIT | https://github.com/tailwindlabs/tailwindcss |
| Radix UI and cmdk | MIT | https://www.radix-ui.com/ · https://cmdk.paco.me/ |
| Zustand | MIT | https://github.com/pmndrs/zustand |
| Lucide icons | ISC | https://lucide.dev/ |
| Geist and Geist Mono typefaces | SIL Open Font License 1.1 | https://fonts.google.com/specimen/Geist |
| Nunito typeface | SIL Open Font License 1.1 | https://fonts.google.com/specimen/Nunito |

Direct and transitive dependencies retain their own license terms.

## Optional, user-installed components

Localize does not bundle, mirror, or include native runtime binaries, model
weights, or their installers in its source archive, application package, or
NSIS installer. A user may explicitly request a direct download from the
upstream publisher in Settings.

| Component | Use | License / source |
| --- | --- | --- |
| llama.cpp | Local inference server | MIT — https://github.com/ggml-org/llama.cpp/blob/master/LICENSE |
| whisper.cpp | Local speech transcription | MIT — https://github.com/ggml-org/whisper.cpp/blob/master/LICENSE |
| MuPDF | Document extraction and rendering | AGPL-3.0 or commercial — https://mupdf.com/releases |
| Ollama models | Local inference models | Model-specific — https://ollama.com/library |
| Hugging Face models | Local GGUF and Whisper model files | Model-specific — https://huggingface.co/docs/hub/en/repositories-licenses |

The person installing an optional runtime or model is responsible for reviewing
and complying with that component's current license, attribution requirements,
and use restrictions. Model licenses are not uniform and can impose additional
conditions beyond the Localize project license.
