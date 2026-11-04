#!/bin/zsh
# Plays the stripsaw demo in the current Ghostty window by typing keystrokes
# into it, so you can screen-record a real session with sharp images.
#
#   1. Size the Ghostty window the way you want it to look in the video.
#   2. Start a screen recording of that window (Cmd-Shift-5, "Record Selected Portion").
#   3. Run:  demo/record-demo.sh
#      Don't touch the keyboard or switch apps until it finishes (~30s).
#   4. Stop the recording.
#
# The keystrokes go to the frontmost app, so Ghostty needs permission under
# System Settings > Privacy & Security > Accessibility (macOS asks on first run).
#
# samples/panels.json is restored to its previous state when the demo ends.
# Set DRY_RUN=1 to print the steps instead of typing them.

set -e
cd "${0:A:h}/.."
go build -o stripsaw .

typeit() { # type text one character at a time, like a person
  local text=$1 i
  if [[ -n $DRY_RUN ]]; then print -r -- "type: $text"; return; fi
  for (( i = 1; i <= ${#text}; i++ )); do
    osascript -e "tell application \"System Events\" to keystroke \"${text[i]}\""
    sleep 0.08
  done
}
enter() {
  if [[ -n $DRY_RUN ]]; then print "enter"; return; fi
  osascript -e 'tell application "System Events" to key code 36'
}
pause() { [[ -n $DRY_RUN ]] && print "pause $1" || sleep $1; }

demo() {
  local manifest=samples/panels.json backup=""
  if [[ -f $manifest ]]; then
    backup=$(mktemp)
    cp $manifest $backup
  fi

  pause 1.5
  typeit "./stripsaw samples"; pause 0.4; enter
  pause 3                                   # first comic with the ruler
  typeit "33,66"; pause 0.4; enter
  pause 3                                   # preview with split lines
  enter                                     # save
  pause 3                                   # second comic
  typeit "32.5,65"; pause 0.4; enter
  pause 3                                   # preview
  enter                                     # save
  pause 2.5                                 # third comic
  typeit "l"; pause 0.4; enter              # show the list
  pause 5
  enter                                     # back to the comic
  pause 1.5
  typeit "q"; pause 0.4; enter
  pause 1

  if [[ -n $backup ]]; then mv $backup $manifest; else rm -f $manifest; fi
}

if [[ -n $DRY_RUN ]]; then
  demo
  exit
fi

# Run the typing in the background and return to the shell immediately,
# so the keystrokes land at the shell prompt and then in stripsaw.
clear
demo >/dev/null 2>&1 &!
