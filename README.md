# stripsaw

A terminal tool for marking the panels in comic strips. It shows each image in
the terminal (Kitty graphics protocol: Ghostty, kitty, WezTerm) with a ruler
overlay. You type the split positions, and it records the panel coordinates in
a `panels.json` manifest, so a website can rearrange the panels for different
screen sizes.

```
go install github.com/oaklandgit/stripsaw@latest

stripsaw samples          # mark images that aren't in samples/panels.json yet
stripsaw list samples     # show which images are marked
```

Or build from a clone with `go build -o stripsaw .`

Run `stripsaw -h` for all commands and flags.

## License

The stripsaw source code is released under the [MIT License](LICENSE).

The comic images in [`samples/`](samples/) are copyright Larry Stone and are
licensed under [CC BY-NC-ND 4.0](https://creativecommons.org/licenses/by-nc-nd/4.0/).
They are **not** covered by the MIT License. You're welcome to share them with
credit and a link to [blabcomic.com](https://blabcomic.com), but not
commercially or in modified form. See [`samples/LICENSE`](samples/LICENSE).
