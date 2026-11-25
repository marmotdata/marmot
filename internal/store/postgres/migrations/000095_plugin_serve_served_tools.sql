-- Plugin actions are now called tools. A row written before this keeps the old JSON keys until its serving process enrols again, at half its certificate's lifetime, and a serving process has to be upgraded to talk to this Marmot anyway.
ALTER TABLE plugin_serve_enrolments RENAME COLUMN served_actions TO served_tools;

---- create above / drop below ----

ALTER TABLE plugin_serve_enrolments RENAME COLUMN served_tools TO served_actions;
