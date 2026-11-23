-- Paying for Shinar (milestone 21, ADR 0034): subscriptions bought through
-- Apple's In-App Purchase, as Apple's signed records say they stand.

-- One row per subscription, as Apple counts them: an original transaction and
-- every renewal after it.
CREATE TABLE subscriptions (
    original_transaction_id text PRIMARY KEY,
    -- The account it belongs to. Deleting the account leaves the
    -- subscription, which Apple goes on billing, so that the account that
    -- restores it next takes it.
    user_id         uuid REFERENCES users(id) ON DELETE SET NULL,
    product_id      text NOT NULL,
    environment     text NOT NULL CHECK (environment IN ('Production', 'Sandbox')),
    -- The newest transaction applied, and the period it paid for.
    transaction_id  text NOT NULL,
    period_start    timestamptz NOT NULL,
    expires_at      timestamptz NOT NULL,
    free_trial      boolean NOT NULL, -- this period is the free trial
    revoked_at      timestamptz,      -- refunded, or taken back by Apple
    -- From the renewal information: whether it renews, and the grace period
    -- while Apple retries a failed payment.
    auto_renew        boolean NOT NULL DEFAULT true,
    grace_until       timestamptz,
    renewal_signed_at timestamptz,    -- so an older renewal record is not applied over a newer one
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX subscriptions_user ON subscriptions (user_id);

-- Each notification from the App Store, once, as it came: the record of what
-- Apple told us. Apple sends one again when it is not sure it arrived.
CREATE TABLE store_notifications (
    id             text PRIMARY KEY, -- Apple's notificationUUID
    type           text NOT NULL,
    subtype        text NOT NULL DEFAULT '',
    signed_payload text NOT NULL,
    received_at    timestamptz NOT NULL DEFAULT now()
);
