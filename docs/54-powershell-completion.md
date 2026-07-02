# Conduit — PowerShell Completion Architecture

> Document ID: `54-powershell-completion`
> Status: Draft (v0.1.0)
> Owner: Principal Developer-Experience Architect
> Last updated: 2026-07-02

Related documents:
- [Completion Engine](50-completion-engine.md)
- [Bash Completion Architecture](51-bash-completion.md)
- [Zsh Completion Architecture](52-zsh-completion.md)
- [Fish Completion Architecture](53-fish-completion.md)
- [IntelliSense](55-intellisense.md)
- [LSP Architecture](56-lsp-architecture.md)

---

## 1. Overview

This document describes how the `conduit` binary (alias `cdt`) provides argument
completion in PowerShell — both Windows PowerShell 5.1 and PowerShell 7+
(`pwsh`), on Windows, Linux, and macOS. Like the bash, zsh, and fish front-ends,
the PowerShell integration is a **thin** translator: all candidate computation
happens in the shared, shell-agnostic engine `conduit __complete`, defined in
[Completion Engine](50-completion-engine.md).

PowerShell's native completion API differs substantially from the POSIX shells.
Rather than emitting text that the shell parses, we register a **script block**
via `Register-ArgumentCompleter` that returns strongly-typed
`[System.Management.Automation.CompletionResult]` objects. This lets Conduit map
the engine's `VALUE\tDESCRIPTION` lines onto rich completion items with tooltips
rendered in the PSReadLine menu.

### 1.1 Recap of the shared protocol

Per [Completion Engine](50-completion-engine.md), `conduit __complete` (and its
no-description sibling `conduit __completeNoDesc`) write UTF-8 to stdout:

- Zero or more **candidate** lines, each `VALUE\tDESCRIPTION` (tab and
  description omitted when there is no description).
- Zero or more **ActiveHelp** lines, each prefixed with the literal marker
  `_activeHelp_ ` (trailing space included).
- Exactly one final **directive** line, `:<bitmask>`.

Directive bitmask:

| Name            | Bit | Value | Meaning                                             |
|-----------------|-----|-------|-----------------------------------------------------|
| `Error`         | 0   | 1     | Engine failed; return no results.                   |
| `NoSpace`       | 1   | 2     | Do not append a trailing space after the candidate. |
| `NoFileComp`    | 2   | 4     | Do not fall back to filename completion.            |
| `FilterFileExt` | 3   | 8     | Candidates are extensions to filter files by.       |
| `FilterDirs`    | 4   | 16    | Complete directories only.                          |
| `KeepOrder`     | 5   | 32    | Preserve engine order; do not sort.                 |

The full round trip must stay under the **< 50 ms latency budget**. PowerShell
invokes the completer script block on demand (Tab or menu open), so a single fast
`conduit __complete` call per request is well within budget.

---

## 2. The PowerShell completion model

### 2.1 `Register-ArgumentCompleter -Native`

PowerShell distinguishes _cmdlet_ argument completers from _native executable_
completers. Because `conduit` is an external `.exe` (or ELF/Mach-O binary), we
register a **native** completer:

```powershell
Register-ArgumentCompleter -CommandName conduit,cdt -Native -ScriptBlock { ... }
```

- `-Native` tells PowerShell this completer applies to a native command, so the
  script block receives the raw AST rather than bound cmdlet parameters.
- `-CommandName conduit,cdt` registers both the primary name and the alias in a
  single call.
- `-ScriptBlock { ... }` is invoked whenever the user requests completion for a
  command line beginning with `conduit` or `cdt`.

### 2.2 Script block parameters

A native argument completer script block receives three positional arguments:

```powershell
param($wordToComplete, $commandAst, $cursorPosition)
```

- `$wordToComplete` — the partial token under the cursor (may be empty).
- `$commandAst` — a `System.Management.Automation.Language.CommandAst`
  representing the whole command line as parsed; its `.CommandElements` are the
  individual tokens.
- `$cursorPosition` — the integer offset of the cursor within the line, used to
  determine whether the cursor sits at the end of the last element or in
  whitespace after it (i.e. whether the user is completing a new, empty token).

