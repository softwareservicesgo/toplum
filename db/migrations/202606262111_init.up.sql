ALTER TABLE orders
ADD COLUMN approved_by_id BIGINT,
ADD CONSTRAINT approved_by_id_orders_fk FOREIGN KEY (approved_by_id) REFERENCES users(id);
