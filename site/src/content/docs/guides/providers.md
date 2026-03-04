---
title: Providers
description: Supported issue tracker providers and how to configure them.
---

## Jira

[Jira](https://www.atlassian.com/software/jira) is the first supported provider.

### Authentication

The easiest way is to run `flow` — on first launch the welcome screen walks you through setup inline. You can also run the auth wizard at any time:

```bash
flow auth jira
```

You will be asked for your Jira domain, email address, and an [API token](https://id.atlassian.com/manage-api-tokens). Credentials are stored in your OS keychain; no plain-text secrets are written to disk.

### Configuration

After authenticating you can optionally filter which projects appear in the dashboard. The setup wizard prompts for project keys (e.g. `PROJ, TEAM`) and lets you save the filter globally or per repository.

## Planned providers

The following providers are on the roadmap:

- **Linear**
- **GitHub Issues**

If you would like to contribute a provider, see the [GitHub repository](https://github.com/benfo/flow).
