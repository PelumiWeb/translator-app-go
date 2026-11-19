# ADR 0030: The waitlist, on our own server

Status: accepted, 2026-11-19

## Context

On 2026-11-19 the owner started a soft launch: a few outside people trying
Shinar before it is on the stores. They asked for a way to leave an email on
the website and be invited to TestFlight. Brief round 4 (section 9) had left
the website's call to action open: a waitlist, a TestFlight link, or
"coming soon". The owner chose:

- **The waitlist on our Go server**, not a form service.
- **iPhone only for now.** TestFlight reaches only iPhones. Android sign-ups
  are kept for when there is an Android test.

The website is plain files (ADR 0028) and is not hosted yet. There is no
domain and no service to send email from.

## Decision

**`POST /v1/waitlist`** on the existing server, open to anyone: JSON `email`,
`phone` (`iphone` or `android`) and an optional first `name`. The answer is
the same `200` whether the address is new or already on the list, so the
endpoint cannot be used to learn who has joined. Joining again keeps the
person's place and takes their latest phone. Addresses are one per person
whatever their capitals (migration 0010, table `waitlist`, apart from
accounts).

**A field hidden from people** (`company`) catches form-filling bots: a
request that fills it in is answered as joined and not kept. No CAPTCHA,
which would be a new supplier and a cost to every real visitor.

**Invitations in batches, by hand.** `make waitlist-invite N=20` takes the
next 20 people waiting with an iPhone, those who joined first, marks them
invited, and writes `invitations.csv`: `first name,,email` lines with no
header, the layout App Store Connect imports testers from. The owner imports
it into the external TestFlight group, and TestFlight sends the invitations.
The owner sets the pace of the soft launch, which on the free tiers
(ADR 0029) must stay small. `make waitlist-count` and
`make waitlist-remove EMAIL=...` do the rest. They run the server's binary on
the machine (`-waitlist-invite`, `-waitlist-count`, `-waitlist-remove`)
through `systemd-run`, as the service's user with its settings, over SSH as
the machine's admin.

**The website is served by the same Caddy as the API.** Everything outside
`/v1/` and `/healthz` is the site's files, in `/opt/shinar/site`, shipped by
`make web-ship`. The form posts to `/v1/waitlist` on whatever name served
the page: today the machine's Azure name, later a domain, which
`deploy/setup.sh` adds as a second site where only `/v1/waitlist` reaches
the server. Same origin either way, so there is no CORS to configure, and
the page never needs the server's address. `make web-preview` does the same
on the Mac with a small standard-library Python script.

**What the form says it does with the address:** it is used only to invite
the person and tell them when Shinar is out; for an iPhone it is given to
Apple, whose TestFlight sends the invitation; it is never given to anyone
else; and how to be taken off the list.

## Alternatives considered

**A form service (Tally, Google Forms).** No code, but another supplier
holding the addresses, to name in the privacy note, and the list would live
apart from the server that sends the invitations later.

**TestFlight's public link.** No addresses to keep at all: anyone with the
link joins, up to a cap the owner sets. Simpler, but the owner wants to know
who is coming and to invite in batches. It can still be used beside the
list.

**Inviting automatically through the App Store Connect API.** It can add
testers to a group. Not now: the owner wants to choose when each batch goes,
and the API key would have to live on the server.

## Consequences

- The site now sends something: ADR 0028's "nothing is sent from it" holds
  for every page but this form.
- No email is sent to confirm an address, since there is no mail service
  (plan, open question 4). A mistyped address waits on the list for nothing.
- No limit on how often one visitor can post. The hidden field stops simple
  bots; a determined one could fill the table. To add if it happens.
- The address people write to, to be taken off the list, is
  `CONTACT_EMAIL` in `web/lib/site.ts`. It is empty until the owner chooses
  one; until then the note says to reply to an email from us.
- Taking the site live needs `deploy/setup.sh` run again on the machine (for
  the new Caddy file and the site's folder), then `make web-ship`.
- The machine's name, `shinar-api.southafricanorth.cloudapp.azure.com`, is a
  poor address to share. A domain is the owner's to buy.

## Addendum, 2026-11-19: the website on Vercel

Before it was used, the owner chose to host the site on Vercel (ADR 0031).
The form still posts to `/v1/waitlist` on the name that served the page;
`web/vercel.json` passes it on to the server. Caddy no longer serves the
site: the paragraph above on "the same Caddy as the API", `make web-ship`,
and the consequence about running `deploy/setup.sh` again no longer hold.
