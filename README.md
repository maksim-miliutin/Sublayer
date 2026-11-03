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
- [whisper.cpp](https://github.com/ggml-org/whisper.cpp) `whisper-server`, with a model
  such as `ggml-large-v3-turbo.bin` on a GPU or `ggml-small.bin` on a CPU.
- [LibreTranslate](https://github.com/LibreTranslate/LibreTranslate) with the `en→ru`
  model, and `fr→en` too for French, which is translated into Russian through English.
- Go, the version named in `go.mod`, to build.

## Build

```
go build ./cmd/sublayer
```

## Run

```
sublayer
```

Sublayer starts whisper-server and LibreTranslate itself when nothing answers at their
addresses, with the programs and the model named in the settings, and stops the ones it
started when it exits. A server you started yourself is used as it is.

The subtitles appear at the bottom left over every window, and clicks go through them.
Press Ctrl+C in the console to stop. `-lang fr` or `-lang en` names the language the
other side speaks for this run, and `-lang auto` lets each phrase tell. `sublayer listen`
prints the same pairs in the console alone, and `sublayer listen file.wav` reads a
recording instead of the speakers.

## Settings

Sublayer keeps its settings in `%APPDATA%\Sublayer\config.json` and writes the defaults
there on the first run. The file names the language the other side speaks and the one to
translate into; for each server, the address it listens on and the program that starts
it, with the model whisper-server loads; how many pairs the overlay shows, from 3 to 6;
and how speech is cut into phrases: the shortest speech that counts, the pause that ends
a phrase and the longest phrase, in milliseconds. A relative path is taken from the folder
of `sublayer.exe`, and forward slashes spare doubling every backslash:

```json
"whisper": {
  "address": "127.0.0.1:8178",
  "program": "C:/whisper/Release/whisper-server.exe",
  "model": "C:/whisper/models/ggml-small.bin"
},
"libretranslate": {
  "address": "127.0.0.1:5000",
  "program": "C:/lt-venv/Scripts/libretranslate.exe"
}
```
