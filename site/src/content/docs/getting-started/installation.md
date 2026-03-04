---
title: Installation
description: How to install flow on macOS, Linux, and Windows.
---

## Prerequisites

- A supported issue tracker account (currently [Jira](https://www.atlassian.com/software/jira))
- Git installed and available in your `PATH`

## Install script (macOS / Linux)

```bash
curl -sSL https://raw.githubusercontent.com/benfo/flow/main/install.sh | bash
```

This downloads the latest release binary for your platform and places it in `~/.local/bin` (added to `PATH` automatically if it isn't already).

## Install script (Windows)

```powershell
irm https://raw.githubusercontent.com/benfo/flow/main/install.ps1 | iex
```

## Manual install

Download the binary for your platform from the [GitHub Releases page](https://github.com/benfo/flow/releases), make it executable, and place it somewhere on your `PATH`.

## Verify the install

```bash
flow --version
```

## First run

Run `flow` in any directory. On the first launch a welcome screen guides you through connecting your issue tracker. You can also authenticate manually at any time:

```bash
flow auth jira
```
