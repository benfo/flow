---
title: Git Integration
description: How flow integrates with Git to link tasks to branches and pull requests.
---

flow connects your tasks to your local Git repository automatically.

## Branch indicators

In the task list, flow shows a badge next to any task that has a matching local branch:

- **Filled pill** (e.g. `● feat/my-task`) — this branch is currently checked out
- **Outlined** (e.g. `○ feat/my-task`) — a local branch exists but is not currently active

## Checking out a branch

From the **task list**, press `b` on any task to open the branch menu.  
From the **task detail view**, press `b` as well.

The branch menu lets you:

- Check out an existing local branch for the task
- Create a new branch from the task (uses the task title as the branch name)
- Move the task to *In Progress* and check out the branch in one step

If your working directory has uncommitted changes, flow will ask whether to stash them before switching branches.

## Opening a pull request

From the **task detail view**, press `P` (capital) to open your browser to the pull request creation page for the task's branch.

flow detects the remote URL (GitHub, GitLab, etc.) and constructs the correct compare URL automatically.
