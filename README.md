# Sublayer

Live subtitles with translation for calls. Sublayer listens to what your speakers
play, the other side of a Teams or Discord call or any other program, cuts it into
phrases, recognizes each one with a local whisper.cpp server and translates it once
the phrase is finished. A transparent overlay on the left edge of the screen shows the
original and the translation under it, and clicks pass through to the windows below.

Your microphone is never captured: only the other side is translated. Speech stays on
your computer unless you pick a cloud translator, and the transcript lives in memory
unless you turn saving on. Tell the people you talk to that subtitles are on.

## Requirements

- Windows 10 or 11, x64. Linux and macOS come later.
- [whisper.cpp](https://github.com/ggml-org/whisper.cpp) `whisper-server` with a model,
  such as `ggml-large-v3-turbo.bin` on a GPU or `ggml-small.bin` on a CPU.
- [LibreTranslate](https://github.com/LibreTranslate/LibreTranslate) running locally.
- Go, the version named in `go.mod`, to build.

## Build

```
go build ./cmd/sublayer
```
