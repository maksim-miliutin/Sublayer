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
started when it exits. A server you started yourself is used as it is. The dot of the
overlay stays grey while the servers start and turns green once Sublayer listens.

The subtitles appear at the bottom left over every window, and clicks go through them.
Sublayer also puts an icon in the notification area. Its colour follows the dot, hovering
over it tells the languages and what is going on, and its menu picks the language the
other side speaks, pauses the listening, hides the subtitles, opens the settings and
exits. A language picked there stays for the next start. Started by a double click,
Sublayer runs without a console window and tells of a failure in a message box; started
from a console, it keeps writing there, and Ctrl+C stops it too.

Keys work over every window: Ctrl+Alt+T shows or hides the subtitles, Ctrl+Alt+P pauses
and resumes the listening, and Ctrl+Alt+C copies the last pair. Ctrl+Alt+M lets the
subtitles be moved: clicks stop passing through them, any plate can be dragged, and the
same keys again fix them where they are, which the next start remembers. The tray menu
names the keys too, and a key another program already holds is told about in a
notification.

`-lang fr` or `-lang en` names the language the other side speaks for this run, and
`-lang auto` lets each phrase tell. `sublayer listen` prints the same pairs in the console
alone, and `sublayer listen file.wav` reads a recording instead of the speakers.

## Settings

Settings… in the tray menu opens a window with every setting: the languages, the servers
with a Browse button for each program and the model, the subtitles, the phrases and the
keys. Save writes them to the settings file and tells of a value Sublayer cannot use
instead. The language the other side speaks changes at once, the rest at the next start.

Sublayer keeps its settings in `%APPDATA%\Sublayer\config.json`, writes the defaults
there on the first run and reads the file at every start. The file names the language the
other side speaks and the one to translate into; for each server, the address it listens
on and the program that starts it, with the model whisper-server loads; how many pairs
the overlay shows, from 1 to 6, and how they look; how speech is cut into phrases: the
shortest speech that counts, the pause that ends a phrase and the longest phrase, in
milliseconds; the keys, written as `Ctrl+Alt+T`; and, once the subtitles were moved, the
screen corner they were moved to. A relative path is taken from the folder of
`sublayer.exe`, and forward slashes spare doubling every backslash:

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

In `overlay`, `text_size` is the size of the translation in points, from 8 to 40, and
the original takes three quarters of it; `width_percent` is the share of the screen width
the subtitles take, from 10 to 80; `background_percent` is how opaque their plates are,
from 0 for none to 100 for solid; and `linger_seconds` is how long a pair stays, up to
120, while 0 keeps it until newer pairs push it out:

```json
"pairs": 3,
"overlay": {
  "text_size": 12,
  "width_percent": 25,
  "background_percent": 75,
  "linger_seconds": 10
},
```

A phrase is translated once it ends, so a shorter `max_phrase_ms` brings long speech to
the screen sooner, cut in more places.
