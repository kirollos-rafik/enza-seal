# enza-seal

Share test data safely. `enza-seal` encrypts text or files for named colleagues so only they can open them. The sealed result can be pasted into Teams, email, Jira or Confluence.

- No shared passwords. Each device has its own key pair.
- The private key is kept in the operating system's secure store (Windows Credential Manager, macOS Keychain). It never leaves the device.
- Built on [age](https://age-encryption.org), an audited encryption format. No home-made cryptography.
- No server, no telemetry. The only network calls download the program and the team file from this repo.
- For **test and non-production data only**.

## Install

One command, no admin rights, nothing else to install.

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/kirollos-rafik/enza-seal/main/install.ps1 | iex
```

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/kirollos-rafik/enza-seal/main/install.sh | sh
```

The installer downloads the release for your computer, checks its SHA-256 checksum, installs to `%LOCALAPPDATA%\enza-seal` (Windows) or `~/.local/bin` (macOS/Linux), and adds that folder to your PATH. Open a new terminal window afterwards.

> **Windows warning on first run.** The pilot build is not code-signed yet. Windows may show "Windows protected your PC". Click **More info**, then **Run anyway**. Some antivirus tools may quarantine the file; ask IT to allow it.

**Update:** `enza-seal update` tells you if a newer version exists; run the same install command again to install it.
**Remove:** `enza-seal uninstall` deletes your key and settings from this device, then delete the program file it names.

**AWS WorkSpace that cannot reach github.com?** Download the release files on a machine that can, copy the folder over, and run the installer with `ENZA_SEAL_BASE_URL` set to that folder.

## First-time setup

```
enza-seal init --person alice --name "Alice M." --team QA
```

This creates your key, stores the private key in the OS secure store, and prints a short block to add to `team/recipients.json` (open a pull request to this repo). Confirm the fingerprint it shows with your team lead by voice or in person.

## Use it

Type `enza-seal` on its own for a numbered menu, or use the commands:

| Command | What it does |
| --- | --- |
| `enza-seal encrypt --to bob` | Hidden prompt for the text; prints the sealed block and copies it to the clipboard |
| `enza-seal encrypt --to bob,carol` | Several people; any one of them can decrypt |
| `enza-seal encrypt --to @payments-dev` | Everyone in a group |
| `enza-seal encrypt --to me` | All of your own devices (laptop to WorkSpace and back) |
| `enza-seal encrypt --to bob --clip` | Take the text from the clipboard (and clear it), seal it |
| `enza-seal encrypt --to bob -f cards.csv` | Encrypt a file to `cards.csv.seal` |
| `enza-seal decrypt` | Reads the sealed block from the clipboard (or lets you paste it) and prints the text |
| `enza-seal decrypt --copy` | Put the text on the clipboard instead of the screen; cleared after 30 s |
| `enza-seal decrypt -f cards.csv.seal` | Decrypt a file |
| `enza-seal team list` / `team sync` | Who you can encrypt to; refresh the team file (also refreshed daily) |
| `enza-seal whoami` / `device list` | Your details and registered devices |

Plain text is never typed on the command line, so it does not end up in shell history. Pipes work too: `type cards.txt | enza-seal encrypt --to bob`.

Decrypting refuses to overwrite an existing file unless you add `--force`. A file that fails verification leaves no partial output.

## Several devices (laptop and AWS WorkSpace)

Each device has its own key; all of them are listed under your name in the team file, and encrypting for a person locks the data for every device they own.

```
enza-seal init --person alice --device workspace
```

Run that on the WorkSpace, then add the printed line to your entry in `team/recipients.json`. After that, data sealed on the WorkSpace with `--to me` opens on your laptop, and the other way round.

**Non-persistent WorkSpaces** (they reset): use `enza-seal init --person alice --encrypt-only`. No private key is stored there; it can encrypt (including `--to me`) but not decrypt. Decrypt on your laptop.

## Keys

| Command | What it does |
| --- | --- |
| `enza-seal key backup --out alice.key` | Save your private key to a file protected by a passphrase you choose |
| `enza-seal key restore --file alice.key` | Restore it on a new device (run `init` there first) |
| `enza-seal key rotate` | Make a new key; replace the old public key in the team file |

## Adding people (team leads)

`team/recipients.json` holds public keys only. A pull request that adds a person or device needs approval from a team lead (see `.github/CODEOWNERS`). Before approving, check the fingerprint shown by `enza-seal init` with the person, by voice or in person. This check is what stops someone adding a fake key under another person's name.

## FAQ

**"This message was not encrypted for your key."** The sender did not include you, or used an old key for you. Ask them to run `enza-seal team sync` and send it again.

**New laptop or rebuilt WorkSpace.** Restore from a backup, or run `enza-seal init --device <name>` again and replace your old key in the team file.

**Lost laptop.** Remove that device's key from `team/recipients.json`. Messages already sent to it can no longer be opened on a new device; ask senders to re-send.

**Someone who left.** Remove their entry from `team/recipients.json`. They can still read messages already sent to them, which is acceptable for test data.

## Limits

- Malware, or someone logged in as you, can read what you can read.
- If you lose your key and have no backup, old messages sent to you cannot be opened.
- Releases publish a Sigstore signature for `checksums.txt` (`checksums.txt.sig` and `.pem`). The installers check the SHA-256 checksum but do not verify the signature themselves; use `cosign verify-blob` if you want to.

## For developers

You need [Go](https://go.dev/dl/), the version named in `go.mod`.

```
go test ./...
go build -o enza-seal.exe ./cmd/enza-seal
```

Releases are built by GitHub Actions when a version tag is pushed (`git tag v1.0.0 && git push origin v1.0.0`).
