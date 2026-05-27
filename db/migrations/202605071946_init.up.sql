CREATE TABLE IF NOT EXISTS offers (
    "id" bigserial PRIMARY KEY,
    "user_id" bigint NOT NULL,
    "content" character varying(250),
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    
    CONSTRAINT user_id_offer_fk FOREIGN KEY ("user_id") REFERENCES users("id")
);

ALTER TABLE items DROP COLUMN is_chosen;
