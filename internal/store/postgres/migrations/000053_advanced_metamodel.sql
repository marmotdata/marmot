-- Give an asset a first-class area, and a version writers can race on,
-- instead of stuffing both into metadata.
--
-- A domain used to live as a free string on the asset. That string is a
-- label, not an identity: renaming it, translating it, or typing the same
-- name twice minted two areas as far as any grant could tell.
-- `business_areas` is the catalog; `assets.business_area_id` is the only
-- reference authorization will use. `name` stays unique so people can find
-- a row, and is never the grant key.
--
-- `business_area_id` is nullable on purpose. System assets, stubs, and rows
-- still waiting to be classified have no area yet. A missing area is not
-- the whole catalog; the authorizer treats it as restricted. A NOT NULL
-- check would reject those rows before they are mapped, so it is not added
-- here. The foreign key refuses a dangling pointer; there is no ON DELETE
-- CASCADE, because dropping an area that still has assets is an
-- application reassignment, not a cascade.
--
-- `Update` replaces `asset.Metadata` when the input carries a map, so two
-- writers who both read, edit, and write can drop each other's fields.
-- `assets.version` is the counter those writers compare: persist only if it
-- still matches, then increment. It is not the metamodel format version and
-- it is not a schemaVersion inside metadata. Rows already in the table
-- start at 1, so a client that sends If-Match: 1 is talking about the row
-- as it existed when this column appeared.
--
-- Role assignments scoped to an area are a later table. This one only
-- introduces the area catalog and the asset pointer.

CREATE TABLE business_areas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 120),
    description TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name)
);

ALTER TABLE assets ADD COLUMN business_area_id UUID REFERENCES business_areas(id);
ALTER TABLE assets ADD COLUMN version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0);
CREATE INDEX assets_business_area_idx ON assets (business_area_id);

COMMENT ON TABLE business_areas IS 'Catalog of authorization areas; stable id, not a translatable name';
COMMENT ON COLUMN business_areas.name IS 'Human label, unique, never the grant key';
COMMENT ON COLUMN business_areas.active IS 'Hidden from pickers when false; rows that still reference it stay valid';
COMMENT ON COLUMN assets.business_area_id IS 'Principal area of the asset; NULL means unclassified, not global';
COMMENT ON COLUMN assets.version IS 'Resource version for compare-and-set; not a metamodel or metadata schema version';

---- create above / drop below ----

COMMENT ON COLUMN assets.business_area_id IS NULL;
COMMENT ON COLUMN assets.version IS NULL;

DROP INDEX assets_business_area_idx;
ALTER TABLE assets DROP COLUMN business_area_id;
ALTER TABLE assets DROP COLUMN version;
DROP TABLE business_areas;
