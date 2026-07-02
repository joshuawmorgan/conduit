# Conduit — Fish Completion Architecture

> Document ID: `53-fish-completion`
> Status: Draft (v0.1.0)
> Owner: Principal Developer-Experience Architect
> Last updated: 2026-07-02

Related documents:
- [Completion Engine](50-completion-engine.md)
- [Bash Completion Architecture](51-bash-completion.md)
- [Zsh Completion Architecture](52-zsh-completion.md)
- [PowerShell Completion Architecture](54-powershell-completion.md)
- [IntelliSense](55-intellisense.md)
- [LSP Architecture](56-lsp-architecture.md)

---

## 1. Overview

This document describes how the `conduit` binary (alias `cdt`) provides shell
completion for the [fish shell](https://fishshell.com/). It is one of a family
of thin per-shell front-ends that all delegate to the single, shell-agnostic
completion engine described in [Completion Engine](50-completion-engine.md).

The design goal is uniform behaviour across bash, zsh, fish, and PowerShell: the
_semantics_ of completion (which candidates, which descriptions, whether to
append a space, whether to fall through to file completion) are computed **once**
in Go, inside the hidden Cobra command `conduit __complete`. Each shell script is
responsible only for (a) collecting the current command line into `argv`,
(b) invoking `conduit __complete`, and (c) translating the wire protocol into
that shell's native completion primitives.

Fish is, in many respects, the most natural fit for this architecture: it has
first-class support for `value\tdescription` completion candidates, and its
`complete` builtin accepts a dynamic command substitution that can be re-run on
every keystroke. This lets us implement the entire integration with a **single**
`complete` registration plus one helper function.

### 1.1 Recap of the shared protocol

Per [Completion Engine](50-completion-engine.md), `conduit __complete` (and its
no-description sibling `conduit __completeNoDesc`) emit UTF-8 text on stdout:

- Zero or more **candidate** lines, each `VALUE\tDESCRIPTION` (the description
  and tab are omitted when there is no description).
- Zero or more **ActiveHelp** lines, each prefixed with the literal marker
  `_activeHelp_ ` (note the trailing space), carrying human-oriented guidance
  rather than an insertable value.
- Exactly one final **directive** line of the form `:<bitmask>`.

The directive bitmask:

| Name            | Bit | Value | Meaning                                             |
|-----------------|-----|-------|-----------------------------------------------------|
| `Error`         | 0   | 1     | Engine failed; shell should do nothing / fall back. |
| `NoSpace`       | 1   | 2     | Do not append a trailing space after the candidate. |
| `NoFileComp`    | 2   | 4     | Do not fall back to filename completion.            |
| `FilterFileExt` | 3   | 8     | Candidates are file extensions to filter on.        |
| `FilterDirs`    | 4   | 16    | Complete directories only.                          |
| `KeepOrder`     | 5   | 32    | Preserve engine order; do not sort candidates.      |

The whole round trip must stay within the **< 50 ms latency budget**; fish
re-invokes the completion function on essentially every Tab, so the helper must
be lean and must never do expensive work of its own.

---

## 2. The fish completion model

Fish completions are declarative. Instead of a procedural "generate candidates
now" callback (as in bash's `compgen`/`COMPREPLY`), you _register rules_ up front
with the `complete` builtin, and fish evaluates the applicable rules when the
user presses Tab.

Key `complete` flags used by Conduit:

| Flag                | Long form         | Purpose                                                                   |
|---------------------|-------------------|---------------------------------------------------------------------------|
| `-c CMD`            | `--command CMD`   | The command these rules apply to (`conduit`).                             |
| `-a 'ARGS'`        | `--arguments`     | Candidate values. May be a **command substitution** for dynamic results. |
| `-f`                | `--no-files`      | Disable fish's default filename completion for this command.             |
| `-F`                | `--force-files`   | Force filename completion (used to _re-enable_ files conditionally).      |
| `-n 'COND'`        | `--condition`     | A guard: the rule only applies if the shell snippet `COND` returns 0.    |
| `-r`                | `--require-parameter` | The option requires an argument (influences spacing behaviour).      |
| `-k`                | `--keep-order`    | Preserve the order of `-a` candidates rather than sorting.               |

### 2.1 Native `value\tdescription` support

A crucial fish property: when a candidate string produced by `-a` contains a
literal tab, fish splits it into the **completion value** (before the tab) and a
**description** (after the tab), and renders the description in a dim column in
the pager:

```
conduit flow ru<TAB>
run       (Execute a .flow pipeline)
runbook   (Manage stored runbooks)
```

This maps _directly_ onto our wire format. Where bash must strip descriptions and
zsh must reformat them into `_describe` arrays, fish can consume the engine's
`VALUE\tDESCRIPTION` lines almost verbatim. That is why the fish integration is
the thinnest of all four.

### 2.2 Why a single dynamic rule

Cobra-style generators sometimes emit a large static tree of `complete` rules
(one per subcommand/flag). Conduit deliberately does **not** do that. Because all
intelligence lives in `conduit __complete`, we register exactly one dynamic rule:

```fish
complete -c conduit -f -a '(__conduit_complete)'
```

- `-f` turns off fish's built-in file completion, so files only appear when the
  engine explicitly asks for them (via the directive), never by accident.
- `-a '(__conduit_complete)'` runs the helper function in a command substitution
  every time completion is requested; the function prints `VALUE\tDESCRIPTION`
  lines that fish renders natively.

The same single rule is duplicated for the `cdt` alias.

---

## 3. The `__conduit_complete` helper function

The helper bridges fish's `commandline` introspection to the engine and back.

### 3.1 Reading the current command line

Fish exposes the in-progress command line via the `commandline` builtin:

- `commandline -opc` — **o**utput the **p**rocessed **c**urrent tokens, i.e. the
  already-typed, tokenized arguments _excluding_ the token under the cursor.
- `commandline -ct` — output the **c**urrent **t**oken (the partial word the user
  is completing, possibly empty).

We forward these to the engine as the argv that Cobra will parse. The final,
possibly-empty current token must always be passed as the last argument so the
engine can compute prefix matches:

```fish
set -l args (commandline -opc)
set -l current (commandline -ct)
conduit __complete -- $args[2..-1] $current
```

`$args[1]` is `conduit`/`cdt` itself, which we drop (`$args[2..-1]`); the engine
is invoked with the subcommand path plus the current token. The `--` separates
our own flags from the reconstructed user argv, exactly as documented in doc 50.

### 3.2 Splitting value and description

Because fish renders `value\tdescription` natively, the helper largely passes
candidate lines straight through. It only needs to:

1. Skip the directive line (`:<bitmask>`) and capture the bitmask.
2. Convert ActiveHelp lines into fish-appropriate, non-insertable hints
   (see §3.4).
3. Emit remaining candidate lines unchanged (they already contain the tab).

### 3.3 Handling the directive bitmask

Fish's declarative model means we cannot always "return" spacing/file behaviour
from the substitution alone; some directives must be reflected back into the
`complete` registration or the `commandline` state. The mapping:

| Directive       | Fish realization                                                                                     |
|-----------------|------------------------------------------------------------------------------------------------------|
| `Error` (1)     | Print nothing and return non-zero; fish shows no candidates and (per `-f`) no files.                 |
| `NoSpace` (2)   | Suppress the trailing space. Achieved via a per-completion `-n`-guarded rule variant or by not terminating the token; see §3.3.1. |
| `NoFileComp` (4)| Already the default because the base rule uses `-f`; ensure no `-F` re-enable rule fires.            |
| `FilterFileExt` (8) | Emit a follow-on file rule restricted to the engine's extensions (Conduit uses `*.flow`).        |
| `FilterDirs` (16)   | Emit a directory-only completion (`__fish_complete_directories`).                                 |
| `KeepOrder` (32)| Register the rule with `-k` so fish does not re-sort candidates.                                     |

#### 3.3.1 NoSpace in fish

Fish appends a space after inserting a completion **unless** the value ends in a
character fish treats as "not word-terminating" or the completion is marked as
requiring a parameter. Conduit's convention (matching the Cobra fish generator)
is: when `NoSpace` is set, the helper appends a trailing sentinel that fish will
not add a space after, and — more robustly — the generated script wires the
`NoSpace` case through a dedicated code path that keeps the current token open.
In practice, because the engine most often sets `NoSpace` together with flags
like `--name=`, the value itself ends in `=` and fish naturally keeps the cursor
adjacent. For the general case the script inspects the directive and, when
`NoSpace` is set, avoids emitting the space-terminating variant.

### 3.4 ActiveHelp presentation

ActiveHelp lines (`_activeHelp_ <text>`) are **not** insertable candidates — they
are guidance such as "Provide the runbook name, e.g. `deploy-prod`". Fish has no
dedicated ActiveHelp channel, so Conduit presents them as a **description-only,
non-insertable pseudo-candidate**: the helper strips the `_activeHelp_ ` marker
and, when `$CONDUIT_ACTIVE_HELP` is not `0`, prints the text as the _description_
of an empty/placeholder value so it appears in the pager without being inserted.
When `CONDUIT_ACTIVE_HELP=0`, these lines are dropped entirely, matching the
opt-out behaviour of the other shells.

---

## 4. Installation flow

Fish **autoloads** completion files from any directory on `$fish_complete_path`;
the canonical user location is `~/.config/fish/completions/`. A file named
`conduit.fish` placed there is sourced lazily the first time the user completes
`conduit` — there is no need to `source` it manually or edit `config.fish`.

```fish
# Generate and install the Conduit fish completions (annotated)
#
# `conduit completion fish` writes the generated script to stdout.
# We redirect it to the per-user completions directory; fish will
# autoload it on demand — no `source` and no shell restart required.

mkdir -p ~/.config/fish/completions              # ensure the autoload dir exists
conduit completion fish > ~/.config/fish/completions/conduit.fish

# Verify: the helper function should now be discoverable.
type -q __conduit_complete; and echo "Conduit fish completions installed"
```

For a quick, non-persistent trial in the current session only:

```fish
conduit completion fish | source   # load for this session; not autoloaded later
```

System-wide installation (e.g. from a package) drops the same file into
`/usr/share/fish/vendor_completions.d/conduit.fish`, which is also on the default
`$fish_complete_path`.

---

## 5. Fish specifics

- **Autoloading, not sourcing.** Files in a completions directory are loaded on
  first use of the command, keyed by filename. The file _must_ be named
  `conduit.fish` (matching the command) for lazy autoloading to trigger.
- **No `source` in `config.fish`.** Unlike bash/zsh, adding completions does not
  require editing startup files. This keeps installs idempotent and fast.
- **`commandline` tokenization.** Fish tokenizes according to its own quoting
  rules; `commandline -opc` yields already-unquoted tokens, so the helper does
  not need to re-parse quotes. The current token from `commandline -ct` may be
  empty (cursor after a space), which the engine handles as "complete the next
  positional/flag".
- **Command substitution runs per completion.** The `(__conduit_complete)`
  substitution executes on every Tab; keep it fast to honour the < 50 ms budget.
  The function does no I/O beyond the single `conduit __complete` call.
- **Alias `cdt`.** The generated script registers identical rules for `cdt` so
  the alias completes with the same fidelity as `conduit`.

---

## 6. Generated `conduit.fish` (annotated)

The following is a realistic, Conduit-branded rendering of what
`conduit completion fish` emits. It resembles Cobra's fish output but is
annotated to explain each moving part.

```fish
# fish completion for conduit                                    -*- shell-script -*-
#
# Generated by `conduit completion fish`. Do not edit by hand.
# All completion logic is delegated to the hidden `conduit __complete`
# command (see docs/50-completion-engine.md). This script is a thin
# front-end that translates the shared wire protocol into fish rules.

function __conduit_debug
    # Emit debug output only when $BASH_COMP_DEBUG_FILE-equivalent is set.
    set -l file "$CONDUIT_COMP_DEBUG_FILE"
    if test -n "$file"
        echo "$argv" >> $file
    end
end

function __conduit_perform_completion
    __conduit_debug "Starting __conduit_perform_completion"

    # Extract all args except the last (already-complete tokens) and the
    # last token (the partial word under the cursor).
    set -l args (commandline -opc)
    set -l lastArg (commandline -ct)

    __conduit_debug "args: $args"
    __conduit_debug "last arg: $lastArg"

    # Build the request. args[1] is the command name (conduit/cdt); the
    # engine is invoked as: <cmd> __complete -- <subargs...> <lastArg>
    set -l requestComp "$args[1] __complete -- $args[2..-1] $lastArg"
    __conduit_debug "Calling $requestComp"

    # Run the engine. Its stdout is the shared protocol:
    #   VALUE\tDESCRIPTION   (candidate lines)
    #   _activeHelp_ TEXT    (ActiveHelp lines)
    #   :<bitmask>           (final directive line)
    set -l results (eval $requestComp 2> /dev/null)

    # The last line is the directive; everything before it is candidates.
    set -l comps $results[1..-2]
    set -l directiveLine $results[-1]

    # Parse the ":<bitmask>" directive. Default to 0 if malformed.
    set -l directive 0
    if string match -qr '^:\d+$' -- "$directiveLine"
        set directive (string sub -s 2 -- "$directiveLine")
    else
        # No directive line at all (older/edge output): treat everything as
        # candidates and assume default behaviour.
        set comps $results
    end

    __conduit_debug "Comps: $comps"
    __conduit_debug "Directive: $directive"

    # Echo candidates for the caller to consume; the directive is returned
    # via the function's exit status math below (see __conduit_complete).
    for comp in $comps
        printf "%s\n" $comp
    end

    printf ":%d\n" $directive
end

function __conduit_complete
    # This is the function referenced by `complete -a '(__conduit_complete)'`.
    # It filters the engine output into fish-native candidate lines and
    # applies the directive.

    set -l response (__conduit_perform_completion)
    set -l directiveLine $response[-1]
    set -l comps $response[1..-2]

    set -l directive (string sub -s 2 -- "$directiveLine")

    # Bit 0 (Error=1): abort silently.
    if test (math "$directive & 1") -ne 0
        __conduit_debug "Received error directive: aborting"
        return 1
    end

    # Bit 4 (FilterDirs=16): complete directories only.
    if test (math "$directive & 16") -ne 0
        __conduit_debug "FilterDirs: directory completion"
        __fish_complete_directories (commandline -ct)
        return 0
    end

    # Bit 3 (FilterFileExt=8): restrict file completion to engine's exts.
    # Conduit uses this for *.flow pipeline files.
    if test (math "$directive & 8") -ne 0
        __conduit_debug "FilterFileExt: restricting to reported extensions"
        for comp in $comps
            # Each $comp here is an extension (e.g. "flow"); complete files.
            __fish_complete_suffix ".$comp"
        end
        return 0
    end

    # Default path: emit VALUE\tDESCRIPTION candidates. Fish splits on the
    # tab natively, rendering the description in the pager.
    for comp in $comps
        # ActiveHelp lines are surfaced as description-only hints.
        if string match -q '_activeHelp_ *' -- "$comp"
            if test "$CONDUIT_ACTIVE_HELP" != 0
                set -l help (string replace '_activeHelp_ ' '' -- "$comp")
                # Present as a non-inserting description on an empty value.
                printf "\t%s\n" $help
            end
            continue
        end
        # Pass the candidate through untouched: fish handles the tab split.
        printf "%s\n" $comp
    end
end

# Turn OFF fish's default file completion for conduit (-f). Files are only
# offered when the engine's directive asks for them (FilterFileExt/FilterDirs).
# Register the single dynamic rule for both the command and its alias.
complete -c conduit -f -a '(__conduit_complete)'
complete -c cdt     -f -a '(__conduit_complete)'
```

> Note on `KeepOrder`: fish sorts `-a` candidates by default. When the engine
> sets `KeepOrder` (bit 5), the generated script emits the rule with `-k`
> (`complete -k -c conduit -f -a '(__conduit_complete)'`) so the engine-provided
> order is preserved. The generator selects the `-k` variant at generation time
> based on the command's declared behaviour, or the helper re-registers with `-k`
> lazily on first observing the directive.

---

## 7. Directive-parsing snippet (annotated)

The core of the bridge is the directive parse. Below is the isolated, annotated
snippet showing exactly how each bit is honoured. This is the fish analogue of
the `COMPREPLY`/`compadd` post-processing in the bash/zsh scripts.

```fish
function __conduit_apply_directive --argument-names directive
    # $directive is the integer bitmask extracted from the ":<n>" line.

    # NoSpace (2): do not append a trailing space after insertion.
    #   Fish decides spacing from the completion string, so we keep the
    #   current token "open". When set together with a "=" flag value the
    #   value already ends in "=", so no extra work is needed; otherwise we
    #   suppress the terminating space by not registering the space-adding
    #   variant of the rule.
    if test (math "$directive & 2") -ne 0
        set -g __conduit_nospace 1     # consumed by the rule selection above
    else
        set -g __conduit_nospace 0
    end

    # NoFileComp (4): never fall back to filename completion. This is already
    #   the base behaviour because we registered the rule with `-f`. We assert
    #   it here so no conditional `-F` (force-files) rule can re-enable files.
    if test (math "$directive & 4") -ne 0
        __conduit_debug "NoFileComp: file fallback stays disabled"
    end

    # FilterFileExt (8): candidates are extensions; complete matching files.
    #   Conduit uses this for `*.flow` inputs to `conduit flow run`.
    if test (math "$directive & 8") -ne 0
        __conduit_debug "FilterFileExt: completing files by extension"
        return  # handled by the file-suffix loop in __conduit_complete
    end

    # FilterDirs (16): complete directories only.
    if test (math "$directive & 16") -ne 0
        __conduit_debug "FilterDirs: completing directories"
        return  # handled by __fish_complete_directories in __conduit_complete
    end

    # KeepOrder (32): preserve engine order. The rule must be registered with
    #   `-k`; if it was not, re-register it now so subsequent completions keep
    #   order for this command.
    if test (math "$directive & 32") -ne 0
        __conduit_debug "KeepOrder: preserving engine-provided order"
        complete -k -c conduit -f -a '(__conduit_complete)'
    end
end
```

---

## 8. Testing and diagnostics

- Set `CONDUIT_COMP_DEBUG_FILE=/tmp/conduit-fish.log` and complete a few
  commands; the `__conduit_debug` calls trace the exact argv, engine output,
  and parsed directive.
- Exercise the engine directly to see the raw wire format fish consumes:

  ```fish
  conduit __complete -- flow run ''      # candidate lines + trailing :<n>
  conduit __complete -- flow run './p'   # FilterFileExt=8 for *.flow inputs
  ```

- Confirm autoloading by checking `functions __conduit_complete` resolves after
  a fresh shell (proves the file was placed on `$fish_complete_path`).
- To validate the < 50 ms budget, time a single engine call:
  `time conduit __complete -- flow ''`.

---

## 9. Cross-references

- [Completion Engine](50-completion-engine.md) — authoritative definition of the
  `conduit __complete` wire protocol, directive bitmask, and ActiveHelp marker
  that this document consumes.
- [LSP Architecture](56-lsp-architecture.md) — the `conduit lsp` server that
  powers editor IntelliSense for FlowDSL (`*.flow`) files; it shares the same
  candidate-generation core as the shell completion engine, so fish shell
  completion and in-editor completion stay semantically aligned.
