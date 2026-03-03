# flow

A terminal dashboard for developers. See your tasks, manage them, and stay in the zone without leaving the terminal.

![flow screenshot placeholder](https://github.com/benfo/flow/releases)

## Features

- Browse and manage tasks from Jira (or a built-in mock for testing)
- Edit tasks, change status, add comments, create subtasks
- Git branch integration — see which branch is linked to a task, create branches directly
- Full keyboard navigation with built-in help (`?`)

## Install

**macOS / Linux**
```sh
curl -sf https://raw.githubusercontent.com/benfo/flow/main/install.sh | sh
```

**Windows (PowerShell)**
```powershell
irm https://raw.githubusercontent.com/benfo/flow/main/install.ps1 | iex
```

**Go**
```sh
go install github.com/benfo/flow@latest
```

## Setup

Run `flow` and follow the setup prompts to connect to Jira, or use the built-in mock to explore the UI straight away.

## Keybindings

Press `?` inside the app to see all keybindings.
