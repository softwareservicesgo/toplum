ALTER TABLE users
DROP COLUMN district;

ALTER TABLE users
ADD COLUMN district_dictionary_id BIGINT,
ADD CONSTRAINT district_dictionary_id_users_fk FOREIGN KEY (district_dictionary_id) REFERENCES dictionary(id);
