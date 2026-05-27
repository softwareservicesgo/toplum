ALTER TABLE businesses
ADD COLUMN can_order boolean DEFAULT false,
ADD COLUMN can_reserve boolean DEFAULT false;
