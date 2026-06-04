ALTER TABLE users
DROP CONSTRAINT district_dictionary_id_users_fk;

ALTER TABLE users
DROP COLUMN district_dictionary_id;

ALTER TABLE users
ADD COLUMN district VARCHAR(255);