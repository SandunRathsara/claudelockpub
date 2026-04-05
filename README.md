# ClaudeLock CLI

Install ClaudeLock with:

```bash
curl -fsSL https://raw.githubusercontent.com/SandunRathsara/claudelockpub/main/install.sh | bash
```

The installer currently supports macOS arm64 and Linux amd64, including WSL on amd64. It installs `claudelock` to `/usr/local/bin`, so you may be prompted for elevated privileges depending on your platform and permissions.

## What the installer does

- Detects your platform and downloads the latest ClaudeLock CLI release
- Installs `claudelock` to `/usr/local/bin`
- Prompts for your username, password, and server URL
- Writes `~/.config/claudelock.yaml`
- Prints a bcrypt hash for sharing with Sandun when a local hashing tool is available
- Otherwise prints instructions for generating the bcrypt hash online with a trusted tool. Do not enter your real password into an untrusted third-party site.

## Manual download

If you do not want to use the installer, the following manual steps apply to the currently supported macOS arm64 and Linux amd64/WSL path:

1. Open the [Releases](../../releases) page and download the archive for your platform.
2. Extract the archive and locate the versioned binary inside it, such as `claudelock-cli-vX.Y.Z-linux-amd64`.
3. Move that binary to a directory on your `PATH`, such as `/usr/local/bin`, and rename it to `claudelock` so you can run `claudelock` directly.
4. Create `~/.config/claudelock.yaml` with the config shown below before running `claudelock`.

Windows archives are published separately, but installation and command placement there are platform-specific and are not covered by these steps.

## Uninstall

Remove ClaudeLock from its default paths with:

```bash
curl -fsSL https://raw.githubusercontent.com/SandunRathsara/claudelockpub/main/uninstall.sh | sh
```

Preview what would be removed from those paths without making changes:

```bash
curl -fsSL https://raw.githubusercontent.com/SandunRathsara/claudelockpub/main/uninstall.sh | sh -s -- --dry-run
```

The uninstall script removes these exact paths if present. It does not search for or remove binaries that were manually copied to other locations:

- `/usr/local/bin/claudelock`
- `~/.config/claudelock.yaml`
- timestamped backup files matching `~/.config/claudelock.yaml.*.bak` where `*` is a 14-digit timestamp

## Local config

The installer writes:

```yaml
server_url: "https://claudelock.vps.digisglobal.com"
username: "your_username"
password: "your_password"
```

## Onboarding

After installation, send Sandun your `username` and either:

- the bcrypt hash printed by the installer, or
- a bcrypt hash you generated separately if the installer could not generate one locally

If you use an online bcrypt generator, use only a tool you trust and avoid submitting your real password to unknown third-party websites.
