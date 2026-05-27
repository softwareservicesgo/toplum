ALTER TABLE items
ADD COLUMN discount_percent integer DEFAULT 0;

ALTER TABLE items
ADD CONSTRAINT discount_percent_check_items
CHECK (discount_percent >= 0 AND discount_percent <= 100);

ALTER TABLE businesses
ADD COLUMN category_id bigint,
ADD COLUMN "value" real,
ADD CONSTRAINT category_id_businesses_fk FOREIGN KEY (category_id) REFERENCES categories(id);

ALTER TABLE items DROP CONSTRAINT  type_id_items_fk;
ALTER TABLE items DROP COLUMN type_id;

DROP TABLE IF EXISTS businesses_types;

DROP TABLE IF EXISTS types;
