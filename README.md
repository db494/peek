# peek

An interactive terminal UI for browsing EC2 instances and connecting to them
over AWS SSM Session Manager — no SSH keys or open ports required.

```
 peek   profile: production · region: eu-west-2 · 6/6 instances

╭──────────────────────────────────────────────────────────────────────────╮
│ ID                    Name           State      Private IP   Public IP   │
│──────────────────────────────────────────────────────────────────────────│
│ i-0a1b2c3d4e5f60001   web-server-1   running    10.0.1.12    54.12.3.4   │
│ i-0a1b2c3d4e5f60003   db-primary     stopped    10.0.2.40    -           │
╰──────────────────────────────────────────────────────────────────────────╯
enter: connect • /: filter • r: region • q: quit
```

## Prerequisites

- **Go 1.21+** (to build)
- **AWS CLI v2** on your `PATH`
- **[Session Manager plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)**
  (`session-manager-plugin`) installed — the AWS CLI delegates the interactive
  session to it
- AWS credentials configured in `~/.aws/config` / `~/.aws/credentials`
- IAM permissions: `ec2:DescribeInstances`, `ec2:DescribeRegions`,
  `ssm:StartSession` (plus `ssm:TerminateSession` on your own sessions)
- Target instances must run the SSM agent and have an instance profile with
  `AmazonSSMManagedInstanceCore` (or equivalent)

## Build

```sh
go build -o peek .
```

## Usage

```sh
peek                          # pick a profile interactively, then browse
peek --profile production     # skip the profile picker
peek --region eu-west-1       # override the profile's default region
peek --demo                   # explore the UI with fake data, no AWS calls
```

Profile selection is skipped when `--profile` is given, when `AWS_PROFILE` is
set, or when only one profile is configured.

### Keybindings

| Key      | Action                                                  |
|----------|---------------------------------------------------------|
| `↑/↓` `k/j` | Move through the instance table                      |
| `enter`  | Connect to the selected instance (running only)         |
| `/`      | Filter instances by ID, name, state, or IP              |
| `esc`    | Clear the filter / leave the region picker              |
| `r`      | Switch region (fetched live via `DescribeRegions`)      |
| `q` / `ctrl+c` | Quit                                              |

Selecting a running instance exits the TUI, restores your terminal, and execs
`aws ssm start-session --target <instance-id>` with the chosen profile and
region. When the session ends, peek exits with the session's exit code.

## Layout

- `main.go` — flags, startup, and the SSM handoff after the TUI exits
- `aws/` — profile discovery (`~/.aws/config` + `credentials`) and EC2 calls
- `tui/` — Bubble Tea state machine, views, and Lipgloss styles
- `ssm/` — wraps the `aws ssm start-session` exec with the std streams attached
