ALTER TABLE user_businesses
ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
ADD COLUMN reason VARCHAR(250);

ALTER TABLE user_businesses
ADD CONSTRAINT chk_user_business_status
CHECK (status IN ('PENDING', 'APPROVED', 'CANCELED'));
