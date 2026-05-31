SET client_encoding TO 'UTF-8';

CREATE TABLE IF NOT EXISTS dictionary (
    "id" bigserial PRIMARY KEY,
    "tm" varchar(250) NOT NULL,
    "en" varchar(250) NOT NULL,
    "ru" varchar(250) NOT NULL,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL
);

INSERT INTO dictionary (tm, en, ru) VALUES 
('Aşgabat', 'Ashgabat', 'Ашхабат'), 
('Daşoguz', 'Dashoguz', 'Дашогуз'), 
('Lebap', 'Lebap', 'Лебап'),
('Mary', 'Mary', 'Мары'),
('Ahal', 'Ahal', 'Ахал'),
('Balkan', 'Balkan', 'Балкан'),
('Arkadag şäher', 'Arkadag city', 'Город Аркадаг');

CREATE TABLE  IF NOT EXISTS provinces (
    "id" bigserial primary key,
    "name_dictionary_id" bigint NOT NULL,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,

    CONSTRAINT name_dictionary_id_provinces_fk FOREIGN KEY ("name_dictionary_id") REFERENCES dictionary("id")
    );

    INSERT INTO provinces (name_dictionary_id) VALUES 
    (1), 
    (2), 
    (3),
    (4),
    (5), 
    (6), 
    (7);

    CREATE TABLE  IF NOT EXISTS users (
    "id" bigserial primary key,
    "name" character varying(250),
    "last_name" varchar(250),
    "phone_number" varchar(250),
    "image_path" varchar(250),
    "location" varchar(250),
    "password" varchar(255),
    "otp" int DEFAULT 0,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL
    );

INSERT INTO users (name, phone_number, password, province_id)
    VALUES ('admin', '+99363652268', '$2a$10$XDiLCdamxsncrl5ncl3GlujwXQDi/euWt.00V4dkXXIW4WB57sTNa', 1);

    CREATE TABLE IF NOT EXISTS categories (
    "id" bigserial PRIMARY KEY,
    "name_dictionary_id" bigint NOT NULL,
    "image_path" character varying(250),
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    
    CONSTRAINT name_dictionary_id_category_fk FOREIGN KEY ("name_dictionary_id") REFERENCES dictionary("id")
);

CREATE TABLE  IF NOT EXISTS subcategories (
    "id" bigserial primary key,
    "category_id" bigint NOT NULL,
    "name_dictionary_id" bigint NOT NULL,
    "image_path" character varying(250),
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,

    CONSTRAINT category_id_subcategories_fk FOREIGN KEY ("category_id") REFERENCES categories("id"),
    CONSTRAINT name_dictionary_id_subcategories_fk FOREIGN KEY ("name_dictionary_id") REFERENCES dictionary("id")
    );

CREATE TABLE IF NOT EXISTS businesses (
    "id" bigserial PRIMARY KEY,
    "name" varchar(250) NOT NULL,
    "province_id" bigint NOT NULL,
    "rating" real NOT NULL,
    "district_dictionary_id"  bigint NOT NULL,
    "phone" character varying(250),
    "description_dictionary_id"  bigint NOT NULL,
    "dress_code_dictionary_id" BIGINT DEFAULT 0,
    "opens_time" TIME NOT NULL DEFAULT '08:00',
    "closes_time" TIME NOT NULL DEFAULT '23:00',
    "expires" BIGINT DEFAULT 0,
    "status" varchar(50) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','APPROVED','CANCELED','EXPIRED')),
    "reason" varchar(250),
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
     
    CONSTRAINT province_id_businesses_fk FOREIGN KEY ("province_id") REFERENCES provinces("id"),
    CONSTRAINT district_dictionary_id_businesses_fk FOREIGN KEY ("district_dictionary_id") REFERENCES dictionary("id"),
    CONSTRAINT description_dictionary_id_businesses_fk FOREIGN KEY ("description_dictionary_id") REFERENCES dictionary("id"),
    CONSTRAINT fk_dress_code_dictionary FOREIGN KEY ("dress_code_dictionary_id") REFERENCES dictionary("id")
    );

CREATE TABLE  IF NOT EXISTS user_businesses (
    "id" bigserial primary key,
    "user_id" bigint NOT NULL,
    "businesses_id" bigint,
    "role" varchar(50) CHECK (role IN ('ADMIN', 'MANAGER', 'EMPLOYEE', 'OPERATOR')),
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,

    CONSTRAINT businesses_id_user_businesses_fk FOREIGN KEY ("businesses_id") REFERENCES businesses("id"),
    CONSTRAINT user_id_user_businesses_fk FOREIGN KEY ("user_id") REFERENCES users("id")
    );

