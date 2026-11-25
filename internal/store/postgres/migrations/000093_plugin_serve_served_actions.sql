-- What a serving process reported at enrolment about each source's actions and triggers: the plugin's manifests and digests and a summary of its local policy, keyed by source name. Informative only, for Marmot to offer actions and show what they need; the serving process enforces its own policy on every request whatever this says. NULL for enrolments that serve no actions.
ALTER TABLE plugin_serve_enrolments ADD COLUMN IF NOT EXISTS served_actions JSONB;

---- create above / drop below ----

ALTER TABLE plugin_serve_enrolments DROP COLUMN IF EXISTS served_actions;
