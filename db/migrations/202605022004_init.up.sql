ALTER TABLE businesses DROP CONSTRAINT fk_dress_code_dictionary;

ALTER TABLE businesses DROP COLUMN dress_code_dictionary_id;
ALTER TABLE businesses DROP COLUMN rating;

ALTER TABLE businesses
ADD COLUMN discount_percent integer DEFAULT 0;

ALTER TABLE businesses
ADD CONSTRAINT discount_percent_check_businesses
CHECK (discount_percent >= 0 AND discount_percent <= 100);

ALTER TABLE orders DROP CONSTRAINT  user_coupon_order_id_fk;

DROP TABLE IF EXISTS user_coupons;
DROP TABLE IF EXISTS businesses_coupons;
DROP TABLE IF EXISTS search_histories;
DROP TABLE IF EXISTS reviews;
