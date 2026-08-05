# BGIT — Developer Docs

## What is BGIT

All the actual logic lives in C++ (`src/main.cpp`). A Go program (`tui.go`) spawns your
compiled C++ binary and renders whatever it prints as a professional-looking TUI using
Bubble Tea + Lipgloss.

The workflow: **write your C++ however you like, rebuild, rerun the Go TUI — no Go changes
needed.**

## The magic — C++ output conventions

The TUI watches your program's output line-by-line and turns certain lines into UI controls:

| What you print in C++                                   | What the TUI does                        |
| ------------------------------------------------------- | ---------------------------------------- |
| `std::cout << "BGIT_VERSION v1.0.0\n"`                  | shows the version in the top bar         |
| `std::cout << "[1] Add, Commit, Push\n"`                | becomes a selectable menu item           |
| `std::cout << "Enter your choice --> "`                 | opens a text input field                 |
| `std::cout << "some normal text\n"`                     | gray line in the output panel            |
| `std::cerr << "something broke\n"`                      | red `[!]` error line in the output panel |

### Rules

- **Menu items**: any line starting with `[number] text` becomes an item. Number keys
  and arrow keys pick it, Enter sends that number to your C++ program.
- **Prompts/input**: any line *containing* `-->` opens the input field. Text typed before
  the marker becomes the label. This works even when you print the prompt *without* a
  newline, like `std::cout << "Enter stuff --> ";`. Pressing Enter sends the typed line
  to your program's stdin (`getline`).
- **Errors**: lines written to **stderr** (via `std::cerr`) render muted red with `[!]`.
  Git's own errors (failed push, etc.) show red automatically since git also writes to stderr.
- **Titles/welcome/anything else**: plain `std::cout` text streams into the output panel
  as gray lines.

## Build & Run

```sh
g++ -o bgit src/main.cpp    # compile the C++ logic
go build -o bgit-tui tui.go # compile the TUI
./bgit-tui                  # run it (spawns ./bgit automatically)
```

- The TUI runs your binary from the current directory. Use the `BGIT_BIN` env var if your
  binary has a different name or path:

  ```sh
  BGIT_BIN=./mytool ./bgit-tui
  ```

- After changing `src/main.cpp`, you must recompile the C++ binary — the Go TUI always
  spawns it fresh on launch, so no Go rebuild is needed for C++ changes.

- **Binary resolution**: the TUI looks for the C++ binary in this order — `BGIT_BIN` env
  var, `./bgit-core` in the current dir, `bgit-core` from PATH, then `./bgit` / `bgit`.
  When installed from the AUR, `bgit` is the TUI and `bgit-core` is the C++ logic.

## TUI Key Controls

| Key                          | Action                                   |
| ---------------------------- | ---------------------------------------- |
| `↑` / `↓`                    | move selection through menu items        |
| `1` `2` ... `9`              | jump-select a menu item by number        |
| `Enter`                      | send (selected item number, or typed input) |
| `Esc`                        | cancel current input                     |
| `r`                          | restart the C++ process (after it exits) |
| `q`                          | quit the TUI (not while typing)          |
| `Ctrl+C`                     | force quit                               |

## How the renderer works

- Go spawns your binary and pipes its stdout **and** stderr separately.
- Output is read with a byte buffer, so prompts printed without `\n`
  (`print("Enter choice --> ")`) are still detected as input prompts.
- Each line is classified: version, menu item, prompt, error line, or plain log.
- The output panel auto-shows the last N lines that fit the terminal height, so long git
  output scrolls naturally.
- Core areas: the top bar (version + status + handle), the menu list, the input row, and
  the streaming output panel.

## Example main.cpp

```cpp
#include <iostream>
#include <string>
#include <cstdlib>
#include <sstream>

const std::string BGIT_VERSION = "v1.0.0";

int main() {
    std::string input;
    std::cout << "BGIT_VERSION " << BGIT_VERSION << "\n";   // version in top bar
    std::cout << "Welcome To BGIT\n";
    std::cout << "[1] Add, Commit, Push\n";                  // menu item
    std::cout << "[2] Add, Custom Commit Message, Push\n";   // menu item
    std::cout << "Enter Your Choice --> ";                   // opens input field
    fflush(stdout);

    std::getline(std::cin, input);

    if (input == "1") {
        std::system("git add . && git commit -m \"update\" && git push");
    } else if (input == "2") {
        std::string msg;
        std::cout << "Enter Commit Message\n --> ";          // second input field
        std::getline(std::cin, msg);
        std::system(("git add . && git commit -m \"" + msg + "\" && git push").c_str());
    } else {
        std::cerr << "add a number lol\n";                   // red [!] error line
    }
    return 0;
}
```

## Troubleshooting

- **Added C++ output but nothing new appears** → recompile: `g++ -o bgit src/main.cpp`.
- **A line shows as gray text instead of a menu item** → make sure it starts with
  `[number] `.
- **Prompt not opening a field** → the line must contain `-->`.
- **Git output looks wrong / chopped** → it's streamed as it comes; long lines are
  truncated to the terminal width.
- **Text echoes glued together** → in C++, remember a trailing `\n` after echoing input
  (e.g. `std::cout << input << "\n";`).