### 2.3 Building the engine request

We reconstruct the argv from `$commandAst.CommandElements`, drop the command name
itself, and append `$wordToComplete` (which may be empty) as the final token so
the engine can compute prefix matches — exactly the same request shape the other
shells build:

```powershell
$elements = $commandAst.CommandElements
# elements[0] is 'conduit'/'cdt'; forward the rest plus the current word.
$args = @($elements | Select-Object -Skip 1 | ForEach-Object { $_.ToString() })
if ($wordToComplete -eq '' -or $args[-1] -ne $wordToComplete) {
    $args += $wordToComplete   # ensure the (possibly empty) current token is last
}
& $Command __complete -- @args
```

---

## 3. Emitting `CompletionResult` objects

The engine's stdout is parsed line by line. Each candidate line becomes one
`[System.Management.Automation.CompletionResult]`:

```powershell
[System.Management.Automation.CompletionResult]::new(
    $completionText,   # text inserted into the command line
    $listItemText,     # label shown in the menu/pager
    $resultType,       # a [CompletionResultType] enum value
    $toolTip           # description shown in the PSReadLine tooltip pane
)
```

Conduit's mapping from the wire format:

| Wire element                   | `CompletionResult` field         |
|--------------------------------|----------------------------------|
| `VALUE` (before the tab)       | `completionText` and `listItemText` |
| `DESCRIPTION` (after the tab)  | `toolTip`                        |
| (derived, see §3.1)            | `resultType`                     |

### 3.1 Choosing `ResultType`

`CompletionResultType` influences how PSReadLine renders and how spacing is
applied. Conduit selects:

- **`ParameterName`** — when `VALUE` begins with `-` or `--` (a flag).
- **`Command`** — for subcommand names (e.g. `flow`, `run`, `lsp`).
- **`ProviderItem`** — for file/path candidates produced under `FilterFileExt` /
  `FilterDirs`.
- **`ParameterValue`** — the default for everything else (enum values, runbook
  names, argument values).

### 3.2 Tooltips in the PSReadLine menu

When `Set-PSReadLineKeyHandler -Key Tab -Function MenuComplete` is active (or the
default `MenuComplete` binding is used), PSReadLine shows a selectable menu and
renders the `toolTip` for the highlighted item. This is where the engine's
`DESCRIPTION` shines: mapping it to `toolTip` gives PowerShell users the same
descriptive experience fish users get natively and zsh users get via `_describe`.

If a candidate has no description, we still pass a non-empty `toolTip` (falling
back to the value itself), because `CompletionResult` rejects a null/empty
tooltip.

### 3.3 ActiveHelp presentation

ActiveHelp lines (`_activeHelp_ <text>`) are guidance, not insertable values.
PowerShell has no dedicated ActiveHelp surface, so — when `$env:CONDUIT_ACTIVE_HELP`
is not `0` — Conduit surfaces each as a `CompletionResult` whose `completionText`
equals the current `$wordToComplete` (so selecting it is a no-op that does not
mangle the line) and whose `listItemText`/`toolTip` carry the help text. When
`CONDUIT_ACTIVE_HELP=0`, these lines are dropped.

---

## 4. Applying the directive

The final `:<bitmask>` line is parsed into an integer and applied:

| Directive         | PowerShell realization                                                                                         |
|-------------------|---------------------------------------------------------------------------------------------------------------|
| `Error` (1)       | Return an empty result set (`return`); PowerShell then does its own default (usually nothing).                |
| `NoSpace` (2)     | Append no trailing space. PowerShell adds a space after a `CompletionResult` by default; we suppress it by appending a zero-width sentinel or by emitting the value without the trailing separator (see §4.1). |
| `NoFileComp` (4)  | Do not synthesize file candidates and return at least one (possibly empty-value) result so PowerShell does not fall through to its native file completer. |
| `FilterFileExt` (8) | Enumerate matching files, restricting to the engine's extensions (Conduit: `*.flow`), emitted as `ProviderItem`. |
| `FilterDirs` (16) | Enumerate directories only, emitted as `ProviderItem`.                                                        |
| `KeepOrder` (32)  | Emit results in engine order; PowerShell preserves the order of returned `CompletionResult`s, so simply do not sort. |

