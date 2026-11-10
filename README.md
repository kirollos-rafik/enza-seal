# enza-seal

Share test data safely. `enza-seal` encrypts text or files for named colleagues so only they can open them. The sealed result can be pasted into Teams, email, Jira or Confluence.

- No shared passwords. Each device has its own key pair.
- The private key is kept in the operating system's secure store (Windows Credential Manager, macOS Keychain). It never leaves the device.
- Built on [age](https://age-encryption.org), an audited encryption format. No home-made cryptography.
- For **test and non-production data only**.

> Status: Phase 1 (core). Installers, the interactive menu, the clipboard helper and signed releases come in Phase 2. See the proposal for the full plan.

## Build (for now)

You need [Go](https://go.dev/dl/) 1.24 or newer.

```
go build -o enza-seal.exe ./cmd/enza-seal
```

(On macOS: `go build -o enza-seal ./cmd/enza-seal`.)

## First-time setup

```
enza-seal init --person alice --name "Alice M." --team QA
```

This creates your key, stores the private key in the OS secure store, and prints a short block to add to `team/recipients.json` (open a pull request). Confirm the fingerprint it shows with your team lead by voice or in person.

Second device, for example your AWS WorkSpace:

```
enza-seal init --person alice --device workspace
```

Encrypting for a person locks the data for all of their devices.

## Daily use

| Command | What it does |
| --- | --- |
| `enza-seal team sync` | Download the latest team file (set `ENZA_SEAL_TEAM_URL` or pass `--url`) |
| `enza-seal team list` | Who you can encrypt to |
| `enza-seal encrypt --to bob` | Hidden prompt for the text; prints a sealed block to paste |
| `enza-seal encrypt --to bob,carol` | Several people; any one of them can decrypt |
| `enza-seal encrypt --to @payments-dev` | Everyone in a group |
| `enza-seal encrypt --to me` | All of your own devices (laptop to WorkSpace and back) |
| `enza-seal encrypt --to bob -f cards.csv` | Encrypt a file to `cards.csv.seal` |
| `enza-seal decrypt` | Paste a sealed block; prints the text |
| `enza-seal decrypt -f cards.csv.seal` | Decrypt a file |
| `enza-seal whoami` | Your name, device, public key and fingerprint |
| `enza-seal device list` | Your registered devices |

Plain text is never typed on the command line, so it does not end up in shell history. Pipes work too: `type cards.txt | enza-seal encrypt --to bob`.

Decrypting refuses to overwrite an existing file unless you add `--force`. A file that fails verification leaves no partial output.

## Windows warning on first run

The pilot build is not code-signed yet. Windows may show "Windows protected your PC". Click **More info**, then **Run anyway**. Some antivirus tools may quarantine the file; ask IT to allow it.

## FAQ

**"This message was not encrypted for your key."** The sender did not include you, or used an old key for you. Ask them to run `enza-seal team sync` and send it again.

**New laptop or rebuilt WorkSpace.** Run `enza-seal init --device <name>` again and replace your old key in the team file.

**Lost laptop.** Remove that device's key from `team/recipients.json`. Messages already sent to it can no longer be opened on a new device; ask senders to re-send.