INSERT INTO user_businesses (user_id, role)
    VALUES (1, 'ADMIN');  

CREATE TABLE IF NOT EXISTS businesses_subcategories (
    "id" bigserial primary key,
    "businesses_id" bigint NOT NULL,
    "subcategory_id" bigint NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,

    CONSTRAINT businesses_id_businesses_subcategories_fk FOREIGN KEY ("businesses_id") REFERENCES businesses("id"),
    CONSTRAINT subcategory_id_businesses_subcategories_fk FOREIGN KEY ("subcategory_id") REFERENCES subcategories("id")
    );

CREATE TABLE IF NOT EXISTS image_businesses (
    "id" bigserial PRIMARY KEY,
    "businesses_id" bigint NOT NULL,
    "image_path" character varying(250) NOT NULL,
    "is_main" boolean DEFAULT FALSE,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
     
    CONSTRAINT businesses_id_image_businesses_fk FOREIGN KEY ("businesses_id") REFERENCES businesses("id")
    );

    CREATE TABLE  IF NOT EXISTS types (
    "id" bigserial primary key,
    "category_id" bigint NOT NULL,
    "name_dictionary_id" bigint NOT NULL,
    "image_path" character varying(250) NOT NULL,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,

    CONSTRAINT category_id_types_fk FOREIGN KEY ("category_id") REFERENCES categories("id"),
    CONSTRAINT name_dictionary_id_types_fk FOREIGN KEY ("name_dictionary_id") REFERENCES dictionary("id")
    );

CREATE TABLE  IF NOT EXISTS businesses_types (
    "id" bigserial primary key,
    "businesses_id" bigint NOT NULL,
    "type_id" bigint NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,

    CONSTRAINT businesses_id_businesses_types_fk FOREIGN KEY ("businesses_id") REFERENCES businesses("id"),
    CONSTRAINT type_id_businesses_types_fk FOREIGN KEY ("type_id") REFERENCES types("id")
    );

    CREATE TABLE  IF NOT EXISTS item_categories (
    "id" bigserial primary key,
    "name_dictionary_id" bigint NOT NULL,
    "image_path" varchar(250) DEFAULT '',
    "businesses_id" bigint NOT NULL,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,

    CONSTRAINT name_dictionary_id_category_food_fk FOREIGN KEY ("name_dictionary_id") REFERENCES dictionary("id"),
    CONSTRAINT businesses_id_food_categories_fk FOREIGN KEY ("businesses_id") REFERENCES businesses("id")
    );

CREATE TABLE IF NOT EXISTS items (
    "id" bigserial PRIMARY KEY,
    "name_dictionary_id" bigint NOT NULL,
    "ingredient_dictionary_id" bigint NOT NULL,
    "image_path" character varying(250) NOT NULL,
    "value" real NOT NULL,
    "businesses_id" bigint NOT NULL,
    "type_id" bigint NOT NULL,
    "is_chosen" boolean DEFAULT FALSE,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,

    CONSTRAINT name_dictionary_id_items_fk FOREIGN KEY ("name_dictionary_id") REFERENCES dictionary("id"),
    CONSTRAINT ingredient_dictionary_id_items_fk FOREIGN KEY ("ingredient_dictionary_id") REFERENCES dictionary("id"),
    CONSTRAINT businesses_id_items_fk FOREIGN KEY ("businesses_id") REFERENCES businesses("id"),
    CONSTRAINT type_id_items_fk FOREIGN KEY ("type_id") REFERENCES types("id")
    );

