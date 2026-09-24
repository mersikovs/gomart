CREATE TYPE order_status AS ENUM ('NEW', 'PROCESSING', 'PROCESSED', 'INVALID');
CREATE TYPE action_type AS ENUM ('EARN', 'SPEND');

CREATE TABLE users (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    login VARCHAR(255) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    current_balance INT DEFAULT 0 CHECK (current_balance >= 0),
    total_spent INT DEFAULT 0 CHECK (total_spent >= 0),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orders (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id INT NOT NULL,
    number VARCHAR(255) UNIQUE NOT NULL,
    status order_status NOT NULL DEFAULT 'NEW',
    action action_type NOT NULL DEFAULT 'EARN',
    points INT NOT NULL DEFAULT 0 CHECK (points >= 0),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_orders_user 
        FOREIGN KEY (user_id) 
        REFERENCES users(id)
        ON DELETE RESTRICT
);

CREATE INDEX idx_orders_user_id ON orders(user_id);