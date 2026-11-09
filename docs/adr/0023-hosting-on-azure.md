# ADR 0023: Hosting the server on Azure

Status: accepted, 2026-11-09

## Context

Until now the server ran on the owner's Mac, reachable only on their Wi-Fi.
TestFlight (ADR 0022) puts the app on testers' phones anywhere, so the
server has to be reachable from anywhere, over HTTPS, all the time. It
needs an always-on process (the workers translate and voice notes as they
arrive), a real Postgres (the queue relies on `SKIP LOCKED` and
`LISTEN/NOTIFY`), and a disk that survives restarts (recordings, kept
voices, the speech model the phones download).

The owner asked for something free for now, paid for as it grows. Paid
hosts start at roughly $8 to $20 a month (Fly.io, Railway, Render). Of the
free routes, the owner chose Azure's 12-month free services, on the Azure
account they opened for the translator (ADR 0020, addendum).

The server also had two endpoints anyone could call: transcription
(`POST /v1/jobs`) and speaking in a voice (`POST /v1/speech`). In public,
that would let anyone spend the server's time and the Fish Audio credit.

## Decision

**One small Linux machine and a managed Postgres, in Johannesburg** (South
Africa North, the nearest Azure region to Nigeria):

- a B2ats v2 machine (2 CPUs, 1 GB) running Ubuntu 24.04, free for 750
  hours a month for 12 months, with a 64 GB disk;
- Azure Database for PostgreSQL 18, flexible server, Burstable B1ms with
  32 GB, free for 12 months, with Azure's daily backups. Reachable only from
  the machine's address, over TLS. It sits in the machine's region: the
  first one was made in Canada Central by mistake, and from there every
  database call took 0.23 s (measured from the machine).

**On the machine** (`deploy/`): Caddy in front, which gets and renews an
HTTPS certificate for the machine's Azure name
(`shinar-api.southafricanorth.cloudapp.azure.com`) and passes requests to
the server, which listens only on the machine itself. The server runs under
systemd as its own user, may write only its own folders
(`/var/lib/shinar`), and reads its settings from `/etc/shinar/env`, which
only root can read. `deploy/setup.sh` prepares a fresh machine once: Caddy,
the folders, the speech model (checked against its hash), the service, a
firewall, and a `deploy` user. Running it again is safe.

**Deploys from GitHub Actions** (`.github/workflows/server.yml`), on every
push to `main` that changes the server or `deploy/`: lint and the tests
against Postgres 18 (the version the host runs) started beside them, then
`deploy/ship.sh` builds for Linux, copies the program over SSH as `deploy`, restarts the service and
waits for `/healthz` over HTTPS. The `deploy` user may restart the service
and nothing else; the machine's SSH key is pinned in the workflow's
secrets. The program replaced stays beside the new one as
`shinar.previous`. The server applies new migrations as it starts.
`make server-ship` does the same from a Mac.

**Transcribing and speaking need a signed-in user.** Both endpoints sit
behind the same check as the rest of the signed-in API, and both phones send
their session token with them. The model download stays open: it is the
same public file for everyone.

## Setting it up

The owner, in the Azure portal:

1. **The database.** Create an Azure Database for PostgreSQL flexible
   server: resource group `shinar`, name `shinar-db`, region South Africa
   North (the machine's region), PostgreSQL 18, workload "Development",
   compute Burstable B1ms, storage 32 GiB, no high availability, PostgreSQL
   authentication only, with an admin login (the owner chose
   `pelumi`) and a long password of letters and digits. Networking:
   public access, no firewall rules yet.
2. **The machine.** Create a virtual machine: resource group `shinar`, name
   `shinar-api`, region South Africa North, image Ubuntu Server 24.04 LTS
   (x64), size B2ats v2 (the free offer's x64 size), SSH public key
   authentication with the form's default user, `azureuser`, and the key
   from `~/.ssh/shinar_admin.pub`, inbound ports SSH, HTTP and HTTPS. Disks: Premium SSD, OS disk 64 GiB (P6), which the
   free offer covers; the form's default is 30 GiB.
3. **Its name.** On the machine's public IP address, Configuration, set the
   DNS name label to `shinar-api`.
4. **The database firewall.** On `shinar-db`, Networking, add a rule for
   the machine's public IP address.

Then, from the Mac (Claude can do this part):

5. `deploy/setup.sh` on the machine, with the host name and
   `~/.ssh/shinar_deploy.pub`; create the database `shinar`.
6. `/etc/shinar/env` from `deploy/env.example`, with the database URL and
   the keys from `server/.env`.
7. The first deploy, `make server-ship`.

And on GitHub, Settings, Secrets and variables, Actions:

| Kind | Name | Value |
| --- | --- | --- |
| Variable | `SERVER_HOST` | `shinar-api.southafricanorth.cloudapp.azure.com` |
| Secret | `SERVER_SSH_KEY` | `~/.ssh/shinar_deploy` (the private key): `pbcopy < ~/.ssh/shinar_deploy` |
| Secret | `SERVER_HOST_KEY` | the machine's key line: `ssh-keyscan -t ed25519 <host>` |
| Variable | `BACKEND_URL` | `https://shinar-api.southafricanorth.cloudapp.azure.com`, for TestFlight builds (ADR 0022) |

## Alternatives considered

**A paid platform** (Render about $15 to $20 a month, Fly.io about $8 to
$12, Railway about $10 to $20): less to look after, but not free.

**Oracle Cloud's Always Free machine.** Bigger and free with no end date,
but reportedly cut this year, often out of capacity, and Postgres would be
ours to run and back up.

**Free tiers that sleep** (Render's free service with a free Supabase or
Neon database). They stop after 15 minutes idle and take about a minute to
wake, which stalls notes, and have no disk, which would mean moving the
recordings to object storage first.

**Containers on Azure** (Container Apps, App Service). Scaling to zero would
stop the workers, and staying on costs more than the free machine.

## Consequences

- Free for 12 months within the free amounts, once the account is upgraded
  to Pay as you go (by about 2026-12-09, as the translator needs anyway).
  The machine's public IP address may cost about $3.65 a month; it is not
  clearly among the free services. After the 12 months, roughly $20 to $25
  a month, or a move.
- One machine: a deploy restarts the server, and notes wait a few seconds.
  An outage of the machine stops the app until it is back.
- Postgres is backed up by Azure; the files on the machine's disk
  (recordings, kept voices) are not, yet. Losing the machine would lose
  them. Disk snapshots or object storage come before real users.
- The owner looks after the machine: Ubuntu installs security updates by
  itself, but reboots for a new kernel are by hand.
- The address is Azure's name for the machine. A domain of the app's own
  comes with the name's trademark check (ADR 0021); moving to it changes
  `SERVER_HOST`, `BACKEND_URL` and the Caddyfile, not the code.
