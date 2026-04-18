# ssm

Interactive CLI to list and select running EC2 instances for AWS SSM Session Manager connections.

## Usage

```bash
ssm --profile <aws-profile> --region <aws-region>
```

Both flags are optional and fall back to the default AWS credential chain and configured region.

## Install

```bash
go install github.com/dalebandoni/ssm@latest
```

Or build locally:

```bash
go build -o ssm .
```

## Navigation

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection |
| Type | Filter by name or instance ID |
| `Enter` | Select instance |
| `q` / `Ctrl+C` | Quit |