CREATE TABLE  IF NOT EXISTS items_item_categories (
    "id" bigserial primary key,
    "item_id" bigint NOT NULL,
    "item_category_id" bigint NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,

    CONSTRAINT item_id_items_item_categories_fk FOREIGN KEY ("item_id") REFERENCES items("id"),
    CONSTRAINT item_category_id_items_item_categories_fk FOREIGN KEY ("item_category_id") REFERENCES item_categories("id")
    );

  CREATE TABLE IF NOT EXISTS reviews (
    "id" bigserial PRIMARY KEY,
    "user_id" bigint NOT NULL,
    "type_id" bigint NOT NULL,
    "type_name" varchar(50) NOT NULL CHECK (type_name IN ('business', 'item')),
    "over_all" bigint NOT NULL DEFAULT 0,
    "food" bigint NOT NULL DEFAULT 0,
    "service" bigint NOT NULL DEFAULT 0,
    "ambience"  bigint NOT NULL DEFAULT 0,
    "value"  bigint NOT NULL DEFAULT 0,
    "comment"  varchar(750),
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,

    CONSTRAINT user_id_review_fk FOREIGN KEY ("user_id") REFERENCES users("id"),
    CONSTRAINT over_all_review_ch CHECK ("over_all" >= 0 AND "over_all" <= 5),
    CONSTRAINT food_review_ch CHECK ("food" >= 0 AND "food" <= 5),
    CONSTRAINT service_review_ch CHECK ("service" >= 0 AND "service" <= 5),
    CONSTRAINT ambience_review_ch CHECK ("ambience" >= 0 AND "ambience" <= 5),
    CONSTRAINT value_review_ch CHECK ("value" >= 0 AND "value" <= 5)
  );

CREATE TABLE IF NOT EXISTS search_histories (
    "id" bigserial PRIMARY KEY,
    "user_id" bigint NOT NULL,
    "search" varchar(250),
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,

    CONSTRAINT user_id_review_fk FOREIGN KEY ("user_id") REFERENCES users("id")
  );

  CREATE TABLE IF NOT EXISTS businesses_tables (
    "id" bigserial PRIMARY KEY,
    "businesses_id" bigint NOT NULL,
    "seats" INT NOT NULL,
    "table_count" INT NOT NULL,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    
    CONSTRAINT businesses_id_businesses_tables_fk FOREIGN KEY ("businesses_id") REFERENCES businesses("id")
);

CREATE TABLE IF NOT EXISTS reservations (
    "id" bigserial PRIMARY KEY,
    "businesses_id" bigint NOT NULL,
    "user_id" bigint NOT NULL,
    "count_person" bigint NOT NULL DEFAULT 1,
    "reservation_date" TIMESTAMP,
    "wish_content" varchar(250),
    "status" varchar(50) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','APPROVED','CONFIRMED','COMPLETED','CANCELLED_BY_BUSINESSES','CANCELLED_BY_CLIENT','NO_SHOW')),
    "businesses_table_id" BIGINT,
    "reason" varchar(250),
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')  NOT NULL,

    CONSTRAINT businesses_id_reservations_fk FOREIGN KEY ("businesses_id") REFERENCES businesses("id"),
    CONSTRAINT user_id_reservations_fk FOREIGN KEY ("user_id") REFERENCES users("id"),
    CONSTRAINT businesses_table_id_reservation_fk FOREIGN KEY ("businesses_table_id") REFERENCES businesses_tables("id")
    );


CREATE TABLE IF NOT EXISTS businesses_coupons (
    "id" bigserial PRIMARY KEY,
    "businesses_id" bigint NOT NULL,
    "coupon_dictionary_id" bigint NOT NULL,
    "life" bigint NOT NULL DEFAULT 0, 
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    
    CONSTRAINT businesses_id_businesses_coupons_fk FOREIGN KEY ("businesses_id") REFERENCES businesses("id"),
    CONSTRAINT coupon_dictionary_id_businesses_coupons_fk FOREIGN KEY ("coupon_dictionary_id") REFERENCES dictionary("id")
);

CREATE TABLE IF NOT EXISTS user_coupons (
    "id" bigserial PRIMARY KEY,
    "user_id" bigint NOT NULL,
    "businesses_coupon_id" bigint NOT NULL,
    "booking_id" bigint,
    "booking_type" VARCHAR(20) CHECK (booking_type IN ('RESERVATION','ORDER')),
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    
    CONSTRAINT user_id_user_coupons_fk FOREIGN KEY ("user_id") REFERENCES users("id"),
    CONSTRAINT businesses_coupon_id_user_coupons_fk FOREIGN KEY ("businesses_coupon_id") REFERENCES businesses_coupons("id")
);

CREATE TABLE IF NOT EXISTS reservation_items (
    "id" bigserial PRIMARY KEY,
    "reservation_id" bigint NOT NULL,
    "item_id" bigint NOT NULL,
    "count" bigint NOT NULL,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    
    CONSTRAINT reservation_id_reservation_items_fk FOREIGN KEY ("reservation_id") REFERENCES reservations("id"),
    CONSTRAINT item_id_reservation_items_fk FOREIGN KEY ("item_id") REFERENCES items("id")
);

