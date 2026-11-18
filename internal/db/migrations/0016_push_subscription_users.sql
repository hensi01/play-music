-- Each browser subscription can be associated with multiple accounts.
CREATE TABLE push_subscription_users (
    endpoint TEXT NOT NULL REFERENCES push_subscriptions(endpoint) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (endpoint, user_id)
);
CREATE INDEX push_subscription_users_user_id_idx ON push_subscription_users(user_id);
INSERT INTO push_subscription_users (endpoint, user_id)
SELECT endpoint, user_id FROM push_subscriptions
WHERE active AND user_id IS NOT NULL;
