CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    login VARCHAR(255) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    current_balance INT DEFAULT 0,
    total_spent INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    number VARCHAR(255) UNIQUE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'NEW',
    action VARCHAR(20) NOT NULL CHECK (action IN ('earn', 'spend')),
    points INT NOT NULL DEFAULT 0 CHECK (points >= 0),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_orders_user 
        FOREIGN KEY (user_id) 
        REFERENCES users(id)
        ON DELETE RESTRICT
);

CREATE INDEX idx_orders_user_id ON orders(user_id);