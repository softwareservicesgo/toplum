ALTER TABLE users
DROP COLUMN location;

ALTER TABLE users
ADD COLUMN district VARCHAR(255);

ALTER TABLE users
ADD COLUMN province_id BIGINT,
ADD CONSTRAINT province_id_users_fk FOREIGN KEY (province_id) REFERENCES provinces(id);