### 4.1 NoSpace specifics

Native completers cannot directly tell PSReadLine "no trailing space". The
established technique (used by Cobra's PowerShell output and adopted here) is:
when `NoSpace` is set, if all candidates share a common trailing character we
leave the token unterminated; more robustly, we append a sentinel space to
`completionText` for candidates that _should_ get a space and omit it for those
that should not, then normalize. In practice Conduit sets `NoSpace` chiefly for
`--flag=` style values (which already end in `=`), so the natural rendering keeps
the cursor adjacent without a hack.

### 4.2 NoFileComp specifics

If the engine returns zero candidates **and** `NoFileComp` is set, PowerShell
would otherwise fall through to filesystem completion. To prevent that, the
script block returns a single `CompletionResult` whose `completionText` is the
unchanged `$wordToComplete`, effectively a no-op that blocks the fallback.

---

## 5. Installation flow

`conduit completion powershell` prints the registration script to stdout. Load
it into the current session, and persist it into your profile for future
sessions.

```powershell
# Load Conduit completions into the CURRENT session (annotated).
# `conduit completion powershell` emits the Register-ArgumentCompleter script;
# Out-String joins it into a single string; Invoke-Expression evaluates it.
conduit completion powershell | Out-String | Invoke-Expression

# Persist for FUTURE sessions by appending the same line to your profile.
# $PROFILE is the per-user, per-host profile path. Create it if missing.
if (-not (Test-Path $PROFILE)) {
    New-Item -ItemType File -Path $PROFILE -Force | Out-Null
}
Add-Content -Path $PROFILE -Value 'conduit completion powershell | Out-String | Invoke-Expression'
```

For faster shell startup you may instead dump the generated script to a file and
dot-source it from `$PROFILE`, avoiding a `conduit` process launch on every
session:

```powershell
$dir = Split-Path $PROFILE
conduit completion powershell > (Join-Path $dir 'conduit.completion.ps1')
Add-Content -Path $PROFILE -Value '. (Join-Path (Split-Path $PROFILE) "conduit.completion.ps1")'
```

### 5.1 Enable menu completion (recommended)

```powershell
# Show a rich, tooltip-bearing menu on Tab instead of cycling inline.
Set-PSReadLineKeyHandler -Key Tab -Function MenuComplete
```

Add that line to `$PROFILE` as well to make it permanent.

---

## 6. Windows specifics

- **Windows PowerShell 5.1 vs PowerShell 7+.** The
  `Register-ArgumentCompleter -Native` API and `CompletionResult` type exist in
  both, so the same generated script works on 5.1 and 7+. Differences to note:
  PowerShell 7+ ships a newer PSReadLine with better menu/tooltip rendering and
  predictive IntelliSense; 5.1 uses `$PsHome`-bundled PSReadLine (often older) so
  tooltips may render more plainly. The `$PROFILE` path also differs
  (`WindowsPowerShell` vs `PowerShell` under `Documents`), but using the
  `$PROFILE` variable rather than a literal path keeps the install portable.
- **Execution policy.** Loading a persisted `.ps1` completion file or running a
  profile that dot-sources one may be blocked by the default `Restricted` policy
  on Windows client SKUs. The recommended, least-privilege fix is:

  ```powershell
  Set-ExecutionPolicy -Scope CurrentUser RemoteSigned
  ```

  `RemoteSigned` allows locally-authored scripts (like your profile and the
  generated completion file) to run while still requiring a signature on
  downloaded scripts. Piping `conduit completion powershell | Out-String |
  Invoke-Expression` executes in-memory and is generally unaffected by execution
  policy, which is why it is the primary install path.
- **Paths and quoting.** Windows paths contain spaces (`C:\Program Files\...`)
  and backslashes; the script block calls the resolved command via
  `& $Command __complete ...` where `$Command` is the actual executable path from
  the AST, and passes reconstructed elements as an array (`@args`) so PowerShell
  quotes each argument correctly rather than re-splitting on spaces.
- **Cross-platform native-arg completion.** `Register-ArgumentCompleter -Native`
  is a PowerShell (7+) feature that works identically on Windows, Linux, and
  macOS. This means the _same_ `conduit completion powershell` output drives
  completion for `pwsh` everywhere, and the engine (being an OS-native binary)
  behaves consistently. Only the profile path and execution-policy concerns are
  Windows-specific.

---

## 7. Generated PowerShell completion script (annotated)

The following is a realistic, Conduit-branded rendering of what
`conduit completion powershell` emits. It resembles Cobra's PowerShell output but
is annotated to explain each step.

```powershell
# PowerShell completion for conduit                              -*- powershell -*-
#
# Generated by `conduit completion powershell`. Do not edit by hand.
# All completion logic is delegated to the hidden `conduit __complete`
# command (see docs/50-completion-engine.md). This script is a thin
# front-end that translates the shared wire protocol into PowerShell
# CompletionResult objects.

Register-ArgumentCompleter -CommandName 'conduit','cdt' -Native -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    # Resolve the actual executable being completed (handles conduit or cdt).
    $Command = $commandAst.CommandElements[0].Value

    # --- Build the argv forwarded to the engine -------------------------------
    # Take every element after the command name and stringify it.
    $elements = @($commandAst.CommandElements | Select-Object -Skip 1 |
                  ForEach-Object { $_.Extent.Text })

    # Ensure the (possibly empty) current word is the LAST argument so the
    # engine computes prefix matches for it.
    if ($elements.Count -eq 0 -or $elements[-1] -ne $wordToComplete) {
        $elements += $wordToComplete
    }

    # --- Invoke the shared completion engine ----------------------------------
    # Output is the shared protocol: VALUE\tDESCRIPTION lines, optional
    # _activeHelp_ lines, and a final ":<bitmask>" directive line.
    $env:CONDUIT_COMP_SHELL = 'powershell'
    $result = & $Command __complete -- @elements 2>$null

    if ($null -eq $result) { return }   # engine produced nothing

    # Split output into candidate lines and the trailing directive line.
    $lines     = @($result)
    $directive = 0
    $last      = $lines[-1]
    if ($last -match '^:(\d+)$') {
        $directive = [int]$Matches[1]
        $lines = $lines[0..($lines.Count - 2)]   # drop the directive line
    }

    # Bit 0 (Error=1): abort with no results.
    if ($directive -band 1) { return }

    # Precompute whether to suppress a trailing space (NoSpace = bit 1 = 2).
    $noSpace = [bool]($directive -band 2)

    foreach ($line in $lines) {
        if ([string]::IsNullOrEmpty($line)) { continue }

        # ActiveHelp: guidance, not an insertable value.
        if ($line.StartsWith('_activeHelp_ ')) {
            if ($env:CONDUIT_ACTIVE_HELP -ne '0') {
                $help = $line.Substring('_activeHelp_ '.Length)
                # Selecting it is a no-op: completionText == current word.
                [System.Management.Automation.CompletionResult]::new(
                    $wordToComplete, $help,
                    [System.Management.Automation.CompletionResultType]::Text,
                    $help)
            }
            continue
        }

        # Split VALUE\tDESCRIPTION. Tooltip must be non-empty.
        $parts = $line -split "`t", 2
        $value = $parts[0]
        $desc  = if ($parts.Count -gt 1 -and $parts[1]) { $parts[1] } else { $value }

        # Pick the ResultType from the value shape.
        $type = if ($value.StartsWith('-')) {
            [System.Management.Automation.CompletionResultType]::ParameterName
        } else {
            [System.Management.Automation.CompletionResultType]::ParameterValue
        }

        # NoSpace: leave the token unterminated by not adding a trailing space.
        $insert = if ($noSpace) { $value } else { $value }

        [System.Management.Automation.CompletionResult]::new(
            $insert,   # completionText: inserted into the line
            $value,    # listItemText: shown in the menu
            $type,     # resultType: drives rendering + default spacing
            $desc)     # toolTip: shown in the PSReadLine tooltip pane
    }
}
```

> Menu completion note: this script only _registers_ the completer. To see
> tooltips, users should bind Tab to `MenuComplete`
> (`Set-PSReadLineKeyHandler -Key Tab -Function MenuComplete`), typically in
> `$PROFILE`. The generated installer prints a reminder to that effect.

---

## 8. CompletionResult + directive snippet (annotated)

Below is the isolated, annotated core: emitting a `CompletionResult` per
candidate and honouring the directive bits that require file/dir handling. This
is the PowerShell analogue of the `COMPREPLY`/`compadd` post-processing in the
POSIX shells.

```powershell
function Get-ConduitCompletions {
    param($lines, [int]$directive, $wordToComplete)

    # FilterDirs (16): directories only, as ProviderItem candidates.
    if ($directive -band 16) {
        Get-ChildItem -Directory -Path "$wordToComplete*" -ErrorAction SilentlyContinue |
            ForEach-Object {
                [System.Management.Automation.CompletionResult]::new(
                    $_.Name, $_.Name,
                    [System.Management.Automation.CompletionResultType]::ProviderItem,
                    $_.FullName)
            }
        return
    }

    # FilterFileExt (8): files restricted to the engine's extensions.
    # Conduit uses this for *.flow pipeline inputs (e.g. `conduit flow run`).
    if ($directive -band 8) {
        # $lines here are the allowed extensions (e.g. 'flow').
        foreach ($ext in $lines) {
            Get-ChildItem -File -Path "$wordToComplete*.$ext" -ErrorAction SilentlyContinue |
                ForEach-Object {
                    [System.Management.Automation.CompletionResult]::new(
                        $_.Name, $_.Name,
                        [System.Management.Automation.CompletionResultType]::ProviderItem,
                        $_.FullName)
                }
        }
        return
    }

    # NoFileComp (4): if there are no candidates, return a no-op result so
    # PowerShell does NOT fall through to its native filesystem completer.
    if (($directive -band 4) -and $lines.Count -eq 0) {
        [System.Management.Automation.CompletionResult]::new(
            $wordToComplete, $wordToComplete,
            [System.Management.Automation.CompletionResultType]::ParameterValue,
            $wordToComplete)
        return
    }

    # Default: one ParameterValue/ParameterName CompletionResult per candidate.
    # KeepOrder (32) is honoured implicitly: PowerShell preserves the emission
    # order of CompletionResults, so we simply do NOT sort $lines.
    foreach ($line in $lines) {
        $parts = $line -split "`t", 2
        $value = $parts[0]
        $desc  = if ($parts.Count -gt 1 -and $parts[1]) { $parts[1] } else { $value }
        $type  = if ($value.StartsWith('-')) {
            [System.Management.Automation.CompletionResultType]::ParameterName
        } else {
            [System.Management.Automation.CompletionResultType]::ParameterValue
        }
        [System.Management.Automation.CompletionResult]::new($value, $value, $type, $desc)
    }
}
```

---

## 9. Testing and diagnostics

- Exercise the engine directly to inspect the raw wire format PowerShell parses:

  ```powershell
  conduit __complete -- flow run ''      # candidate lines + trailing :<n>
  conduit __complete -- flow run '.\p'   # FilterFileExt=8 for *.flow inputs
  ```

- Verify registration inside a session:

  ```powershell
  # Load, then confirm the completer is active by simulating a Tab.
  conduit completion powershell | Out-String | Invoke-Expression
  TabExpansion2 'conduit flow ru' 15 | Select-Object -ExpandProperty CompletionMatches
  ```

- Confirm tooltips: with `MenuComplete` bound, the `toolTip` (engine
  `DESCRIPTION`) appears beneath the highlighted menu item.
- Validate the < 50 ms budget:
  `Measure-Command { conduit __complete -- flow '' } | Select-Object TotalMilliseconds`.

---

## 10. Cross-references

- [Completion Engine](50-completion-engine.md) — authoritative definition of the
  `conduit __complete` wire protocol, directive bitmask, and ActiveHelp marker
  that this document consumes.
- [LSP Architecture](56-lsp-architecture.md) — the `conduit lsp` server that
  powers editor IntelliSense for FlowDSL (`*.flow`) files; it shares the same
  candidate-generation core as the shell completion engine, keeping PowerShell
  completion and in-editor completion semantically aligned.
