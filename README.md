# peek

Interactive CLI to browse running EC2 instances and start an AWS SSM Session Manager shell.

## Prerequisites

- AWS credentials configured (env vars, `~/.aws/credentials`, or IAM role)
- [`session-manager-plugin`](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) installed and in `PATH`

## Usage

```bash
peek [--profile <aws-profile>] [--region <aws-region>]
```

Both flags are optional. `--profile` / `-p` accepts any profile from `~/.aws/config` or `~/.aws/credentials`. Run `peek --list-profiles` to see available profiles.

## Install

```bash
go install github.com/db494/peek@latest
```

Or build locally:

```bash
go build -o peek .
```

## Navigation

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection |
| `Enter` | Connect via SSM |
| `q` / `Ctrl+C` | Quit |

The table shows: **Name**, **Instance ID**, **Private IP**, **Type**, **OS**, **AMI**, **State**.
