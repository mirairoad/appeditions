# Fonts

Static TrueType, one file per family and weight, embedded into the binary by
`internal/render/fonts.go`.

Static rather than variable on purpose: `golang.org/x/image/font/sfnt` reads a
variable font's *default* instance and has no way to select an axis position,
so a variable Inter would render every headline at weight 400 no matter what
the renderer asked for. The files come from
[expo/google-fonts](https://github.com/expo/google-fonts), which publishes one
static TTF per named instance of each Google font.

| file | family | licence |
| --- | --- | --- |
| `inter-*.ttf` | Inter | OFL 1.1 |
| `dm-sans-*.ttf` | DM Sans | OFL 1.1 |
| `poppins-*.ttf` | Poppins | OFL 1.1 |
| `space-grotesk-*.ttf` | Space Grotesk | OFL 1.1 |
| `playfair-*.ttf` | Playfair Display | OFL 1.1 |
| `noto-jp-*.ttf` | Noto Sans JP | OFL 1.1 |

`OFL.txt` is the licence all of them are published under.

Noto Sans JP is the CJK fallback: none of the Latin faces contain kana or
kanji, and a headline in Japanese would otherwise render as a row of blanks.
It covers Japanese only. Korean and Chinese fall back to the host's own fonts
(`AppleSDGothicNeo.ttc`, `Hiragino Sans GB.ttc` on macOS), and any `.ttf`/`.ttc`
dropped in `~/.appeditions/fonts/` joins the chain ahead of those.