CREATE TABLE IF NOT EXISTS device_token (
    "id" bigserial PRIMARY KEY,
    "user_id" bigint ,
    "token" varchar(250) NOT NULL UNIQUE,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    
    CONSTRAINT user_id_deviceToken_fk FOREIGN KEY ("user_id") REFERENCES users("id")
);

CREATE TABLE IF NOT EXISTS notifications (
    "id" bigserial PRIMARY KEY,
    "businesses_id" bigint NOT NULL,
    "title_dictionary_id" bigint NOT NULL,
    "content_dictionary_id" bigint NOT NULL,
    "life" bigint,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    
    CONSTRAINT title_dictionary_id_notifications_fk FOREIGN KEY ("title_dictionary_id") REFERENCES dictionary("id"),
    CONSTRAINT content_dictionary_id_notifications_fk FOREIGN KEY ("content_dictionary_id") REFERENCES dictionary("id"),
    CONSTRAINT businesses_id_notifications_fk FOREIGN KEY ("businesses_id") REFERENCES businesses("id")
);

CREATE TABLE IF NOT EXISTS user_notifications (
    "id" bigserial PRIMARY KEY,
    "user_id" bigint ,
    "notification_id" bigint NOT NULL,
    "device_token_id" bigint NOT NULL,
    "read" BOOLEAN DEFAULT FALSE,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    
    CONSTRAINT notification_id_user_notifications_fk FOREIGN KEY ("notification_id") REFERENCES notifications("id") ON DELETE CASCADE,
    CONSTRAINT user_id_user_notifications_fk FOREIGN KEY ("user_id") REFERENCES users("id"),
    CONSTRAINT device_token_id_user_notifications_fk FOREIGN KEY ("device_token_id") REFERENCES device_token("id"),
    CONSTRAINT unique_user_notifications_fk UNIQUE (notification_id, user_id)
);

CREATE TABLE IF NOT EXISTS basket (
    "id" bigserial PRIMARY KEY,
    "user_id" bigint NOT NULL,
    "item_id" bigint NOT NULL,
    "count" bigint NOT NULL,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    
    CONSTRAINT user_id_basket_fk FOREIGN KEY ("user_id") REFERENCES users("id"),
    CONSTRAINT item_id_basket_fk FOREIGN KEY ("item_id") REFERENCES items("id")
);

CREATE TABLE IF NOT EXISTS orders (
    "id" bigserial PRIMARY KEY,
    "user_id" bigint NOT NULL,
    "businesses_id" bigint NOT NULL,
    "user_coupon_id" bigint,
    "total_price" real NOT NULL,
    "place" varchar(250) NOT NULL,
    "order_time" TIMESTAMP NOT NULL,
    "status" varchar(50) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','APPROVED','COMPLETED_BY_CLIENT','COMPLETED_BY_BUSINESSES','CANCELED_BY_BUSINESSES','CANCELED_BY_CLIENT')),
    "reason" varchar(250),
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC'),
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC'),

    CONSTRAINT user_id_order_fk FOREIGN KEY ("user_id") REFERENCES users(id),
    CONSTRAINT businesses_id_order_fk FOREIGN KEY ("businesses_id") REFERENCES businesses(id),
    CONSTRAINT user_coupon_order_id_fk FOREIGN KEY ("user_coupon_id") REFERENCES user_coupons(id)
);

CREATE TABLE IF NOT EXISTS order_items (
    "id" bigserial PRIMARY KEY,
    "order_id" bigint NOT NULL,
    "item_id" bigint NOT NULL,
    "quantity" int NOT NULL CHECK (quantity > 0),
    "price" real NOT NULL CHECK (price >= 0),
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC'),
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC'),

    CONSTRAINT fk_order_id_order_items FOREIGN KEY ("order_id") REFERENCES orders(id),
    CONSTRAINT fk_item_id_order_items FOREIGN KEY ("item_id") REFERENCES items(id),
    CONSTRAINT uq_order_items UNIQUE ("order_id", "item_id")
);

CREATE TABLE IF NOT EXISTS blacklist (
    "id" bigserial PRIMARY KEY,
    "token" varchar(250) NOT NULL,
    "created_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC'),
    "updated_at" TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')
);
