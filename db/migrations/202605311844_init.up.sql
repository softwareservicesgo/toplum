ALTER TABLE items
ADD COLUMN content_dictionary_id BIGINT,
ADD CONSTRAINT content_dictionary_id_users_fk FOREIGN KEY (content_dictionary_id) REFERENCES dictionary(id);
